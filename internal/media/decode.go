package media

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

var (
	ErrNoAudioTrack          = errors.New("media: no audio track found")
	ErrUnsupportedAudioCodec = errors.New("media: unsupported audio codec")
)

const targetSampleRate = 16000
const defaultMaxChunkDuration = 2 * time.Hour
const defaultCompressedAudioBitrate = "64k"

type AudioFile struct {
	Path   string
	Offset time.Duration
}

type Audio struct {
	SampleRate int
	Channels   int
	PCM        []byte
	Duration   time.Duration
	FilePath   string
	Files      []AudioFile
	Cleanup    func() error
}

type Extractor struct {
	FFmpegPath       string
	MaxChunkDuration time.Duration
	AudioBitrate     string
}

func (e Extractor) Extract(ctx context.Context, inputPath string) (Audio, error) {
	return extractCompressedWithFFmpeg(ctx, inputPath, e)
}

func DecodeMP4(path string) (Audio, error) {
	audio, err := decodeWithFFmpeg(context.Background(), path, "", false)
	if err != nil {
		return Audio{}, err
	}
	audio.FilePath = ""
	audio.Cleanup = nil
	return audio, nil
}

func decodeWithFFmpeg(ctx context.Context, inputPath, overridePath string, keepFile bool) (Audio, error) {
	ffmpegPath, err := resolveFFmpegPath(overridePath)
	if err != nil {
		return Audio{}, err
	}

	tempDir, err := os.MkdirTemp("", "mp4-md-*")
	if err != nil {
		return Audio{}, err
	}
	cleanup := func() error {
		return os.RemoveAll(tempDir)
	}
	if !keepFile {
		defer cleanup()
	}

	outputPath := filepath.Join(tempDir, "audio.wav")
	cmd := exec.CommandContext(ctx, ffmpegPath,
		"-y",
		"-hide_banner",
		"-loglevel", "error",
		"-i", inputPath,
		"-vn",
		"-ac", "1",
		"-ar", "16000",
		"-sample_fmt", "s16",
		"-f", "wav",
		outputPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.ToLower(string(output))
		switch {
		case strings.Contains(message, "does not contain any stream"),
			strings.Contains(message, "matches no streams"),
			strings.Contains(message, "stream specifier ':a'"):
			_ = cleanup()
			return Audio{}, ErrNoAudioTrack
		default:
			_ = cleanup()
			return Audio{}, fmt.Errorf("run ffmpeg: %w: %s", err, strings.TrimSpace(string(output)))
		}
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		_ = cleanup()
		return Audio{}, err
	}

	wav, err := parseWAV(data)
	if err != nil {
		_ = cleanup()
		return Audio{}, err
	}
	audio := Audio{
		SampleRate: wav.SampleRate,
		Channels:   1,
		PCM:        wav.PCMData,
		Duration:   pcmDuration(len(wav.PCMData), wav.SampleRate, 1),
		FilePath:   outputPath,
		Files:      []AudioFile{{Path: outputPath}},
	}
	if keepFile {
		audio.Cleanup = cleanup
	}
	return audio, nil
}

func extractCompressedWithFFmpeg(ctx context.Context, inputPath string, extractor Extractor) (Audio, error) {
	ffmpegPath, err := resolveFFmpegPath(extractor.FFmpegPath)
	if err != nil {
		return Audio{}, err
	}

	tempDir, err := os.MkdirTemp("", "mp4-md-*")
	if err != nil {
		return Audio{}, err
	}
	cleanup := func() error {
		return os.RemoveAll(tempDir)
	}

	maxChunkDuration := extractor.MaxChunkDuration
	if maxChunkDuration <= 0 {
		maxChunkDuration = defaultMaxChunkDuration
	}
	audioBitrate := strings.TrimSpace(extractor.AudioBitrate)
	if audioBitrate == "" {
		audioBitrate = defaultCompressedAudioBitrate
	}

	outputPattern := filepath.Join(tempDir, "audio-%03d.m4a")
	cmd := exec.CommandContext(ctx, ffmpegPath,
		"-y",
		"-hide_banner",
		"-loglevel", "error",
		"-i", inputPath,
		"-vn",
		"-ac", "1",
		"-ar", "16000",
		"-c:a", "aac",
		"-b:a", audioBitrate,
		"-f", "segment",
		"-segment_time", fmt.Sprintf("%.3f", maxChunkDuration.Seconds()),
		"-reset_timestamps", "1",
		"-segment_format", "mp4",
		outputPattern,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		_ = cleanup()
		message := strings.ToLower(string(output))
		switch {
		case strings.Contains(message, "does not contain any stream"),
			strings.Contains(message, "matches no streams"),
			strings.Contains(message, "stream specifier ':a'"):
			return Audio{}, ErrNoAudioTrack
		default:
			return Audio{}, fmt.Errorf("run ffmpeg: %w: %s", err, strings.TrimSpace(string(output)))
		}
	}

	matches, err := filepath.Glob(filepath.Join(tempDir, "audio-*.m4a"))
	if err != nil {
		_ = cleanup()
		return Audio{}, err
	}
	sort.Strings(matches)
	if len(matches) == 0 {
		_ = cleanup()
		return Audio{}, ErrNoAudioTrack
	}

	files := make([]AudioFile, 0, len(matches))
	for index, match := range matches {
		files = append(files, AudioFile{
			Path:   match,
			Offset: time.Duration(index) * maxChunkDuration,
		})
	}
	return Audio{
		SampleRate: targetSampleRate,
		Channels:   1,
		FilePath:   files[0].Path,
		Files:      files,
		Cleanup:    cleanup,
	}, nil
}

func EncodeWAV(audio Audio) ([]byte, error) {
	if audio.SampleRate <= 0 {
		return nil, errors.New("media: sample rate must be positive")
	}
	if audio.Channels <= 0 {
		return nil, errors.New("media: channels must be positive")
	}
	if len(audio.PCM)%2 != 0 {
		return nil, errors.New("media: pcm must be 16-bit aligned")
	}

	dataLen := len(audio.PCM)
	blockAlign := audio.Channels * 2
	byteRate := audio.SampleRate * blockAlign

	buf := make([]byte, 44+dataLen)
	copy(buf[0:4], []byte("RIFF"))
	binary.LittleEndian.PutUint32(buf[4:8], uint32(36+dataLen))
	copy(buf[8:12], []byte("WAVE"))
	copy(buf[12:16], []byte("fmt "))
	binary.LittleEndian.PutUint32(buf[16:20], 16)
	binary.LittleEndian.PutUint16(buf[20:22], 1)
	binary.LittleEndian.PutUint16(buf[22:24], uint16(audio.Channels))
	binary.LittleEndian.PutUint32(buf[24:28], uint32(audio.SampleRate))
	binary.LittleEndian.PutUint32(buf[28:32], uint32(byteRate))
	binary.LittleEndian.PutUint16(buf[32:34], uint16(blockAlign))
	binary.LittleEndian.PutUint16(buf[34:36], 16)
	copy(buf[36:40], []byte("data"))
	binary.LittleEndian.PutUint32(buf[40:44], uint32(dataLen))
	copy(buf[44:], audio.PCM)
	return buf, nil
}

func resolveFFmpegPath(override string) (string, error) {
	candidates := make([]string, 0, 4)
	if strings.TrimSpace(override) != "" {
		candidates = append(candidates, override)
	}
	if env := strings.TrimSpace(os.Getenv("MP4MD_FFMPEG")); env != "" {
		candidates = append(candidates, env)
	}
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		binaryName := "ffmpeg"
		if runtime.GOOS == "windows" {
			binaryName = "ffmpeg.exe"
		}
		candidates = append(candidates, filepath.Join(exeDir, binaryName))
	}

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	path, err := exec.LookPath("ffmpeg")
	if err == nil {
		return path, nil
	}
	return "", errors.New("media: ffmpeg not found; place ffmpeg next to mp4-md, set MP4MD_FFMPEG, or add it to PATH")
}

type wavData struct {
	SampleRate int
	PCMData    []byte
}

func parseWAV(data []byte) (wavData, error) {
	if len(data) < 44 {
		return wavData{}, fmt.Errorf("invalid wav: file too small")
	}
	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return wavData{}, fmt.Errorf("invalid wav: missing RIFF/WAVE header")
	}

	offset := 12
	sampleRate := 0
	audioFormat := uint16(0)
	bitsPerSample := uint16(0)
	channelCount := uint16(0)
	dataChunk := []byte(nil)

	for offset+8 <= len(data) {
		chunkID := string(data[offset : offset+4])
		chunkSize := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		offset += 8

		if offset+chunkSize > len(data) {
			return wavData{}, fmt.Errorf("invalid wav: chunk overflow for %s", chunkID)
		}

		chunk := data[offset : offset+chunkSize]
		switch chunkID {
		case "fmt ":
			if len(chunk) < 16 {
				return wavData{}, fmt.Errorf("invalid wav: fmt chunk too small")
			}
			audioFormat = binary.LittleEndian.Uint16(chunk[0:2])
			channelCount = binary.LittleEndian.Uint16(chunk[2:4])
			sampleRate = int(binary.LittleEndian.Uint32(chunk[4:8]))
			bitsPerSample = binary.LittleEndian.Uint16(chunk[14:16])
		case "data":
			dataChunk = append([]byte(nil), chunk...)
		}

		offset += chunkSize
		if chunkSize%2 == 1 && offset < len(data) {
			offset++
		}
	}

	if sampleRate <= 0 {
		return wavData{}, fmt.Errorf("invalid wav: missing sample rate")
	}
	if audioFormat != 1 {
		return wavData{}, fmt.Errorf("unsupported wav audio format: %d", audioFormat)
	}
	if channelCount != 1 {
		return wavData{}, fmt.Errorf("unsupported wav channels: %d", channelCount)
	}
	if bitsPerSample != 16 {
		return wavData{}, fmt.Errorf("unsupported wav bit depth: %d", bitsPerSample)
	}
	if len(dataChunk) == 0 {
		return wavData{}, fmt.Errorf("invalid wav: missing data chunk")
	}

	return wavData{
		SampleRate: sampleRate,
		PCMData:    dataChunk,
	}, nil
}

func pcmDuration(byteLen, sampleRate, channels int) time.Duration {
	if byteLen <= 0 || sampleRate <= 0 || channels <= 0 {
		return 0
	}
	samples := byteLen / 2 / channels
	return time.Duration(samples) * time.Second / time.Duration(sampleRate)
}
