package markdown

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"mp4-md/internal/model"
)

type Options struct {
	SpeakerNames map[int]string
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
		writeSegments(&body, transcript.Segments, opts.SpeakerNames)
	} else {
		body.WriteString(strings.TrimSpace(transcript.Text))
	}
	body.WriteString("\n")
	return body.String()
}

func writeSegments(body *strings.Builder, segments []model.Segment, speakerNames map[int]string) {
	for _, segment := range segments {
		text := strings.TrimSpace(segment.Text)
		if text == "" {
			continue
		}
		if segment.SpeakerID != nil {
			body.WriteString("**")
			body.WriteString(speakerLabel(*segment.SpeakerID, speakerNames))
			body.WriteString("**: ")
		}
		body.WriteString(text)
		body.WriteString("\n\n")
	}
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
