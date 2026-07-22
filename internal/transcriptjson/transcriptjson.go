// Package transcriptjson 把 model.Transcript 序列化成稳定的结构化 JSON，
// 供下游工具（如 Paper Edit Studio 的剪辑流水线）消费词级时间戳，
// 而不必解析渲染后的 Markdown。
package transcriptjson

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"mp4-md/internal/model"
)

// SchemaVersion 标识 JSON 结构版本；下游按此判断兼容性。
const SchemaVersion = "video2md/transcript@1"

type Word struct {
	Text        string  `json:"text"`
	Punctuation string  `json:"punctuation,omitempty"`
	BeginMS     int     `json:"begin_ms"`
	EndMS       int     `json:"end_ms"`
	Confidence  float64 `json:"confidence"`
}

type Segment struct {
	Index     int    `json:"index"`
	Text      string `json:"text"`
	BeginMS   int    `json:"begin_ms"`
	EndMS     int    `json:"end_ms"`
	SpeakerID *int   `json:"speaker_id"`
	Words     []Word `json:"words"`
}

type Document struct {
	Schema   string    `json:"schema"`
	Source   string    `json:"source"`
	TaskID   string    `json:"task_id,omitempty"`
	Text     string    `json:"text"`
	Segments []Segment `json:"segments"`
}

// Build 从 model.Transcript 构造可序列化的文档。
func Build(sourcePath string, transcript model.Transcript) Document {
	segments := make([]Segment, 0, len(transcript.Segments))
	for i, segment := range transcript.Segments {
		words := make([]Word, 0, len(segment.Words))
		for _, word := range segment.Words {
			words = append(words, Word{
				Text:        word.Text,
				Punctuation: word.Punctuation,
				BeginMS:     word.BeginMS,
				EndMS:       word.EndMS,
				Confidence:  word.Confidence,
			})
		}
		segments = append(segments, Segment{
			Index:     i + 1,
			Text:      strings.TrimSpace(segment.Text),
			BeginMS:   segment.BeginMS,
			EndMS:     segment.EndMS,
			SpeakerID: segment.SpeakerID,
			Words:     words,
		})
	}
	return Document{
		Schema:   SchemaVersion,
		Source:   filepath.Base(sourcePath),
		TaskID:   strings.TrimSpace(transcript.TaskID),
		Text:     strings.TrimSpace(transcript.Text),
		Segments: segments,
	}
}

// Marshal 输出缩进后的 JSON（便于调试与 diff）。
func Marshal(sourcePath string, transcript model.Transcript) ([]byte, error) {
	return json.MarshalIndent(Build(sourcePath, transcript), "", "  ")
}
