package asrclient

import (
	"context"
	"errors"
	"strings"

	"mp4-md/internal/media"
	"mp4-md/internal/model"
)

type Client struct {
	APIKey    string
	Language  string
	ChunkSize int
}

func (c Client) Transcribe(ctx context.Context, _ string, audio media.Audio) (model.Transcript, error) {
	if strings.TrimSpace(c.APIKey) == "" {
		return model.Transcript{}, errors.New("asrclient: api key is required")
	}
	return model.Transcript{}, errors.New("asrclient: realtime transcription is not included in this CLI build; use RecordedClient")
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
