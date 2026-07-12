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
}

type Output struct {
	InputPath  string
	OutputPath string
	// TimestampedPath 是带时间戳的伴生文件，未启用时为空。
	TimestampedPath string
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
	if p.SkipExisting {
		if _, statErr := os.Stat(outputPath); statErr == nil {
			return Output{InputPath: inputPath, OutputPath: outputPath}, nil
		}
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
	timestampedPath := ""
	if p.Timestamps != markdown.TimestampsNone && p.Timestamps != "" {
		timestampedPath = timestampedOutputPath(outputPath)
		if err := os.WriteFile(timestampedPath, []byte(markdown.Render(inputPath, transcript, markdown.Options{
			SpeakerNames: p.SpeakerNames,
			PlainText:    p.PlainText,
			Timestamps:   p.Timestamps,
		})), 0o644); err != nil {
			return Output{}, err
		}
	}
	return Output{InputPath: inputPath, OutputPath: outputPath, TimestampedPath: timestampedPath}, nil
}

// timestampedOutputPath 把 out/a.md 变成 out/a.timestamped.md。
func timestampedOutputPath(outputPath string) string {
	ext := filepath.Ext(outputPath)
	return strings.TrimSuffix(outputPath, ext) + ".timestamped" + ext
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
