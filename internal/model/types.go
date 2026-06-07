package model

type Segment struct {
	Text      string
	BeginMS   int
	EndMS     int
	SpeakerID *int
}

type Transcript struct {
	Text     string
	Segments []Segment
	TaskID   string
}
