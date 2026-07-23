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

type fakeWordTranscriber struct{}

func (fakeWordTranscriber) Transcribe(_ context.Context, inputPath string, _ media.Audio) (model.Transcript, error) {
	return model.Transcript{
		Text:   "你好世界",
		TaskID: "task-xyz",
		Segments: []model.Segment{
			{
				Text:    "你好世界",
				BeginMS: 100,
				EndMS:   900,
				Words: []model.Word{
					{Text: "你好", BeginMS: 100, EndMS: 500, Confidence: 0.9},
					{Text: "世界", BeginMS: 500, EndMS: 900, Confidence: 0.8},
				},
			},
		},
	}, nil
}

func TestProcessor_EmitJSONWritesTranscriptFile(t *testing.T) {
	outDir := t.TempDir()
	inputDir := t.TempDir()
	input := filepath.Join(inputDir, "clip.mp4")
	if err := os.WriteFile(input, []byte("test"), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	processor := Processor{
		Extractor:   fakeExtractor{},
		Transcriber: fakeWordTranscriber{},
		OutDir:      outDir,
		Workers:     1,
		EmitJSON:    true,
	}
	results, err := processor.Process(context.Background(), []string{input})
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results len = %d, want 1", len(results))
	}
	jsonPath := results[0].TranscriptJSONPath
	if jsonPath == "" {
		t.Fatalf("TranscriptJSONPath is empty")
	}
	if filepath.Base(jsonPath) != "clip.transcript.json" {
		t.Fatalf("json basename = %s, want clip.transcript.json", filepath.Base(jsonPath))
	}
	data, readErr := os.ReadFile(jsonPath)
	if readErr != nil {
		t.Fatalf("read json %s: %v", jsonPath, readErr)
	}
	for _, want := range []string{"video2md/transcript@1", "\"begin_ms\": 100", "\"你好\"", "\"confidence\": 0.9"} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("json missing %q:\n%s", want, string(data))
		}
	}
}

func TestProcessor_SkipExistingDoesNotSkipMissingJSON(t *testing.T) {
	outDir := t.TempDir()
	inputDir := t.TempDir()
	input := filepath.Join(inputDir, "clip.mp4")
	if err := os.WriteFile(input, []byte("test"), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}
	// 预置一个已存在的 .md，但没有 .transcript.json：模拟“先前不带 --emit-json 跑过”。
	mdPath := filepath.Join(outDir, "clip.md")
	if err := os.WriteFile(mdPath, []byte("stale markdown"), 0o644); err != nil {
		t.Fatalf("write stale md: %v", err)
	}

	processor := Processor{
		Extractor:    fakeExtractor{},
		Transcriber:  fakeWordTranscriber{},
		OutDir:       outDir,
		Workers:      1,
		SkipExisting: true,
		EmitJSON:     true,
	}
	results, err := processor.Process(context.Background(), []string{input})
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results len = %d, want 1", len(results))
	}

	// 缺 JSON 时不应跳过：JSON 必须被补产出。
	jsonPath := results[0].TranscriptJSONPath
	if jsonPath == "" {
		t.Fatalf("TranscriptJSONPath 为空：skip-existing 错误地跳过且从未产出 JSON")
	}
	if _, statErr := os.Stat(jsonPath); statErr != nil {
		t.Fatalf("transcript json 未写出: %v", statErr)
	}
	// .md 应被重新渲染（不再是预置的 stale 内容）。
	data, readErr := os.ReadFile(mdPath)
	if readErr != nil {
		t.Fatalf("read md: %v", readErr)
	}
	if strings.Contains(string(data), "stale markdown") {
		t.Fatalf("markdown 未被重新生成，仍是旧内容")
	}
}

func TestProcessor_SkipExistingSkipsWhenAllArtifactsExist(t *testing.T) {
	outDir := t.TempDir()
	inputDir := t.TempDir()
	input := filepath.Join(inputDir, "clip.mp4")
	if err := os.WriteFile(input, []byte("test"), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}
	// .md 与 .transcript.json 都已存在：应被跳过，且 Output 仍回填 JSON 路径、不重写文件。
	mdPath := filepath.Join(outDir, "clip.md")
	jsonPath := filepath.Join(outDir, "clip.transcript.json")
	if err := os.WriteFile(mdPath, []byte("existing md"), 0o644); err != nil {
		t.Fatalf("write md: %v", err)
	}
	if err := os.WriteFile(jsonPath, []byte("existing json"), 0o644); err != nil {
		t.Fatalf("write json: %v", err)
	}

	processor := Processor{
		Extractor:    fakeExtractor{},
		Transcriber:  fakeWordTranscriber{},
		OutDir:       outDir,
		Workers:      1,
		SkipExisting: true,
		EmitJSON:     true,
	}
	results, err := processor.Process(context.Background(), []string{input})
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if results[0].TranscriptJSONPath == "" {
		t.Fatalf("跳过时 Output 仍应回填已存在的 TranscriptJSONPath")
	}
	if data, _ := os.ReadFile(jsonPath); string(data) != "existing json" {
		t.Fatalf("跳过时不应重写 json，实际: %q", string(data))
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
