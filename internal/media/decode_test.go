package media

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestDecodeMP4_AACFixture(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available in PATH")
	}
	fixtures := generateMediaFixtures(t)
	audio, err := DecodeMP4(fixtures.withAudio)
	if err != nil {
		t.Fatalf("DecodeMP4 returned error: %v", err)
	}
	if audio.SampleRate != 16000 {
		t.Fatalf("sample rate = %d, want 16000", audio.SampleRate)
	}
	if audio.Channels != 1 {
		t.Fatalf("channels = %d, want 1", audio.Channels)
	}
	if len(audio.PCM) == 0 {
		t.Fatal("pcm is empty")
	}
	if audio.Duration < 14*time.Second || audio.Duration > 16*time.Second {
		t.Fatalf("duration = %s, want about 15s", audio.Duration)
	}
}

func TestExtractor_ExtractKeepsTemporaryM4AUntilCleanup(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available in PATH")
	}
	audio, err := Extractor{}.Extract(
		t.Context(),
		generateMediaFixtures(t).withAudio,
	)
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	if audio.FilePath == "" {
		t.Fatal("FilePath is empty")
	}
	if filepath.Ext(audio.FilePath) != ".m4a" {
		t.Fatalf("FilePath ext = %q, want .m4a", filepath.Ext(audio.FilePath))
	}
	if len(audio.Files) != 1 {
		t.Fatalf("files len = %d, want 1", len(audio.Files))
	}
	if _, err := os.Stat(audio.FilePath); err != nil {
		t.Fatalf("expected extracted m4a to exist before cleanup: %v", err)
	}
	if cleanup := audio.Cleanup; cleanup == nil {
		t.Fatal("Cleanup is nil")
	} else if err := cleanup(); err != nil {
		t.Fatalf("Cleanup returned error: %v", err)
	}
	if _, err := os.Stat(audio.FilePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected extracted m4a to be deleted after cleanup, err=%v", err)
	}
}

func TestExtractor_ExtractSplitsLongAudioByChunkDuration(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available in PATH")
	}
	audio, err := Extractor{
		MaxChunkDuration: 5 * time.Second,
	}.Extract(
		t.Context(),
		generateMediaFixtures(t).withAudio,
	)
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	defer audio.Cleanup()
	if len(audio.Files) < 2 {
		t.Fatalf("files len = %d, want at least 2", len(audio.Files))
	}
	for i, file := range audio.Files {
		if filepath.Ext(file.Path) != ".m4a" {
			t.Fatalf("file %d ext = %q, want .m4a", i, filepath.Ext(file.Path))
		}
		if _, err := os.Stat(file.Path); err != nil {
			t.Fatalf("expected chunk %d to exist: %v", i, err)
		}
		if i > 0 && file.Offset <= audio.Files[i-1].Offset {
			t.Fatalf("chunk offsets not increasing: %#v", audio.Files)
		}
	}
}

func TestDecodeMP4_NoAudioTrack(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available in PATH")
	}
	_, err := DecodeMP4(generateMediaFixtures(t).silentVideo)
	if !errors.Is(err, ErrNoAudioTrack) {
		t.Fatalf("error = %v, want %v", err, ErrNoAudioTrack)
	}
}

type mediaFixtures struct {
	withAudio   string
	silentVideo string
}

func generateMediaFixtures(t *testing.T) mediaFixtures {
	t.Helper()
	dir := t.TempDir()
	withAudio := filepath.Join(dir, "video-with-audio.mp4")
	silentVideo := filepath.Join(dir, "silent-video.mp4")

	runFFmpeg(t,
		"-f", "lavfi", "-i", "testsrc=size=320x240:rate=15:duration=15",
		"-f", "lavfi", "-i", "sine=frequency=660:duration=15:sample_rate=16000",
		"-shortest", "-pix_fmt", "yuv420p", "-c:v", "libx264", "-c:a", "aac", "-ac", "1", "-ar", "16000",
		withAudio,
	)
	runFFmpeg(t,
		"-f", "lavfi", "-i", "testsrc=size=320x240:rate=15:duration=15",
		"-pix_fmt", "yuv420p", "-an",
		silentVideo,
	)
	return mediaFixtures{withAudio: withAudio, silentVideo: silentVideo}
}

func runFFmpeg(t *testing.T, args ...string) {
	t.Helper()
	fullArgs := append([]string{"-hide_banner", "-loglevel", "error", "-y"}, args...)
	cmd := exec.Command("ffmpeg", fullArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ffmpeg failed: %v\n%s", err, output)
	}
}

func TestEncodeWAV(t *testing.T) {
	data, err := EncodeWAV(Audio{
		SampleRate: 16000,
		Channels:   1,
		PCM:        []byte{0, 0, 1, 0, 255, 127, 0, 128},
	})
	if err != nil {
		t.Fatalf("EncodeWAV returned error: %v", err)
	}
	if len(data) <= 44 {
		t.Fatalf("wav length = %d, want header + payload", len(data))
	}
	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		t.Fatalf("invalid wav header: %q %q", data[0:4], data[8:12])
	}
}
