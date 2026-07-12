package markdown

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"mp4-md/internal/model"
)

// Timestamps 控制时间戳粒度。
type Timestamps string

const (
	TimestampsNone     Timestamps = "none"     // 不带时间戳（正文默认）
	TimestampsSentence Timestamps = "sentence" // 每句一个起止时间
	TimestampsWord     Timestamps = "word"     // 句级 + 逐词时间与置信度
)

type Options struct {
	SpeakerNames map[int]string
	// PlainText 去掉「说话人N」前缀，只留文本。
	PlainText bool
	// Timestamps 决定是否标注时间，缺省 TimestampsNone。
	Timestamps Timestamps
}

func Render(sourcePath string, transcript model.Transcript, options ...Options) string {
	opts := Options{}
	if len(options) > 0 {
		opts = options[0]
	}

	base := filepath.Base(sourcePath)
	title := strings.TrimSuffix(base, filepath.Ext(base))

	var body strings.Builder
	body.WriteString("# ")
	body.WriteString(title)
	body.WriteString("\n\n")
	body.WriteString(fmt.Sprintf("来源：`%s`\n\n", base))
	if strings.TrimSpace(transcript.TaskID) != "" {
		body.WriteString(fmt.Sprintf("任务：`%s`\n\n", strings.TrimSpace(transcript.TaskID)))
	}
	if len(transcript.Segments) > 0 {
		writeSegments(&body, transcript.Segments, opts)
	} else {
		body.WriteString(strings.TrimSpace(transcript.Text))
	}
	body.WriteString("\n")
	return body.String()
}

func writeSegments(body *strings.Builder, segments []model.Segment, opts Options) {
	withTimestamps := opts.Timestamps == TimestampsSentence || opts.Timestamps == TimestampsWord

	for _, segment := range segments {
		text := strings.TrimSpace(segment.Text)
		if text == "" {
			continue
		}

		prefix := make([]string, 0, 2)
		if segment.SpeakerID != nil && !opts.PlainText {
			prefix = append(prefix, "**"+speakerLabel(*segment.SpeakerID, opts.SpeakerNames)+"**")
		}
		if withTimestamps {
			prefix = append(prefix, fmt.Sprintf(
				"`[%s → %s]`",
				formatTimestamp(segment.BeginMS), formatTimestamp(segment.EndMS),
			))
		}
		if len(prefix) > 0 {
			body.WriteString(strings.Join(prefix, " "))
			body.WriteString(": ")
		}
		body.WriteString(text)
		body.WriteString("\n\n")

		if opts.Timestamps == TimestampsWord && len(segment.Words) > 0 {
			writeWords(body, segment.Words)
		}
	}
}

func writeWords(body *strings.Builder, words []model.Word) {
	body.WriteString("| 起 | 止 | 词 | 置信度 |\n|---|---|---|---|\n")
	for _, word := range words {
		body.WriteString(fmt.Sprintf(
			"| %s | %s | %s | %.3f |\n",
			formatTimestamp(word.BeginMS), formatTimestamp(word.EndMS),
			word.Text+word.Punctuation, word.Confidence,
		))
	}
	body.WriteString("\n")
}

// formatTimestamp 把毫秒转成 HH:MM:SS.mmm。
func formatTimestamp(totalMS int) string {
	if totalMS < 0 {
		totalMS = 0
	}
	ms := totalMS % 1000
	totalSeconds := totalMS / 1000
	seconds := totalSeconds % 60
	minutes := (totalSeconds / 60) % 60
	hours := totalSeconds / 3600
	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, seconds, ms)
}

func speakerLabel(speakerID int, speakerNames map[int]string) string {
	displayID := speakerID
	if speakerID >= 0 {
		displayID = speakerID + 1
	}
	if speakerNames != nil {
		if name := strings.TrimSpace(speakerNames[displayID]); name != "" {
			return name
		}
	}
	return "说话人" + strconv.Itoa(displayID)
}
