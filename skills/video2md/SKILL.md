---
name: video2md
description: 把本地视频/音频转成文字稿（Markdown），支持说话人分离和时间戳。当用户要求把视频转文字、音频转文字、视频转字幕、提取视频里的对话内容、导出转写稿、把录音整理成文稿、区分谁在说话、要带时间戳的转写，或提到 video2md、mp4-md 时使用。支持单个文件或整个文件夹批量处理。关键词：视频转文字, 视频转字幕, 音频转文字, 转写, 转文稿, 提取字幕, 说话人分离, 时间戳, 会议记录转文字, 采访转文字, video2md, mp4-md。
platform: universal
category: content
version: 1.4.0
author: michael
---

# video2md — 视频/音频转文字稿

把本地视频或音频转成 Markdown 文字稿，自动区分说话人。转码在本地跑（ffmpeg），
识别走阿里云百炼 Fun-ASR。**只需要一个 `DASHSCOPE_API_KEY`，不需要任何 OSS 配置。**

## 首次使用前确认

1. **ffmpeg 已安装**：`ffmpeg -version`，没有就 `brew install ffmpeg`
2. **凭证已配置**：`~/.video2md-cli.env` 里有 `DASHSCOPE_API_KEY`

   ```bash
   echo 'export DASHSCOPE_API_KEY=sk-你的key' > ~/.video2md-cli.env
   chmod 600 ~/.video2md-cli.env
   ```

   Key 从哪来：① 找 Michael 要（团队共用）；② 自己去 https://bailian.console.aliyun.com/ 申请。

> 平台：macOS(arm64/amd64) 与 Windows(amd64) 随包已带预编译二进制，**无需安装 Go**；
> Linux/其它架构见 `INSTALL.md`。

脚本会自己检查这两项，缺了会给出明确提示，**不要提前追问用户**。

## 默认工作流

1. 解析输入路径。可以是单个文件，也可以是整个文件夹（自动批量）。
2. 输出目录：用户指定了就用；没指定就用输入文件所在目录。
3. **不要编造说话人名字。** 用户没明确给名字就不要传 `--speaker`，
   让它输出「说话人1 / 说话人2」。
4. 跑 `scripts/video2md.sh`。
5. 把生成的 Markdown 路径报给用户。默认会出两个文件（正文 + 时间戳版），
   两个都要提到，并说明区别。

## 命令

```bash
# 基本用法（脚本在本 skill 目录下，用相对路径调用）
scripts/video2md.sh --out-dir ./out /path/to/video.mp4

# 整个文件夹
scripts/video2md.sh --out-dir ./out /path/to/videos/
```

**Windows**（agent 环境没有 bash 时）改用 PowerShell 入口，参数完全相同：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/video2md.ps1 --out-dir ./out C:\path\to\video.mp4
```

常用选项：

| 选项 | 作用 |
|---|---|
| `--plain` | 只要纯文本，不带「说话人N」前缀 |
| `--timestamps word` | 时间戳版里额外带逐词时间和置信度（默认 `sentence`，`none` 则不出时间戳文件） |
| `--speaker-count 3` | 已知有几个说话人时告诉它，提升分离准确度（默认 2） |
| `--speaker 1=张三 --speaker 2=李四` | **仅当用户明确给了名字时**才用 |
| `--vocab vocab-xxx` | 热词表 ID，提升专有名词准确率 |
| `--workers 4` | 批量时的并发数（默认 2） |
| `--skip-existing` | 跳过已经转过的文件 |
| `--emit-json` | 额外产出结构化 `<名字>.transcript.json`（段/词级时间戳、置信度、说话人），供程序按词级时间做剪辑映射；默认只出 md |

## 输出长什么样

**默认一次出两个文件。** 正文永远是干净的，时间戳单独放一份，互不干扰：

`meeting.md` —— 正文，带说话人、无时间戳，直接能读能贴：

```markdown
# 团队会议
来源：`meeting.mp4`

**说话人1**: 你好，我想问一下这个工具怎么用？

**说话人2**: 很简单，把视频拖进来就行。
```

`meeting.timestamped.md` —— 句级时间戳版，用来回溯原视频位置：

```markdown
**说话人1** `[00:00:00.160 → 00:00:04.480]`: 你好，我想问一下这个工具怎么用？

**说话人2** `[00:00:04.680 → 00:00:10.960]`: 很简单，把视频拖进来就行。
```

加 `--timestamps word`，时间戳版里每句后面再跟一张逐词表（含置信度，可用来定位识别不准的词）：

```markdown
| 起 | 止 | 词 | 置信度 |
|---|---|---|---|
| 00:00:00.160 | 00:00:00.600 | 你好， | 0.980 |
```

用户只要正文时加 `--timestamps none`；不要说话人前缀时加 `--plain`（两者可叠加）。

## 行为说明

- 转码（ffmpeg）在本地完成，只有压缩后的音频（16k 单声道 m4a）会上传识别。
- 音频存放在 DashScope 自带的临时空间，48 小时自动过期，无需清理。
- 超过 2 小时的文件自动切段识别，再合并回一个 Markdown。
- 说话人分离默认开启。

## 注意

- 不要打印或回显 API key。
- 用户没给说话人名字时，不要自己猜名字。
- 如果转写结果为空且报 `ASR_RESPONSE_HAVE_NO_WORDS`，说明音频里没有可识别的人声，
  先确认视频确实有声音（`ffprobe 文件名` 看有没有 Audio 流）。
