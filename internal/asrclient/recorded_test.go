package asrclient

import (
	"context"
	"testing"
	"time"

	"mp4-md/internal/asrbatch"
	"mp4-md/internal/media"
	"mp4-md/internal/storage"
)

func TestRecordedClient_TranscribeUploadsAndRequestsDiarization(t *testing.T) {
	speakerOne := 1
	provider := &fakeRecordedProvider{
		result: asrbatch.Result{
			TaskID:     "task-123",
			TaskStatus: asrbatch.TaskStatusSucceeded,
			Transcripts: []asrbatch.FileTranscript{
				{
					Text: "第一句",
					Segments: []asrbatch.Segment{
						{Text: "第一句", BeginMS: 0, EndMS: 1200, SpeakerID: &speakerOne},
					},
				},
			},
		},
	}
	uploader := &fakeUploader{
		uploaded: storage.UploadedObject{
			ObjectKey: "asr-temp/audio.wav",
			ReadURL:   "https://oss.example.com/audio.wav?signature=abc",
		},
	}
	client := RecordedClient{
		APIKey:       "test-key",
		Uploader:     uploader,
		Provider:     provider,
		VocabularyID: "vocab-1",
		SpeakerCount: 2,
	}

	got, err := client.Transcribe(context.Background(), "input.mp4", media.Audio{
		FilePath: "/tmp/audio.wav",
	})
	if err != nil {
		t.Fatalf("Transcribe returned error: %v", err)
	}
	if uploader.uploadedPath != "/tmp/audio.wav" {
		t.Fatalf("uploaded path = %q", uploader.uploadedPath)
	}
	if uploader.deletedKey != "asr-temp/audio.wav" {
		t.Fatalf("deleted key = %q", uploader.deletedKey)
	}
	if len(provider.request.FileURLs) != 1 || provider.request.FileURLs[0] != uploader.uploaded.ReadURL {
		t.Fatalf("file urls = %#v", provider.request.FileURLs)
	}
	if provider.request.DiarizationEnabled == nil || *provider.request.DiarizationEnabled != true {
		t.Fatalf("diarization flag = %#v", provider.request.DiarizationEnabled)
	}
	if provider.request.SpeakerCount == nil || *provider.request.SpeakerCount != 2 {
		t.Fatalf("speaker count = %#v", provider.request.SpeakerCount)
	}
	if provider.request.VocabularyID != "vocab-1" {
		t.Fatalf("vocabulary id = %q", provider.request.VocabularyID)
	}
	if got.TaskID != "task-123" {
		t.Fatalf("task id = %q", got.TaskID)
	}
	if len(got.Segments) != 1 || got.Segments[0].SpeakerID == nil || *got.Segments[0].SpeakerID != 1 {
		t.Fatalf("segments = %#v", got.Segments)
	}
}

func TestTranscriptFromBatchResultDeduplicatesRepeatedSegments(t *testing.T) {
	speakerZero := 0
	segment := asrbatch.Segment{
		Text:      "重复句子",
		BeginMS:   0,
		EndMS:     1200,
		SpeakerID: &speakerZero,
	}

	got := transcriptFromBatchResult(asrbatch.Result{
		Transcripts: []asrbatch.FileTranscript{
			{
				Text:     "重复句子",
				Segments: []asrbatch.Segment{segment},
				Channels: []asrbatch.ChannelTranscript{
					{Text: "重复句子", Segments: []asrbatch.Segment{segment}},
				},
			},
		},
	})

	if len(got.Segments) != 1 {
		t.Fatalf("segments len = %d, want 1: %#v", len(got.Segments), got.Segments)
	}
}

type fakeUploader struct {
	uploaded      storage.UploadedObject
	uploads       []storage.UploadedObject
	uploadedPath  string
	uploadedPaths []string
	deletedKey    string
	deletedKeys   []string
}

func (f *fakeUploader) Upload(_ context.Context, filePath string) (storage.UploadedObject, error) {
	f.uploadedPath = filePath
	f.uploadedPaths = append(f.uploadedPaths, filePath)
	if len(f.uploads) > 0 {
		item := f.uploads[0]
		f.uploads = f.uploads[1:]
		return item, nil
	}
	return f.uploaded, nil
}

func (f *fakeUploader) Delete(_ context.Context, objectKey string) error {
	f.deletedKey = objectKey
	f.deletedKeys = append(f.deletedKeys, objectKey)
	return nil
}

type fakeRecordedProvider struct {
	request  asrbatch.Request
	requests []asrbatch.Request
	result   asrbatch.Result
	results  []asrbatch.Result
}

func (f *fakeRecordedProvider) Recognize(_ context.Context, request asrbatch.Request) (asrbatch.Result, error) {
	f.request = request
	f.requests = append(f.requests, request)
	if len(f.results) > 0 {
		result := f.results[0]
		f.results = f.results[1:]
		return result, nil
	}
	return f.result, nil
}

func TestRecordedClient_TranscribesMultipleChunksAndOffsetsSegments(t *testing.T) {
	speakerZero := 0
	provider := &fakeRecordedProvider{
		results: []asrbatch.Result{
			{
				TaskID: "task-1",
				Transcripts: []asrbatch.FileTranscript{
					{
						FileURL: "https://oss.example.com/part-1.m4a",
						Segments: []asrbatch.Segment{
							{Text: "第一段", BeginMS: 100, EndMS: 900, SpeakerID: &speakerZero},
						},
					},
				},
			},
			{
				TaskID: "task-2",
				Transcripts: []asrbatch.FileTranscript{
					{
						FileURL: "https://oss.example.com/part-2.m4a",
						Segments: []asrbatch.Segment{
							{Text: "第二段", BeginMS: 200, EndMS: 1000, SpeakerID: &speakerZero},
						},
					},
				},
			},
		},
	}
	uploader := &fakeUploader{
		uploads: []storage.UploadedObject{
			{ObjectKey: "part-1", ReadURL: "https://oss.example.com/part-1.m4a"},
			{ObjectKey: "part-2", ReadURL: "https://oss.example.com/part-2.m4a"},
		},
	}
	client := RecordedClient{
		APIKey:       "test-key",
		Uploader:     uploader,
		Provider:     provider,
		SpeakerCount: 2,
	}

	got, err := client.Transcribe(context.Background(), "input.mp4", media.Audio{
		Files: []media.AudioFile{
			{Path: "/tmp/part-1.m4a", Offset: 0},
			{Path: "/tmp/part-2.m4a", Offset: 2 * time.Hour},
		},
	})
	if err != nil {
		t.Fatalf("Transcribe returned error: %v", err)
	}
	if len(provider.requests) != 2 {
		t.Fatalf("request count = %d, want 2", len(provider.requests))
	}
	for index, request := range provider.requests {
		if len(request.FileURLs) != 1 {
			t.Fatalf("request %d file urls = %#v, want exactly one", index, request.FileURLs)
		}
	}
	if len(uploader.deletedKeys) != 2 {
		t.Fatalf("deleted keys = %#v", uploader.deletedKeys)
	}
	if len(got.Segments) != 2 {
		t.Fatalf("segments = %#v", got.Segments)
	}
	if got.Segments[1].BeginMS != int((2*time.Hour).Milliseconds())+200 {
		t.Fatalf("second segment begin = %d", got.Segments[1].BeginMS)
	}
	if got.TaskID != "task-1,task-2" {
		t.Fatalf("task id = %q", got.TaskID)
	}
}
