package transcriptjson

import (
	"encoding/json"
	"testing"

	"mp4-md/internal/model"
)

func TestBuildMapsWordsAndTimestamps(t *testing.T) {
	speaker := 2
	transcript := model.Transcript{
		Text:   "你好 世界",
		TaskID: "task-123",
		Segments: []model.Segment{
			{
				Text:      "你好",
				BeginMS:   160,
				EndMS:     480,
				SpeakerID: &speaker,
				Words: []model.Word{
					{Text: "你好", Punctuation: "，", BeginMS: 160, EndMS: 480, Confidence: 0.98},
				},
			},
		},
	}

	doc := Build("/tmp/meeting.mp4", transcript)

	if doc.Schema != SchemaVersion {
		t.Fatalf("schema = %q, want %q", doc.Schema, SchemaVersion)
	}
	if doc.Source != "meeting.mp4" {
		t.Fatalf("source = %q, want meeting.mp4", doc.Source)
	}
	if doc.TaskID != "task-123" {
		t.Fatalf("task_id = %q, want task-123", doc.TaskID)
	}
	if len(doc.Segments) != 1 {
		t.Fatalf("segments = %d, want 1", len(doc.Segments))
	}
	seg := doc.Segments[0]
	if seg.Index != 1 || seg.BeginMS != 160 || seg.EndMS != 480 {
		t.Fatalf("segment fields unexpected: %+v", seg)
	}
	if seg.SpeakerID == nil || *seg.SpeakerID != 2 {
		t.Fatalf("speaker_id = %v, want 2", seg.SpeakerID)
	}
	if len(seg.Words) != 1 {
		t.Fatalf("words = %d, want 1", len(seg.Words))
	}
	w := seg.Words[0]
	if w.Text != "你好" || w.Punctuation != "，" || w.BeginMS != 160 || w.EndMS != 480 || w.Confidence != 0.98 {
		t.Fatalf("word fields unexpected: %+v", w)
	}
}

func TestMarshalRoundTrips(t *testing.T) {
	transcript := model.Transcript{
		Segments: []model.Segment{
			{Text: "只有句级", BeginMS: 0, EndMS: 1000},
		},
	}
	payload, err := Marshal("clip.mov", transcript)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var doc Document
	if err := json.Unmarshal(payload, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(doc.Segments) != 1 || doc.Segments[0].Words == nil {
		// words 应为空数组而非 null，保证下游语言解析一致。
		t.Fatalf("expected non-nil empty words slice, got %+v", doc.Segments)
	}
}
