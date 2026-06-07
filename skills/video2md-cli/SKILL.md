---
name: video2md-cli
description: Use when the user asks to transcribe local audio/video files, convert video/audio to Markdown, run speaker diarization, apply user-provided speaker names, use hotword vocabulary, or process folders of media with the local video2md/mp4-md CLI. Keywords: 视频转文字, 音频转文字, 转 Markdown, 说话人分离, speaker diarization, hotword, 热词, mp4-md, video2md.
---

# video2md CLI

Use this skill to convert local audio/video files into speaker-separated Markdown with the local `mp4-md` CLI.

## Default Workflow

1. Resolve input path(s). Accept files or folders.
2. Ask only if output directory is missing and cannot be reasonably assumed.
3. Do not invent speaker names. If the user does not explicitly provide names, do not pass `--speaker`; the CLI will render `说话人1`, `说话人2`, etc.
4. Prefer output directory:
   - user-provided `--out-dir`
   - otherwise `/tmp/video2md-output-YYYYMMDD-HHMMSS`
5. Run `scripts/video2md.sh`.
6. Report generated Markdown path(s).

## Command Pattern

```bash
~/.codex/skills/video2md-cli/scripts/video2md.sh \
  --out-dir /tmp/video2md-output \
  /path/to/video-or-folder
```

Optional:

```bash
--vocab vocab-xxx
--speaker-count 2
--speaker 1=Alice
--speaker 2=Bob
--workers 2
--skip-existing
```

## Behavior

- The CLI uses `ffmpeg` locally.
- It reads credentials from `~/.video2md-cli.env` by default.
- It uploads compressed `m4a/aac` chunks to Aliyun OSS.
- It calls DashScope Fun-ASR recorded-file transcription.
- Speaker diarization is enabled by default.
- Files longer than 2 hours are chunked automatically.
- Temporary OSS objects are deleted automatically.

## Notes

- Do not print API keys or vault contents.
- Do not pass `--speaker` unless the user provided speaker names.
- If credentials are missing, ask the user to run the repository's `scripts/setup-secrets.sh` or edit `~/.video2md-cli.env`.
- If the CLI binary is missing, ask the user to run the repository's installer.
