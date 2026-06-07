# video2md-cli

`video2md-cli` turns local audio or video files into speaker-separated Markdown.

It is designed for two use cases:

- People can run the `mp4-md` CLI directly.
- AI agents can install the bundled Codex skill and call the CLI with the right defaults.

Default output uses Chinese speaker labels:

```markdown
**说话人1**: ...

**说话人2**: ...
```

If you know the speaker names, pass explicit mappings such as `--speaker 1=Alice --speaker 2=Bob`.

## What It Does

1. Uses `ffmpeg` to extract or normalize audio to compressed `16k / mono / aac m4a`.
2. Uploads the temporary audio to Aliyun OSS.
3. Calls DashScope Fun-ASR recorded-file transcription.
4. Enables speaker diarization.
5. Writes speaker-separated Markdown.
6. Deletes temporary OSS objects after transcription.

Files longer than 2 hours are split into 2-hour chunks, then merged back into one Markdown file with adjusted timestamps.

## Install With An AI Agent

If you do not want to install manually, send this repository to your AI agent and ask it to handle setup:

```text
Please install https://github.com/wangjialiang678/video2md-cli on this machine.

After installation, configure the private credentials in the local env file:

~/.video2md-cli.env

Use the DashScope and OSS values that I provide through a private channel. Do not commit credentials to GitHub, write them into README files, paste them into public chats, or include them in screenshots.

When transcribing files, use:

~/.video2md-cli/bin/video2md --out-dir ./out --speaker-count 2 /path/to/video.mp4

Do not pass --speaker by default. Let the output use "说话人1", "说话人2", etc. Only add --speaker mappings when the real speaker names are known.
```

## Install On Mac

Clone the repository:

```bash
git clone https://github.com/wangjialiang678/video2md-cli.git
cd video2md-cli
```

Install the CLI and Codex skill:

```bash
./scripts/install-mac.sh
```

Configure private credentials on this machine:

```bash
./scripts/setup-secrets.sh
```

The installer writes:

- CLI binary: `~/.video2md-cli/bin/mp4-md`
- env-loading wrapper: `~/.video2md-cli/bin/video2md`
- optional Codex skill: `~/.codex/skills/video2md-cli`
- private env file: `~/.video2md-cli.env`

The env file is created with `0600` permissions and is never committed to Git.

## Install On Windows

From PowerShell:

```powershell
git clone https://github.com/wangjialiang678/video2md-cli.git
cd video2md-cli
powershell -ExecutionPolicy Bypass -File .\scripts\install-windows.ps1
```

Configure private credentials on this machine:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\setup-secrets.ps1
```

You can also edit the generated private env file manually with `notepad $env:USERPROFILE\.video2md-cli.env`.

The Windows installer writes:

- CLI binary: `%USERPROFILE%\.video2md-cli\bin\mp4-md.exe`
- env-loading wrapper: `%USERPROFILE%\.video2md-cli\bin\video2md.ps1`
- optional Codex skill: `%USERPROFILE%\.codex\skills\video2md-cli`
- private env file: `%USERPROFILE%\.video2md-cli.env`

Windows still requires `ffmpeg.exe` in `PATH` or `MP4MD_FFMPEG` pointing to `ffmpeg.exe`.

## API Keys And OSS Credentials

Do not put secrets in this repository.

Each user needs a local env file at `~/.video2md-cli.env`:

```bash
export DASHSCOPE_API_KEY=your_dashscope_api_key
export OSS_ACCESS_KEY_ID=your_aliyun_oss_access_key_id
export OSS_ACCESS_KEY_SECRET=your_aliyun_oss_access_key_secret
export OSS_BUCKET=your_oss_bucket
export OSS_ENDPOINT=oss-cn-shanghai.aliyuncs.com
export OSS_OBJECT_KEY_PREFIX=asr-temp/video2md/
export OSS_READ_URL_TTL=2h
```

Recommended way to share credentials with a colleague:

1. Create a separate DashScope API key and Aliyun RAM user for that colleague.
2. Give the RAM user only the OSS bucket permissions needed for upload, signed read URL, and delete under `asr-temp/video2md/`.
3. Send the env values through a password manager, encrypted note, or another private channel.
4. Ask the colleague to run `./scripts/setup-secrets.sh` on Mac, or edit `%USERPROFILE%\.video2md-cli.env` on Windows.

Avoid sending credentials in GitHub, Slack public channels, email threads, or screenshots.

## CLI Usage

Put flags before input paths:

```bash
~/.video2md-cli/bin/video2md --out-dir ./out --speaker-count 2 ./meeting.mp4
~/.video2md-cli/bin/video2md --out-dir ./out ./meeting.wav
~/.video2md-cli/bin/video2md --out-dir ./out --speaker 1=Alice --speaker 2=Bob ./meeting.mp4
```

On Windows PowerShell:

```powershell
& "$env:USERPROFILE\.video2md-cli\bin\video2md.ps1" --out-dir .\out --speaker-count 2 .\meeting.mp4
```

Useful flags:

- `--out-dir DIR`: output directory for Markdown files
- `--workers N`: concurrent file count
- `--skip-existing`: skip files whose Markdown already exists
- `--speaker N=Name`: optional speaker-name mapping. If omitted, output uses `说话人1`, `说话人2`, etc. `1=Name` maps DashScope `speaker_id=0`.
- `--speaker-count N`: expected speaker count, default `2`
- `--vocab ID` / `--vocabulary-id ID`: DashScope hotword vocabulary ID
- `--oss-bucket`, `--oss-endpoint`, `--oss-prefix`: override OSS env config

Supported inputs:

- Video: `.mp4`, `.mov`, `.m4v`, `.3gp`, `.mkv`, `.webm`
- Audio: `.wav`, `.mp3`, `.m4a`, `.aac`, `.ogg`, `.flac`

## Use With Codex

After running `./scripts/install-mac.sh`, restart Codex.

Then ask the agent something like:

> 转写 `~/Downloads/demo.mp4`，输出 Markdown。

The installed skill tells the agent to use:

```bash
~/.codex/skills/video2md-cli/scripts/video2md.sh \
  --out-dir /tmp/video2md-output \
  /path/to/video-or-folder
```

You can also install only the skill from GitHub with Codex's built-in skill installer:

```bash
python ~/.codex/skills/.system/skill-installer/scripts/install-skill-from-github.py \
  --repo wangjialiang678/video2md-cli \
  --path skills/video2md-cli
```

Restart Codex after installing a skill.

## Development

```bash
go test ./...
go build -o dist/local/mp4-md-darwin-arm64 ./cmd/mp4-md
GOOS=windows GOARCH=amd64 go build -o dist/build/windows-amd64/mp4-md.exe ./cmd/mp4-md
```

Generate local media fixtures when needed:

```bash
./scripts/create-fixtures.sh
```

Build release archives:

```bash
./scripts/build-release.sh
```

## Notes

- The CLI requires `ffmpeg`.
- The normal path deletes OSS temporary objects after transcription.
- DashScope speaker diarization returns speaker ids, not real names or gender.
