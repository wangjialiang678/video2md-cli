package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"mp4-md/internal/markdown"
	"mp4-md/internal/media"
	"mp4-md/internal/model"
	"mp4-md/internal/transcriptjson"
)

type Extractor interface {
	Extract(ctx context.Context, inputPath string) (media.Audio, error)
}

type Transcriber interface {
	Transcribe(ctx context.Context, inputPath string, audio media.Audio) (model.Transcript, error)
}

type Processor struct {
	Extractor    Extractor
	Transcriber  Transcriber
	OutDir       string
	Workers      int
	SkipExisting bool
	SpeakerNames map[int]string
	PlainText    bool
	// Timestamps 控制额外产出的时间戳版文件；为空或 none 则不产出。
	Timestamps markdown.Timestamps
	// EmitJSON 为 true 时额外产出结构化转写 JSON（<stem>.transcript.json），
	// 供下游工具消费词级时间戳，无需解析 Markdown。
	EmitJSON bool
}

type Output struct {
	InputPath  string
	OutputPath string
	// TimestampedPath 是带时间戳的伴生文件，未启用时为空。
	TimestampedPath string
	// TranscriptJSONPath 是结构化 JSON 伴生文件，未启用 EmitJSON 时为空。
	TranscriptJSONPath string
}

func (p Processor) Process(ctx context.Context, inputs []string) ([]Output, error) {
	if p.Extractor == nil {
		return nil, errors.New("app: extractor is required")
	}
	if p.Transcriber == nil {
		return nil, errors.New("app: transcriber is required")
	}

	paths, err := ExpandInputs(inputs)
	if err != nil {
		return nil, err
	}

	workerCount := p.Workers
	if workerCount <= 0 {
		workerCount = 1
	}

	type result struct {
		output Output
		err    error
	}

	jobs := make(chan string)
	results := make(chan result, len(paths))

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for inputPath := range jobs {
				output, jobErr := p.processOne(ctx, inputPath)
				results <- result{output: output, err: jobErr}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, path := range paths {
			jobs <- path
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	outputs := make([]Output, 0, len(paths))
	var errs []error
	for item := range results {
		if item.err != nil {
			errs = append(errs, item.err)
			continue
		}
		outputs = append(outputs, item.output)
	}

	sort.Slice(outputs, func(i, j int) bool {
		return outputs[i].InputPath < outputs[j].InputPath
	})

	if len(errs) > 0 {
		return outputs, errors.Join(errs...)
	}
	return outputs, nil
}

func ExpandInputs(inputs []string) ([]string, error) {
	if len(inputs) == 0 {
		return nil, errors.New("app: at least one input path is required")
	}

	seen := make(map[string]struct{})
	var out []string

	add := func(path string) {
		clean := filepath.Clean(path)
		if _, ok := seen[clean]; ok {
			return
		}
		seen[clean] = struct{}{}
		out = append(out, clean)
	}

	for _, input := range inputs {
		info, err := os.Stat(input)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			if isSupportedMediaPath(input) {
				add(input)
			}
			continue
		}

		err = filepath.WalkDir(input, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			if isSupportedMediaPath(path) {
				add(path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	sort.Strings(out)
	if len(out) == 0 {
		return nil, errors.New("app: no supported audio or video files found")
	}
	return out, nil
}

func (p Processor) processOne(ctx context.Context, inputPath string) (Output, error) {
	outputPath := p.outputPath(inputPath)

	// 预先算出本次会产出的所有伴生文件路径，供 skip-existing 判断与最终返回复用。
	timestampedPath := ""
	if p.Timestamps != markdown.TimestampsNone && p.Timestamps != "" {
		timestampedPath = timestampedOutputPath(outputPath)
	}
	transcriptJSONPath := ""
	if p.EmitJSON {
		transcriptJSONPath = transcriptJSONOutputPath(outputPath)
	}

	// 只有当所有请求的产物都已存在时才跳过；否则会漏产（例如已有 .md 但缺 .transcript.json）。
	if p.SkipExisting && allPathsExist(outputPath, timestampedPath, transcriptJSONPath) {
		return Output{
			InputPath:          inputPath,
			OutputPath:         outputPath,
			TimestampedPath:    timestampedPath,
			TranscriptJSONPath: transcriptJSONPath,
		}, nil
	}

	audio, err := p.Extractor.Extract(ctx, inputPath)
	if err != nil {
		return Output{}, fmt.Errorf("%s: %w", inputPath, err)
	}
	if audio.Cleanup != nil {
		defer audio.Cleanup()
	}

	transcript, err := p.Transcriber.Transcribe(ctx, inputPath, audio)
	if err != nil {
		return Output{}, fmt.Errorf("%s: %w", inputPath, err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return Output{}, err
	}
	if err := os.WriteFile(outputPath, []byte(markdown.Render(inputPath, transcript, markdown.Options{
		SpeakerNames: p.SpeakerNames,
		PlainText:    p.PlainText,
	})), 0o644); err != nil {
		return Output{}, err
	}

	// 正文永远是干净文本；时间戳版另存一份，方便按需定位原视频。
	if timestampedPath != "" {
		if err := os.WriteFile(timestampedPath, []byte(markdown.Render(inputPath, transcript, markdown.Options{
			SpeakerNames: p.SpeakerNames,
			PlainText:    p.PlainText,
			Timestamps:   p.Timestamps,
		})), 0o644); err != nil {
			return Output{}, err
		}
	}

	// 结构化 JSON：下游按词级时间戳做剪辑点映射，不解析 Markdown。
	if transcriptJSONPath != "" {
		payload, marshalErr := transcriptjson.Marshal(inputPath, transcript)
		if marshalErr != nil {
			return Output{}, marshalErr
		}
		if err := os.WriteFile(transcriptJSONPath, payload, 0o644); err != nil {
			return Output{}, err
		}
	}
	return Output{
		InputPath:          inputPath,
		OutputPath:         outputPath,
		TimestampedPath:    timestampedPath,
		TranscriptJSONPath: transcriptJSONPath,
	}, nil
}

// allPathsExist 报告所有非空路径是否都已存在；空路径表示该产物本次不产出，跳过检查。
func allPathsExist(paths ...string) bool {
	for _, path := range paths {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err != nil {
			return false
		}
	}
	return true
}

// timestampedOutputPath 把 out/a.md 变成 out/a.timestamped.md。
func timestampedOutputPath(outputPath string) string {
	ext := filepath.Ext(outputPath)
	return strings.TrimSuffix(outputPath, ext) + ".timestamped" + ext
}

// transcriptJSONOutputPath 把 out/a.md 变成 out/a.transcript.json。
func transcriptJSONOutputPath(outputPath string) string {
	ext := filepath.Ext(outputPath)
	return strings.TrimSuffix(outputPath, ext) + ".transcript.json"
}

func (p Processor) outputPath(inputPath string) string {
	base := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath)) + ".md"
	if p.OutDir == "" {
		return filepath.Join(filepath.Dir(inputPath), base)
	}
	return filepath.Join(p.OutDir, base)
}

func isSupportedMediaPath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp4", ".mov", ".m4v", ".3gp", ".mkv", ".webm",
		".wav", ".mp3", ".m4a", ".aac", ".ogg", ".flac":
		return true
	default:
		return false
	}
}
