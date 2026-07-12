package model

// Word 是词级识别结果：DashScope 对每个词都返回起止时间与置信度。
type Word struct {
	Text        string
	Punctuation string
	BeginMS     int
	EndMS       int
	Confidence  float64
}

type Segment struct {
	Text      string
	BeginMS   int
	EndMS     int
	SpeakerID *int
	Words     []Word
}

type Transcript struct {
	Text     string
	Segments []Segment
	TaskID   string
}
