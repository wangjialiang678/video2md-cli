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

func TestRenderPlainTextOmitsSpeakerLabels(t *testing.T) {
	speaker0, speaker1 := 0, 1
	transcript := model.Transcript{Segments: []model.Segment{
		{Text: "第一句", BeginMS: 0, EndMS: 1000, SpeakerID: &speaker0},
		{Text: "第二句", BeginMS: 1000, EndMS: 2000, SpeakerID: &speaker1},
	}}

	withSpeakers := Render("a.mp4", transcript, Options{})
	if !strings.Contains(withSpeakers, "**说话人1**: 第一句") {
		t.Errorf("default output should keep speaker labels, got:\n%s", withSpeakers)
	}

	plain := Render("a.mp4", transcript, Options{PlainText: true})
	if strings.Contains(plain, "说话人") || strings.Contains(plain, "**") {
		t.Errorf("--plain output should not contain speaker labels, got:\n%s", plain)
	}
	for _, want := range []string{"第一句", "第二句"} {
		if !strings.Contains(plain, want) {
			t.Errorf("--plain output missing %q, got:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, ":00") || strings.Contains(plain, "1000") {
		t.Errorf("output should never contain timestamps, got:\n%s", plain)
	}
}

func TestRenderSentenceTimestamps(t *testing.T) {
	speaker := 0
	transcript := model.Transcript{Segments: []model.Segment{
		{Text: "你好", BeginMS: 160, EndMS: 4480, SpeakerID: &speaker},
	}}

	out := Render("a.mp4", transcript, Options{Timestamps: TimestampsSentence})
	if !strings.Contains(out, "`[00:00:00.160 → 00:00:04.480]`") {
		t.Errorf("sentence timestamps missing, got:\n%s", out)
	}
	if !strings.Contains(out, "**说话人1**") {
		t.Errorf("speaker label should survive alongside timestamps, got:\n%s", out)
	}
	if strings.Contains(out, "| 起 |") {
		t.Errorf("sentence mode must not emit the word table, got:\n%s", out)
	}
}

func TestRenderWordTimestamps(t *testing.T) {
	speaker := 0
	transcript := model.Transcript{Segments: []model.Segment{{
		Text: "你好", BeginMS: 160, EndMS: 600, SpeakerID: &speaker,
		Words: []model.Word{
			{Text: "你好", Punctuation: "，", BeginMS: 160, EndMS: 600, Confidence: 0.98},
		},
	}}}

	out := Render("a.mp4", transcript, Options{Timestamps: TimestampsWord})
	for _, want := range []string{"| 起 | 止 | 词 | 置信度 |", "00:00:00.160", "你好，", "0.980"} {
		if !strings.Contains(out, want) {
			t.Errorf("word table missing %q, got:\n%s", want, out)
		}
	}
}

func TestRenderTimestampsDefaultOff(t *testing.T) {
	speaker := 0
	transcript := model.Transcript{Segments: []model.Segment{
		{Text: "你好", BeginMS: 160, EndMS: 4480, SpeakerID: &speaker},
	}}
	out := Render("a.mp4", transcript, Options{})
	if strings.Contains(out, "00:00:") || strings.Contains(out, "→") {
		t.Errorf("default Render must stay timestamp-free, got:\n%s", out)
	}
}

func TestFormatTimestampCrossesHour(t *testing.T) {
	cases := map[int]string{
		0:        "00:00:00.000",
		1500:     "00:00:01.500",
		61_000:   "00:01:01.000",
		3_723_45: "00:06:12.345",
		7_200_00: "00:12:00.000",
	}
	for ms, want := range cases {
		if got := formatTimestamp(ms); got != want {
			t.Errorf("formatTimestamp(%d) = %s, want %s", ms, got, want)
		}
	}
	if got := formatTimestamp(3_661_500); got != "01:01:01.500" {
		t.Errorf("formatTimestamp(3661500) = %s, want 01:01:01.500", got)
	}
}
