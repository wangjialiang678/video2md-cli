package asrbatch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	defaultFunASRRecordedModel        = "fun-asr"
	defaultFunASRBaseHTTPAPIURL       = "https://dashscope.aliyuncs.com/api/v1"
	defaultFunASRRecordedPollInterval = 800 * time.Millisecond
	defaultFunASRRecordedWaitTimeout  = 10 * time.Minute
	defaultFunASRRecordedHTTPTimeout  = 30 * time.Second
	recordedTranscriptionPath         = "/services/audio/asr/transcription"
	recordedTaskPathPrefix            = "/tasks/"

	// 让 DashScope 把 oss:// 临时文件地址解析成它自己能下载的预签名 URL。
	// 只有走 DashScope 临时存储时才需要；自建 OSS 传的是 https 预签名 URL，不加此头。
	ossResourceResolveHeader = "X-DashScope-OssResourceResolve"
	ossURLScheme             = "oss://"
)

func hasOSSSchemeURL(fileURLs []string) bool {
	for _, fileURL := range fileURLs {
		if strings.HasPrefix(strings.TrimSpace(fileURL), ossURLScheme) {
			return true
		}
	}
	return false
}

type FunASRRecordedConfig struct {
	APIKey         string
	BaseHTTPAPIURL string
	Model          string
	HTTPClient     *http.Client
	PollInterval   time.Duration
	WaitTimeout    time.Duration
	RequestHeaders map[string]string
	Parameters     map[string]any
}

type RecordedProvider struct {
	config FunASRRecordedConfig
}

func NewRecordedProvider(config FunASRRecordedConfig) (*RecordedProvider, error) {
	config = config.withDefaults()
	if strings.TrimSpace(config.APIKey) == "" {
		return nil, errors.New("asrbatch: apiKey is required")
	}
	return &RecordedProvider{config: config}, nil
}

func (p *RecordedProvider) Name() string {
	return "funasr"
}

func (p *RecordedProvider) Recognize(ctx context.Context, request Request) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := request.Validate(); err != nil {
		return Result{}, err
	}

	fileURLs := request.SanitizedFileURLs()
	pollInterval := firstDuration(request.PollInterval, p.config.PollInterval)
	waitTimeout := firstDuration(request.WaitTimeout, p.config.WaitTimeout)

	model := strings.TrimSpace(request.Model)
	if model == "" {
		model = p.config.Model
	}

	taskID, requestID, err := p.submitTask(ctx, model, request, fileURLs)
	if err != nil {
		return Result{}, err
	}

	result := Result{Provider: p.Name(), TaskID: taskID, RequestID: requestID}
	waitCtx, cancel := context.WithTimeout(ctx, waitTimeout)
	defer cancel()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		snapshot, err := p.queryTask(waitCtx, taskID)
		if err != nil {
			return result, err
		}

		if snapshot.RequestID != "" {
			result.RequestID = snapshot.RequestID
		}
		result.TaskStatus = snapshot.TaskStatus
		result.Subtasks = append([]Subtask(nil), snapshot.Subtasks...)
		result.SubmitTime = snapshot.SubmitTime
		result.ScheduledAt = snapshot.ScheduledTime
		result.EndTime = snapshot.EndTime
		result.Usage = cloneUsage(snapshot.Usage)

		switch snapshot.TaskStatus {
		case TaskStatusSucceeded:
			transcripts, err := p.fetchTranscripts(waitCtx, snapshot.Subtasks, result.RequestID)
			if err != nil {
				return result, err
			}
			result.Transcripts = transcripts
			return result, nil
		case TaskStatusFailed:
			return result, p.failedTaskError(snapshot)
		}

		select {
		case <-waitCtx.Done():
			return result, fmt.Errorf("funasr recorded task wait timeout: %w", waitCtx.Err())
		case <-ticker.C:
		}
	}
}

func (p *RecordedProvider) submitTask(ctx context.Context, model string, request Request, fileURLs []string) (string, string, error) {
	payload := map[string]any{
		"model": model,
		"input": map[string]any{"file_urls": fileURLs},
	}
	if parameters := buildRecordedParameters(p.config.Parameters, request); len(parameters) > 0 {
		payload["parameters"] = parameters
	}

	extraHeaders := map[string]string{}
	if hasOSSSchemeURL(fileURLs) {
		extraHeaders[ossResourceResolveHeader] = "enable"
	}

	response := dashScopeTaskResponse{}
	if err := p.callAPI(ctx, http.MethodPost, recordedTranscriptionPath, payload, true, extraHeaders, &response); err != nil {
		return "", "", err
	}
	taskID := strings.TrimSpace(response.Output.TaskID)
	if taskID == "" {
		return "", strings.TrimSpace(response.RequestID), errors.New("funasr recorded response missing task_id")
	}
	return taskID, strings.TrimSpace(response.RequestID), nil
}

func (p *RecordedProvider) queryTask(ctx context.Context, taskID string) (taskSnapshot, error) {
	response := dashScopeTaskResponse{}
	if err := p.callAPI(ctx, http.MethodPost, recordedTaskPathPrefix+strings.TrimSpace(taskID), nil, false, nil, &response); err != nil {
		return taskSnapshot{}, err
	}

	results := response.Output.normalizedResults()
	snapshot := taskSnapshot{
		RequestID:     strings.TrimSpace(response.RequestID),
		TaskID:        strings.TrimSpace(response.Output.TaskID),
		TaskStatus:    NormalizeTaskStatus(response.Output.TaskStatus),
		SubmitTime:    strings.TrimSpace(response.Output.SubmitTime),
		ScheduledTime: strings.TrimSpace(response.Output.ScheduledTime),
		EndTime:       strings.TrimSpace(response.Output.EndTime),
		Usage:         parseUsageMap(response.Usage),
		Subtasks:      make([]Subtask, 0, len(results)),
	}
	for _, item := range results {
		snapshot.Subtasks = append(snapshot.Subtasks, Subtask{
			FileURL:          strings.TrimSpace(item.normalizedFileURL()),
			Status:           NormalizeTaskStatus(item.normalizedStatus()),
			Code:             strings.TrimSpace(item.normalizedCode()),
			Message:          strings.TrimSpace(item.normalizedMessage()),
			TranscriptionURL: strings.TrimSpace(item.normalizedTranscriptionURL()),
		})
	}
	return snapshot, nil
}

func (p *RecordedProvider) failedTaskError(snapshot taskSnapshot) error {
	for _, subtask := range snapshot.Subtasks {
		if subtask.Status != TaskStatusFailed {
			continue
		}
		message := firstNonEmpty(subtask.Message, "funasr recorded subtask failed")
		if subtask.Code != "" {
			message = fmt.Sprintf("%s [%s]", message, subtask.Code)
		}
		return errors.New(message)
	}
	return errors.New("funasr recorded task failed")
}

func (p *RecordedProvider) fetchTranscripts(ctx context.Context, subtasks []Subtask, requestID string) ([]FileTranscript, error) {
	transcripts := make([]FileTranscript, 0, len(subtasks))
	succeededSubtaskCount := 0
	missingTranscriptionURLCount := 0

	for _, subtask := range subtasks {
		if subtask.Status != TaskStatusSucceeded {
			continue
		}
		succeededSubtaskCount++
		url := strings.TrimSpace(subtask.TranscriptionURL)
		if url == "" {
			missingTranscriptionURLCount++
			continue
		}
		fileTranscript, err := p.fetchTranscriptFile(ctx, url, requestID)
		if err != nil {
			return nil, err
		}
		if fileTranscript.FileURL == "" {
			fileTranscript.FileURL = subtask.FileURL
		}
		transcripts = append(transcripts, fileTranscript)
	}

	if len(transcripts) == 0 {
		switch {
		case succeededSubtaskCount == 0:
			return nil, errors.New("funasr recorded task succeeded but has no succeeded subtasks")
		case missingTranscriptionURLCount == succeededSubtaskCount:
			return nil, errors.New("funasr recorded task succeeded but missing transcript result url in subtasks")
		default:
			return nil, errors.New("funasr recorded task succeeded but transcript content is empty")
		}
	}
	return transcripts, nil
}

func (p *RecordedProvider) fetchTranscriptFile(ctx context.Context, resultURL string, requestID string) (FileTranscript, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, resultURL, nil)
	if err != nil {
		return FileTranscript{}, err
	}
	response, err := p.config.HTTPClient.Do(request)
	if err != nil {
		return FileTranscript{}, fmt.Errorf("download transcript failed: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return FileTranscript{}, err
	}
	if response.StatusCode >= 400 {
		return FileTranscript{}, fmt.Errorf("download transcript failed: HTTP %d", response.StatusCode)
	}

	var raw map[string]any
	if len(bytes.TrimSpace(body)) > 0 {
		_ = json.Unmarshal(body, &raw)
	}

	var payload transcriptFilePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return FileTranscript{}, err
	}

	channelOutputs := make([]ChannelTranscript, 0, len(payload.Transcripts))
	allSegments := make([]Segment, 0)
	textParts := make([]string, 0, len(payload.Transcripts))

	for _, channel := range payload.Transcripts {
		channelSegments := make([]Segment, 0, len(channel.Sentences))
		for _, sentence := range channel.Sentences {
			segment := Segment{
				Text:      strings.TrimSpace(sentence.Text),
				BeginMS:   sentence.BeginTime,
				EndMS:     sentence.EndTime,
				IsFinal:   true,
				SpeakerID: sentence.SpeakerID,
				Words:     mapWords(sentence.Words),
			}
			if segment.Text == "" && len(segment.Words) == 0 {
				continue
			}
			channelSegments = append(channelSegments, segment)
		}

		sortSegments(channelSegments)
		text := strings.TrimSpace(channel.Text)
		if text == "" {
			text = joinSegmentText(channelSegments)
		}
		if text == "" && len(channelSegments) == 0 {
			continue
		}
		if text != "" {
			textParts = append(textParts, text)
		}

		channelOutputs = append(channelOutputs, ChannelTranscript{
			ChannelID:  channel.ChannelID,
			Text:       text,
			DurationMS: channel.ContentDurationMS,
			Segments:   channelSegments,
		})
		allSegments = append(allSegments, channelSegments...)
	}

	sortSegments(allSegments)
	fileURL := firstNonEmpty(strings.TrimSpace(payload.FileURL), strings.TrimSpace(toString(raw["file_url"])), strings.TrimSpace(toString(raw["fileUrl"])))
	if len(channelOutputs) == 0 {
		fallbackText := extractFallbackTranscriptText(raw)
		if fallbackText == "" {
			return FileTranscript{}, errors.New("transcript file contains no usable transcript content")
		}
		return FileTranscript{FileURL: fileURL, Text: fallbackText, Raw: raw, RequestID: requestID}, nil
	}

	return FileTranscript{
		FileURL:   fileURL,
		Text:      strings.TrimSpace(strings.Join(textParts, "\n")),
		Channels:  channelOutputs,
		Segments:  allSegments,
		Raw:       raw,
		RequestID: requestID,
	}, nil
}

func (p *RecordedProvider) callAPI(ctx context.Context, method string, path string, requestBody any, async bool, extraHeaders map[string]string, output *dashScopeTaskResponse) error {
	var bodyReader io.Reader
	if requestBody != nil {
		bodyBytes, err := json.Marshal(requestBody)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	request, err := http.NewRequestWithContext(ctx, method, p.config.BaseHTTPAPIURL+path, bodyReader)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(p.config.APIKey))
	request.Header.Set("Content-Type", "application/json")
	if async {
		request.Header.Set("X-DashScope-Async", "enable")
	}
	for key, value := range p.config.RequestHeaders {
		if strings.TrimSpace(key) != "" && strings.TrimSpace(value) != "" {
			request.Header.Set(strings.TrimSpace(key), strings.TrimSpace(value))
		}
	}
	for key, value := range extraHeaders {
		if strings.TrimSpace(key) != "" && strings.TrimSpace(value) != "" {
			request.Header.Set(strings.TrimSpace(key), strings.TrimSpace(value))
		}
	}

	response, err := p.config.HTTPClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	rawBody, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}

	var payload dashScopeTaskResponse
	if len(bytes.TrimSpace(rawBody)) > 0 {
		if err := json.Unmarshal(rawBody, &payload); err != nil {
			return err
		}
	}
	if response.StatusCode >= 400 {
		return fmt.Errorf("dashscope request failed: %s", firstNonEmpty(payload.Message, fmt.Sprintf("HTTP %d", response.StatusCode)))
	}
	if strings.TrimSpace(payload.Code) != "" {
		return fmt.Errorf("dashscope request failed [%s]: %s", payload.Code, firstNonEmpty(payload.Message, "unknown error"))
	}
	if output != nil {
		*output = payload
	}
	return nil
}

func (c FunASRRecordedConfig) withDefaults() FunASRRecordedConfig {
	out := c
	if strings.TrimSpace(out.BaseHTTPAPIURL) == "" {
		out.BaseHTTPAPIURL = defaultFunASRBaseHTTPAPIURL
	}
	out.BaseHTTPAPIURL = strings.TrimRight(out.BaseHTTPAPIURL, "/")
	if strings.TrimSpace(out.Model) == "" {
		out.Model = defaultFunASRRecordedModel
	}
	if out.HTTPClient == nil {
		out.HTTPClient = &http.Client{Timeout: defaultFunASRRecordedHTTPTimeout}
	}
	if out.PollInterval <= 0 {
		out.PollInterval = defaultFunASRRecordedPollInterval
	}
	if out.WaitTimeout <= 0 {
		out.WaitTimeout = defaultFunASRRecordedWaitTimeout
	}
	out.RequestHeaders = cloneStringMap(out.RequestHeaders)
	out.Parameters = cloneMap(out.Parameters)
	return out
}

func buildRecordedParameters(defaults map[string]any, request Request) map[string]any {
	out := cloneMap(defaults)
	if out == nil {
		out = make(map[string]any)
	}
	for key, value := range request.Parameters {
		out[key] = value
	}
	if strings.TrimSpace(request.VocabularyID) != "" {
		out["vocabulary_id"] = strings.TrimSpace(request.VocabularyID)
	}
	if len(request.LanguageHints) > 0 {
		out["language_hints"] = append([]string(nil), request.LanguageHints...)
	}
	if request.DiarizationEnabled != nil {
		out["diarization_enabled"] = *request.DiarizationEnabled
	}
	if request.SpeakerCount != nil && *request.SpeakerCount > 0 {
		out["speaker_count"] = *request.SpeakerCount
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseUsageMap(raw map[string]any) *Usage {
	if len(raw) == 0 {
		return nil
	}
	usage := &Usage{Raw: cloneMap(raw)}
	if durationValue, ok := raw["duration"]; ok {
		if duration, ok := toFloat64(durationValue); ok {
			usage.Duration = &duration
		}
	}
	return usage
}

func mapWords(items []transcriptWordPayload) []Word {
	words := make([]Word, 0, len(items))
	for _, item := range items {
		text := strings.TrimSpace(item.Text)
		if text == "" {
			continue
		}
		word := Word{Text: text, BeginMS: item.BeginTime, EndMS: item.EndTime}
		if item.Confidence != nil {
			confidence := *item.Confidence
			word.Confidence = &confidence
		}
		if punctuation := strings.TrimSpace(item.Punctuation); punctuation != "" {
			word.Punctuation = &punctuation
		}
		words = append(words, word)
	}
	if len(words) == 0 {
		return nil
	}
	return words
}

func joinSegmentText(segments []Segment) string {
	parts := make([]string, 0, len(segments))
	for _, item := range segments {
		if text := strings.TrimSpace(item.Text); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func sortSegments(segments []Segment) {
	sort.Slice(segments, func(i, j int) bool {
		if segments[i].BeginMS == segments[j].BeginMS {
			return segments[i].EndMS < segments[j].EndMS
		}
		return segments[i].BeginMS < segments[j].BeginMS
	})
}

func cloneMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func cloneUsage(usage *Usage) *Usage {
	if usage == nil {
		return nil
	}
	copied := *usage
	copied.Raw = cloneMap(usage.Raw)
	return &copied
}

func firstDuration(values ...time.Duration) time.Duration {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func toString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	default:
		return ""
	}
}

func toFloat64(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		value, err := typed.Float64()
		return value, err == nil
	default:
		return 0, false
	}
}

type taskSnapshot struct {
	RequestID     string
	TaskID        string
	TaskStatus    TaskStatus
	SubmitTime    string
	ScheduledTime string
	EndTime       string
	Subtasks      []Subtask
	Usage         *Usage
}

type dashScopeTaskResponse struct {
	RequestID string              `json:"request_id"`
	Code      string              `json:"code"`
	Message   string              `json:"message"`
	Output    dashScopeTaskOutput `json:"output"`
	Usage     map[string]any      `json:"usage"`
}

type dashScopeTaskOutput struct {
	TaskID        string                `json:"task_id"`
	TaskStatus    string                `json:"task_status"`
	SubmitTime    string                `json:"submit_time"`
	ScheduledTime string                `json:"scheduled_time"`
	EndTime       string                `json:"end_time"`
	Results       []dashScopeTaskResult `json:"results"`
	Result        []dashScopeTaskResult `json:"result"`
	Items         []dashScopeTaskResult `json:"items"`
}

type dashScopeTaskResult struct {
	FileURL               string `json:"file_url"`
	FileURLCamel          string `json:"fileUrl"`
	AudioURL              string `json:"audio_url"`
	AudioURLCamel         string `json:"audioUrl"`
	SubtaskStatus         string `json:"subtask_status"`
	SubtaskStatusCamel    string `json:"subtaskStatus"`
	Status                string `json:"status"`
	TaskStatus            string `json:"task_status"`
	Code                  string `json:"code"`
	ErrorCode             string `json:"error_code"`
	ErrorCodeCamel        string `json:"errorCode"`
	Message               string `json:"message"`
	ErrorMessage          string `json:"error_message"`
	ErrorMessageCamel     string `json:"errorMessage"`
	TranscriptionURL      string `json:"transcription_url"`
	TranscriptionURLCamel string `json:"transcriptionUrl"`
	ResultURL             string `json:"result_url"`
	ResultURLCamel        string `json:"resultUrl"`
	OutputURL             string `json:"output_url"`
	OutputURLCamel        string `json:"outputUrl"`
	URL                   string `json:"url"`
}

type transcriptFilePayload struct {
	FileURL     string                     `json:"file_url"`
	Transcripts []transcriptChannelPayload `json:"transcripts"`
}

type transcriptChannelPayload struct {
	ChannelID         int                         `json:"channel_id"`
	ContentDurationMS int                         `json:"content_duration_in_milliseconds"`
	Text              string                      `json:"text"`
	Sentences         []transcriptSentencePayload `json:"sentences"`
}

type transcriptSentencePayload struct {
	BeginTime int                     `json:"begin_time"`
	EndTime   int                     `json:"end_time"`
	Text      string                  `json:"text"`
	SpeakerID *int                    `json:"speaker_id"`
	Words     []transcriptWordPayload `json:"words"`
}

type transcriptWordPayload struct {
	BeginTime   int      `json:"begin_time"`
	EndTime     int      `json:"end_time"`
	Text        string   `json:"text"`
	Punctuation string   `json:"punctuation"`
	Confidence  *float64 `json:"confidence"`
}

func (output dashScopeTaskOutput) normalizedResults() []dashScopeTaskResult {
	if len(output.Results) > 0 {
		return output.Results
	}
	if len(output.Result) > 0 {
		return output.Result
	}
	if len(output.Items) > 0 {
		return output.Items
	}
	return nil
}

func (result dashScopeTaskResult) normalizedFileURL() string {
	return firstNonEmpty(result.FileURL, result.FileURLCamel, result.AudioURL, result.AudioURLCamel)
}

func (result dashScopeTaskResult) normalizedStatus() string {
	return firstNonEmpty(result.SubtaskStatus, result.SubtaskStatusCamel, result.Status, result.TaskStatus)
}

func (result dashScopeTaskResult) normalizedCode() string {
	return firstNonEmpty(result.Code, result.ErrorCode, result.ErrorCodeCamel)
}

func (result dashScopeTaskResult) normalizedMessage() string {
	return firstNonEmpty(result.Message, result.ErrorMessage, result.ErrorMessageCamel)
}

func (result dashScopeTaskResult) normalizedTranscriptionURL() string {
	return firstNonEmpty(result.TranscriptionURL, result.TranscriptionURLCamel, result.ResultURL, result.ResultURLCamel, result.OutputURL, result.OutputURLCamel, result.URL)
}

func extractFallbackTranscriptText(raw map[string]any) string {
	if len(raw) == 0 {
		return ""
	}
	if text := firstNonEmpty(
		strings.TrimSpace(toString(raw["text"])),
		strings.TrimSpace(toString(raw["transcript"])),
		strings.TrimSpace(toString(raw["transcription"])),
		strings.TrimSpace(toString(raw["result_text"])),
	); text != "" {
		return text
	}
	for _, key := range []string{"output", "result", "data"} {
		nested, ok := raw[key].(map[string]any)
		if ok {
			if text := extractFallbackTranscriptText(nested); text != "" {
				return text
			}
		}
	}
	for _, key := range []string{"sentences", "segments", "results", "transcripts"} {
		items, ok := raw[key].([]any)
		if ok {
			if text := joinFallbackTextItems(items); text != "" {
				return text
			}
		}
	}
	return ""
}

func joinFallbackTextItems(items []any) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		switch value := item.(type) {
		case string:
			if normalized := strings.TrimSpace(value); normalized != "" {
				parts = append(parts, normalized)
			}
		case map[string]any:
			text := firstNonEmpty(strings.TrimSpace(toString(value["text"])), strings.TrimSpace(toString(value["transcript"])), strings.TrimSpace(toString(value["transcription"])))
			if text == "" {
				text = extractFallbackTranscriptText(value)
			}
			if text != "" {
				parts = append(parts, text)
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}
