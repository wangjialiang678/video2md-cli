package asrclient

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"mp4-md/internal/asrbatch"
	"mp4-md/internal/media"
	"mp4-md/internal/model"
	"mp4-md/internal/storage"
)

type ObjectUploader interface {
	Upload(ctx context.Context, filePath string) (storage.UploadedObject, error)
	Delete(ctx context.Context, objectKey string) error
}

type RecordedProvider interface {
	Recognize(ctx context.Context, request asrbatch.Request) (asrbatch.Result, error)
}

type RecordedClient struct {
	APIKey        string
	Uploader      ObjectUploader
	Provider      RecordedProvider
	Model         string
	VocabularyID  string
	LanguageHints []string
	SpeakerCount  int
}

func (c RecordedClient) Transcribe(ctx context.Context, _ string, audio media.Audio) (model.Transcript, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	audioFiles := normalizedAudioFiles(audio)
	if len(audioFiles) == 0 {
		return model.Transcript{}, errors.New("asrclient: extracted audio file path is required for recorded transcription")
	}
	if c.Uploader == nil {
		return model.Transcript{}, errors.New("asrclient: object uploader is required")
	}

	provider, err := c.recordedProvider()
	if err != nil {
		return model.Transcript{}, err
	}

	uploads := make([]storage.UploadedObject, 0, len(audioFiles))
	fileURLs := make([]string, 0, len(audioFiles))
	offsetsByURL := make(map[string]int, len(audioFiles))
	for _, audioFile := range audioFiles {
		uploaded, err := c.Uploader.Upload(ctx, audioFile.Path)
		if err != nil {
			return model.Transcript{}, err
		}
		uploads = append(uploads, uploaded)
		fileURLs = append(fileURLs, uploaded.ReadURL)
		offsetsByURL[uploaded.ReadURL] = int(audioFile.Offset.Milliseconds())
	}
	defer c.deleteUploadedObjects(uploads)

	diarizationEnabled := true
	speakerCount := c.SpeakerCount
	if speakerCount <= 0 {
		speakerCount = 2
	}

	results := make([]model.Transcript, 0, len(fileURLs))
	for _, fileURL := range fileURLs {
		result, err := provider.Recognize(ctx, asrbatch.Request{
			FileURLs:           []string{fileURL},
			Model:              strings.TrimSpace(c.Model),
			VocabularyID:       strings.TrimSpace(c.VocabularyID),
			LanguageHints:      cleanStrings(c.LanguageHints),
			DiarizationEnabled: &diarizationEnabled,
			SpeakerCount:       &speakerCount,
		})
		if err != nil {
			return model.Transcript{}, err
		}
		results = append(results, transcriptFromBatchResult(result, offsetsByURL))
	}

	return mergeTranscripts(results), nil
}

func (c RecordedClient) deleteUploadedObjects(uploads []storage.UploadedObject) {
	for _, uploaded := range uploads {
		if err := c.Uploader.Delete(context.Background(), uploaded.ObjectKey); err != nil {
			log.Printf("warning: delete oss object failed: %v", err)
		}
	}
}

func (c RecordedClient) recordedProvider() (RecordedProvider, error) {
	if c.Provider != nil {
		return c.Provider, nil
	}
	apiKey := strings.TrimSpace(c.APIKey)
	if apiKey == "" {
		return nil, errors.New("asrclient: api key is required")
	}
	return asrbatch.NewRecordedProvider(asrbatch.FunASRRecordedConfig{
		APIKey: apiKey,
		Model:  strings.TrimSpace(c.Model),
	})
}

func transcriptFromBatchResult(result asrbatch.Result, offsetsByURL ...map[string]int) model.Transcript {
	segments := make([]model.Segment, 0)
	textParts := make([]string, 0)
	offsets := map[string]int(nil)
	if len(offsetsByURL) > 0 {
		offsets = offsetsByURL[0]
	}

	for _, transcript := range result.Transcripts {
		offsetMs := offsets[strings.TrimSpace(transcript.FileURL)]
		if strings.TrimSpace(transcript.Text) != "" {
			textParts = append(textParts, strings.TrimSpace(transcript.Text))
		}
		segments = append(segments, convertSegments(transcript.Segments, offsetMs)...)
		for _, channel := range transcript.Channels {
			if strings.TrimSpace(channel.Text) != "" {
				textParts = append(textParts, strings.TrimSpace(channel.Text))
			}
			segments = append(segments, convertSegments(channel.Segments, offsetMs)...)
		}
	}

	if len(textParts) == 0 && len(segments) > 0 {
		for _, segment := range segments {
			if strings.TrimSpace(segment.Text) != "" {
				textParts = append(textParts, strings.TrimSpace(segment.Text))
			}
		}
	}

	return model.Transcript{
		Text:     strings.Join(textParts, "\n"),
		Segments: dedupeSegments(segments),
		TaskID:   result.TaskID,
	}
}

func mergeTranscripts(input []model.Transcript) model.Transcript {
	textParts := make([]string, 0, len(input))
	segments := make([]model.Segment, 0)
	taskIDs := make([]string, 0, len(input))
	for _, transcript := range input {
		if text := strings.TrimSpace(transcript.Text); text != "" {
			textParts = append(textParts, text)
		}
		if taskID := strings.TrimSpace(transcript.TaskID); taskID != "" {
			taskIDs = append(taskIDs, taskID)
		}
		segments = append(segments, transcript.Segments...)
	}
	return model.Transcript{
		Text:     strings.Join(textParts, "\n"),
		Segments: dedupeSegments(segments),
		TaskID:   strings.Join(taskIDs, ","),
	}
}

func convertSegments(input []asrbatch.Segment, offsetMs int) []model.Segment {
	output := make([]model.Segment, 0, len(input))
	for _, segment := range input {
		text := strings.TrimSpace(segment.Text)
		if text == "" {
			continue
		}
		var speakerID *int
		if segment.SpeakerID != nil {
			value := *segment.SpeakerID
			speakerID = &value
		}
		output = append(output, model.Segment{
			Text:      text,
			BeginMS:   segment.BeginMS + offsetMs,
			EndMS:     segment.EndMS + offsetMs,
			SpeakerID: speakerID,
		})
	}
	return output
}

func normalizedAudioFiles(audio media.Audio) []media.AudioFile {
	if len(audio.Files) > 0 {
		output := make([]media.AudioFile, 0, len(audio.Files))
		for _, file := range audio.Files {
			if strings.TrimSpace(file.Path) != "" {
				output = append(output, file)
			}
		}
		return output
	}
	if strings.TrimSpace(audio.FilePath) == "" {
		return nil
	}
	return []media.AudioFile{{Path: audio.FilePath}}
}

func cleanStrings(input []string) []string {
	output := make([]string, 0, len(input))
	for _, item := range input {
		if value := strings.TrimSpace(item); value != "" {
			output = append(output, value)
		}
	}
	return output
}

func dedupeSegments(input []model.Segment) []model.Segment {
	output := make([]model.Segment, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, segment := range input {
		key := segmentKey(segment)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		output = append(output, segment)
	}
	return output
}

func segmentKey(segment model.Segment) string {
	speakerID := -1
	if segment.SpeakerID != nil {
		speakerID = *segment.SpeakerID
	}
	return fmt.Sprintf("%d:%d:%d:%s", segment.BeginMS, segment.EndMS, speakerID, strings.TrimSpace(segment.Text))
}
