package asrbatch

import (
	"context"
	"errors"
	"strings"
	"time"
)

var ErrFileURLRequired = errors.New("asrbatch: fileURLs is required")

type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "PENDING"
	TaskStatusRunning   TaskStatus = "RUNNING"
	TaskStatusSucceeded TaskStatus = "SUCCEEDED"
	TaskStatusFailed    TaskStatus = "FAILED"
	TaskStatusUnknown   TaskStatus = "UNKNOWN"
)

type Request struct {
	FileURLs           []string
	Model              string
	VocabularyID       string
	LanguageHints      []string
	DiarizationEnabled *bool
	SpeakerCount       *int
	Parameters         map[string]any
	PollInterval       time.Duration
	WaitTimeout        time.Duration
}

type Result struct {
	Provider    string
	TaskID      string
	TaskStatus  TaskStatus
	RequestID   string
	SubmitTime  string
	ScheduledAt string
	EndTime     string
	Usage       *Usage
	Subtasks    []Subtask
	Transcripts []FileTranscript
}

type Usage struct {
	Duration *float64
	Raw      map[string]any
}

type Subtask struct {
	FileURL          string
	Status           TaskStatus
	Code             string
	Message          string
	TranscriptionURL string
}

type FileTranscript struct {
	FileURL   string
	Text      string
	Channels  []ChannelTranscript
	Segments  []Segment
	Raw       map[string]any
	RequestID string
}

type ChannelTranscript struct {
	ChannelID  int
	Text       string
	DurationMS int
	Segments   []Segment
}

type Word struct {
	Text        string
	BeginMS     int
	EndMS       int
	Confidence  *float64
	Punctuation *string
}

type Segment struct {
	Text      string
	BeginMS   int
	EndMS     int
	IsFinal   bool
	SpeakerID *int
	Words     []Word
}

type Provider interface {
	Name() string
	Recognize(ctx context.Context, request Request) (Result, error)
}

func NormalizeTaskStatus(value string) TaskStatus {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case string(TaskStatusPending):
		return TaskStatusPending
	case string(TaskStatusRunning):
		return TaskStatusRunning
	case string(TaskStatusSucceeded):
		return TaskStatusSucceeded
	case string(TaskStatusFailed):
		return TaskStatusFailed
	default:
		return TaskStatusUnknown
	}
}

func (r Request) Validate() error {
	if len(r.FileURLs) == 0 {
		return ErrFileURLRequired
	}
	for _, item := range r.FileURLs {
		if strings.TrimSpace(item) != "" {
			return nil
		}
	}
	return ErrFileURLRequired
}

func (r Request) SanitizedFileURLs() []string {
	output := make([]string, 0, len(r.FileURLs))
	for _, item := range r.FileURLs {
		if normalized := strings.TrimSpace(item); normalized != "" {
			output = append(output, normalized)
		}
	}
	return output
}
