package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"mp4-md/internal/app"
	"mp4-md/internal/asrclient"
	"mp4-md/internal/media"
	"mp4-md/internal/storage"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "-h", "--help", "help":
			return usageError()
		}
	}
	return runTranscribe(args)
}

func runTranscribe(args []string) error {
	fs := flag.NewFlagSet("mp4-md", flag.ContinueOnError)
	var outDir string
	var workers int
	var language string
	var apiKey string
	var vocabularyID string
	var speakerCount int
	var skipExisting bool
	var ossAccessKeyID string
	var ossAccessKeySecret string
	var ossBucket string
	var ossEndpoint string
	var ossPrefix string
	var speakerNames speakerMapFlag

	fs.StringVar(&outDir, "out-dir", "", "directory for generated markdown files")
	fs.IntVar(&workers, "workers", 2, "number of files to process concurrently")
	fs.StringVar(&language, "lang", "zh", "transcription language hint")
	fs.StringVar(&apiKey, "api-key", "", "DashScope API key, defaults to DASHSCOPE_API_KEY")
	fs.StringVar(&vocabularyID, "vocab", "", "DashScope hotword vocabulary_id")
	fs.StringVar(&vocabularyID, "vocabulary-id", "", "DashScope hotword vocabulary_id")
	fs.IntVar(&speakerCount, "speaker-count", 2, "expected speaker count for diarization")
	fs.Var(&speakerNames, "speaker", "optional speaker name mapping, e.g. --speaker 1=Name --speaker 2=Name")
	fs.BoolVar(&skipExisting, "skip-existing", false, "skip files whose markdown output already exists")
	fs.StringVar(&ossAccessKeyID, "oss-access-key-id", "", "OSS access key id, defaults to OSS_ACCESS_KEY_ID or ALICLOUD_ACCESS_KEY_ID")
	fs.StringVar(&ossAccessKeySecret, "oss-access-key-secret", "", "OSS access key secret, defaults to OSS_ACCESS_KEY_SECRET or ALICLOUD_ACCESS_KEY_SECRET")
	fs.StringVar(&ossBucket, "oss-bucket", "", "OSS bucket, defaults to OSS_BUCKET")
	fs.StringVar(&ossEndpoint, "oss-endpoint", "", "OSS endpoint, defaults to OSS_ENDPOINT")
	fs.StringVar(&ossPrefix, "oss-prefix", "", "OSS object key prefix, defaults to OSS_OBJECT_KEY_PREFIX or asr-temp/video2md/")
	fs.SetOutput(os.Stderr)

	if err := fs.Parse(args); err != nil {
		return err
	}
	inputs := fs.Args()
	if len(inputs) == 0 {
		return usageError()
	}

	if apiKey == "" {
		apiKey = os.Getenv("DASHSCOPE_API_KEY")
	}
	if apiKey == "" {
		return fmt.Errorf("missing api key: use --api-key or DASHSCOPE_API_KEY")
	}

	ossConfig := storage.OSSConfigFromEnv()
	if ossAccessKeyID != "" {
		ossConfig.AccessKeyID = ossAccessKeyID
	}
	if ossAccessKeySecret != "" {
		ossConfig.AccessKeySecret = ossAccessKeySecret
	}
	if ossBucket != "" {
		ossConfig.Bucket = ossBucket
	}
	if ossEndpoint != "" {
		ossConfig.Endpoint = ossEndpoint
	}
	if ossPrefix != "" {
		ossConfig.ObjectKeyPrefix = ossPrefix
	}
	uploader, err := storage.NewAliyunOSSUploader(ossConfig)
	if err != nil {
		return err
	}

	processor := app.Processor{
		Extractor: media.Extractor{},
		Transcriber: asrclient.RecordedClient{
			APIKey:        apiKey,
			Uploader:      uploader,
			VocabularyID:  vocabularyID,
			LanguageHints: []string{language},
			SpeakerCount:  speakerCount,
		},
		OutDir:       outDir,
		Workers:      workers,
		SkipExisting: skipExisting,
		SpeakerNames: speakerNames.Map(),
	}

	results, err := processor.Process(context.Background(), inputs)
	for _, item := range results {
		fmt.Printf("%s -> %s\n", item.InputPath, item.OutputPath)
	}
	return err
}

func usageError() error {
	return fmt.Errorf("usage: mp4-md [options] <file-or-dir> [more paths]\ntry: mp4-md --out-dir ./out ./videos")
}

type speakerMapFlag map[int]string

func (s *speakerMapFlag) Set(value string) error {
	parts := strings.SplitN(value, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("speaker mapping must be N=Name")
	}
	id, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || id <= 0 {
		return fmt.Errorf("speaker id must be a positive integer")
	}
	name := strings.TrimSpace(parts[1])
	if name == "" {
		return fmt.Errorf("speaker name is required")
	}
	if *s == nil {
		*s = make(map[int]string)
	}
	(*s)[id] = name
	return nil
}

func (s *speakerMapFlag) String() string {
	if s == nil || len(*s) == 0 {
		return ""
	}
	parts := make([]string, 0, len(*s))
	for id, name := range *s {
		parts = append(parts, fmt.Sprintf("%d=%s", id, name))
	}
	return strings.Join(parts, ",")
}

func (s speakerMapFlag) Map() map[int]string {
	if len(s) == 0 {
		return nil
	}
	out := make(map[int]string, len(s))
	for id, name := range s {
		out[id] = name
	}
	return out
}
