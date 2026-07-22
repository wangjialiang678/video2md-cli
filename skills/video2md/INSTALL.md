# 安装与依赖说明

一句话:**在 macOS 和 Windows 上,你不需要安装 Go。** 二进制已随本 skill 包预编译好,
装完只要有 `ffmpeg` 和一个 `DASHSCOPE_API_KEY` 就能用。

## 平台矩阵

| 平台 | 随包二进制 | 需要装 Go 吗 | 入口脚本 |
|---|---|---|---|
| macOS Apple Silicon (arm64) | ✅ `mp4-md-darwin-arm64` | 否 | `scripts/video2md.sh` |
| macOS Intel (amd64) | ✅ `mp4-md-darwin-amd64` | 否 | `scripts/video2md.sh` |
| Windows (amd64) | ✅ `mp4-md-windows-amd64.exe` | 否 | `scripts/video2md.ps1` |
| Linux / 其它架构 | ❌ 未随包 | **是**,需从源码 `go build` | 自行构建后放入 `bin/` |

> Linux 用户:本包不含 Linux 二进制,也不含源码。请到
> https://github.com/wangjialiang678/video2md-cli clone 后 `go build`,
> 或找负责人要一个 `linux/amd64` 版本。

## 依赖清单(macOS / Windows)

| 依赖 | 必需? | 怎么装 | 校验 |
|---|---|---|---|
| **二进制** `mp4-md` | 必需 | 已随包,无需操作 | 自动挑选 |
| **ffmpeg** | 必需(本地抽音频) | macOS `brew install ffmpeg`;Windows `winget install Gyan.FFmpeg` | `ffmpeg -version` |
| **DASHSCOPE_API_KEY** | 必需(云端识别) | 写入 `~/.video2md-cli.env` | 见下 |
| 网络 | 必需 | 能访问 `dashscope.aliyuncs.com` | — |
| OSS_* 四件套 | 可选 | 仅大批量想自建 OSS 时 | — |
| **Go** | **不需要**(Mac/Win) | — | — |

## 三步装好

```bash
# 1) 装 skill(从 SkillHub,含预编译二进制,免 clone 免构建)
curl -fsSL "$HUB_URL/install/video2md?token=$HUB_TOKEN" | bash
#   Codex 装到 ~/.agents/skills:上面命令加  -s -- --dir ~/.agents/skills

# 2) 装 ffmpeg
brew install ffmpeg            # Windows: winget install Gyan.FFmpeg

# 3) 填 API key
echo 'export DASHSCOPE_API_KEY=sk-你的key' > ~/.video2md-cli.env
chmod 600 ~/.video2md-cli.env
```

## 常见报错

| 症状 | 原因 / 处理 |
|---|---|
| `找不到可用的 mp4-md 二进制` | 多半是在 **Linux/异架构**上——那才需要 Go 从源码构建;Mac/Win 出现此提示说明包不完整,重装一次 |
| macOS 首次运行被拦(“无法验证开发者”/进程被杀) | 二进制未经 Apple 公证。`scripts/video2md.sh` 已会自动剥离隔离属性;若仍被拦,手动执行 `xattr -d com.apple.quarantine bin/mp4-md-darwin-*` |
| Windows 提示脚本无法运行 | 用 `powershell -ExecutionPolicy Bypass -File scripts/video2md.ps1 ...` 调用;若 SmartScreen 拦 `.exe`,在文件属性里勾“解除锁定” |
| 缺 ffmpeg | 按上表安装 |
| 缺 `DASHSCOPE_API_KEY` | 检查 `~/.video2md-cli.env` |
| 转写为空,报 `ASR_RESPONSE_HAVE_NO_WORDS` | 视频没有可识别人声,用 `ffprobe 文件名` 确认有音轨 |
