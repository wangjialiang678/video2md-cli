package markdown

import (
	"strings"
	"testing"

	"mp4-md/internal/model"
)

func TestRender(t *testing.T) {
	got := Render("demo.mp4", model.Transcript{
		Text: "第一行\n第二行",
	})

	if !strings.Contains(got, "# demo") {
		t.Fatalf("markdown missing title: %q", got)
	}
	if !strings.Contains(got, "来源：`demo.mp4`") {
		t.Fatalf("markdown missing source: %q", got)
	}
	if !strings.Contains(got, "第一行\n第二行") {
		t.Fatalf("markdown missing transcript: %q", got)
	}
}

func TestRender_WithSpeakerSegments(t *testing.T) {
	speakerZero := 0
	speakerOne := 1

	got := Render("meeting.mp4", model.Transcript{
		TaskID: "task-123",
		Segments: []model.Segment{
			{Text: "我们开始吧", BeginMS: 0, EndMS: 1200, SpeakerID: &speakerZero},
			{Text: "好的", BeginMS: 1300, EndMS: 1800, SpeakerID: &speakerOne},
		},
	}, Options{
		SpeakerNames: map[int]string{
			1: "我",
			2: "张三",
		},
	})

	if !strings.Contains(got, "任务：`task-123`") {
		t.Fatalf("markdown missing task id: %q", got)
	}
	if !strings.Contains(got, "**我**: 我们开始吧") {
		t.Fatalf("markdown missing speaker one segment: %q", got)
	}
	if !strings.Contains(got, "**张三**: 好的") {
		t.Fatalf("markdown missing speaker two segment: %q", got)
	}
}

func TestRender_MapsZeroBasedProviderSpeakerIDsToOneBasedNames(t *testing.T) {
	speakerZero := 0
	speakerOne := 1

	got := Render("meeting.mp4", model.Transcript{
		Segments: []model.Segment{
			{Text: "第一句", SpeakerID: &speakerZero},
			{Text: "第二句", SpeakerID: &speakerOne},
		},
	}, Options{
		SpeakerNames: map[int]string{
			1: "我",
			2: "同事",
		},
	})

	if !strings.Contains(got, "**我**: 第一句") {
		t.Fatalf("markdown did not map zero-based speaker id: %q", got)
	}
	if !strings.Contains(got, "**同事**: 第二句") {
		t.Fatalf("markdown did not map second zero-based speaker id: %q", got)
	}
}

func TestRender_UsesOneBasedDefaultLabelsForZeroBasedProviderIDs(t *testing.T) {
	speakerZero := 0
	speakerOne := 1

	got := Render("meeting.mp4", model.Transcript{
		Segments: []model.Segment{
			{Text: "第一句", SpeakerID: &speakerZero},
			{Text: "第二句", SpeakerID: &speakerOne},
		},
	})

	if !strings.Contains(got, "**说话人1**: 第一句") {
		t.Fatalf("markdown missing one-based label for speaker 0: %q", got)
	}
	if !strings.Contains(got, "**说话人2**: 第二句") {
		t.Fatalf("markdown missing one-based label for speaker 1: %q", got)
	}
}

func TestRender_UsesOneBasedLabelsWhenOnlySecondZeroBasedSpeakerAppears(t *testing.T) {
	speakerOne := 1

	got := Render("meeting.mp4", model.Transcript{
		Segments: []model.Segment{
			{Text: "第一句", SpeakerID: &speakerOne},
		},
	}, Options{
		SpeakerNames: map[int]string{
			2: "同事",
		},
	})

	if !strings.Contains(got, "**同事**: 第一句") {
		t.Fatalf("markdown did not map single nonzero zero-based speaker id: %q", got)
	}
}

func TestRender_DoesNotUseDirectProviderIDAsSpeakerNameFallback(t *testing.T) {
	speakerOne := 1

	got := Render("meeting.mp4", model.Transcript{
		Segments: []model.Segment{
			{Text: "第一句", SpeakerID: &speakerOne},
		},
	}, Options{
		SpeakerNames: map[int]string{
			1: "我",
		},
	})

	if strings.Contains(got, "**我**") {
		t.Fatalf("markdown unexpectedly used direct provider id mapping: %q", got)
	}
	if !strings.Contains(got, "**说话人2**: 第一句") {
		t.Fatalf("markdown missing one-based label for speaker 1: %q", got)
	}
}
