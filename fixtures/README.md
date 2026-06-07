# Fixtures

These generated samples are used for repeatable local tests.

Generate them with:

```bash
./scripts/create-fixtures.sh
```

Files:

- `audio-clip-15s.wav`: generated 15 second sine-wave audio, PCM 16-bit / mono / 16kHz.
- `video-sample.mp4`: generated silent test-pattern video for no-audio-track failure tests.
- `video-clip-15s-with-audio.mp4`: generated test-pattern video with a sine-wave audio track.
- `video-sample-with-audio.mp4`: duplicate positive video fixture, kept for compatibility with existing tests.

The binary media files are ignored by Git. Generate them locally before running tests that need ffmpeg fixtures.
