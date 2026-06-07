package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mp4-md/internal/media"
	"mp4-md/internal/model"
)

type fakeExtractor struct{}

func (fakeExtractor) Extract(_ context.Context, _ string) (media.Audio, error) {
	return media.Audio{
		SampleRate: 16000,
		Channels:   1,
		PCM:        []byte{0, 0, 1, 0},
	}, nil
}

type fakeTranscriber struct{}

func (fakeTranscriber) Transcribe(_ context.Context, inputPath string, _ media.Audio) (model.Transcript, error) {
	return model.Transcript{Text: "transcript for " + filepath.Base(inputPath)}, nil
}

func TestProcessor_ProcessDirectory(t *testing.T) {
	outDir := t.TempDir()
	inputDir := createSupportedMediaFiles(t)
	processor := Processor{
		Extractor:   fakeExtractor{},
		Transcriber: fakeTranscriber{},
		OutDir:      outDir,
		Workers:     2,
	}

	results, err := processor.Process(context.Background(), []string{inputDir})
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("results len = %d, want 3", len(results))
	}

	for _, item := range results {
		data, readErr := os.ReadFile(item.OutputPath)
		if readErr != nil {
			t.Fatalf("read output %s: %v", item.OutputPath, readErr)
		}
		if !strings.Contains(string(data), "transcript for "+filepath.Base(item.InputPath)) {
			t.Fatalf("output %s missing transcript text", item.OutputPath)
		}
	}
}

func TestExpandInputs_IncludesCommonAudioAndVideo(t *testing.T) {
	paths, err := ExpandInputs([]string{createSupportedMediaFiles(t)})
	if err != nil {
		t.Fatalf("ExpandInputs returned error: %v", err)
	}
	joined := strings.Join(paths, "\n")
	for _, want := range []string{
		"audio-clip-15s.wav",
		"video-clip-15s-with-audio.mp4",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("paths missing %s: %s", want, joined)
		}
	}
}

func createSupportedMediaFiles(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{
		"audio-clip-15s.wav",
		"video-clip-15s-with-audio.mp4",
		"ignore.txt",
		"meeting.m4a",
	} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
			t.Fatalf("write fixture %s: %v", path, err)
		}
	}
	return dir
}
