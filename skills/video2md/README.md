# video2md · 视频/音频转文字稿

把视频或录音丢给 AI，让它转成能直接读、直接用的文字稿。**自动区分谁在说话**，可选带时间戳。

---

## 什么时候用它

- 开完会有录屏／录音，想要一份文字纪要
- 采访、访谈、播客，需要逐字稿
- 课程／讲座视频，想把内容变成可搜索、可引用的文本
- 手上一堆视频要批量转，不想一个个上传到某个网站

**不适合**：实时字幕（这是离线批量转写，不是直播场景）。

## 在哪里用

装完之后，**直接跟你的 AI 说人话就行**，不用记命令：

> 「把 ~/Downloads/周会录屏.mp4 转成文字」
> 「这个文件夹里的视频全转一遍」
> 「转成文字，我只要纯文本不要说话人标签」

支持的 AI 环境：**Claude Code / Codex / WorkBuddy** 都行。

也可以自己在终端敲命令（见最下面）。

---

## 怎么装（三步，5 分钟）

**1. 装工具**（一行命令，二进制已经打包好，不用装编译器）

```bash
# Claude Code
curl -fsSL "$HUB_URL/install/video2md?token=$HUB_TOKEN" | bash

# Codex
curl -fsSL "$HUB_URL/install/video2md?token=$HUB_TOKEN" | bash -s -- --dir ~/.codex/skills

# WorkBuddy
curl -fsSL "$HUB_URL/install/video2md?token=$HUB_TOKEN" | bash -s -- --dir ~/.agents/skills
```

（`$HUB_URL` 和团队口令找负责人要，或直接复制本页顶部的「安装命令」按钮。）

**2. 装 ffmpeg**（本地抽音频要用，只装一次）

```bash
brew install ffmpeg
```

**3. 填 API key**（找 Michael 要，或自己去[百炼控制台](https://bailian.console.aliyun.com/)右上角免费申请）

```bash
echo 'export DASHSCOPE_API_KEY=sk-你的key' > ~/.video2md-cli.env
chmod 600 ~/.video2md-cli.env
```

装完重启一下 AI 工具，让它认出这个 skill。

---

## 你会得到什么

**默认一次出两个文件。** 正文保持干净，时间戳单独放一份，互不干扰。

**`会议录屏.md`** —— 正文，能直接读、直接贴进纪要：

```
# 会议录屏
来源：`会议录屏.mp4`

**说话人1**: 你好，我想问一下这个视频转文字的工具怎么用？

**说话人2**: 很简单，你只要把视频文件拖进来，它就会自动帮你转成文字稿。
```

**`会议录屏.timestamped.md`** —— 同样内容，但每句带时间戳，用来回原视频找位置：

```
**说话人1** `[00:00:00.160 → 00:00:04.480]`: 你好，我想问一下这个视频转文字的工具怎么用？

**说话人2** `[00:00:04.680 → 00:00:10.960]`: 很简单，你只要把视频文件拖进来，它就会自动帮你转成文字稿。
```

---

## 能调什么（跟 AI 说这些话就行）

| 你想要 | 跟 AI 说 | 底层参数 |
|---|---|---|
| 只要纯文本，不要「说话人1/2」 | 「不要说话人标签」 | `--plain` |
| 不要时间戳那个文件 | 「不用出时间戳版」 | `--timestamps none` |
| 逐词时间戳 + 置信度 | 「要词级时间戳」 | `--timestamps word` |
| 已知说话人是谁 | 「说话人1是张三，2是李四」 | `--speaker 1=张三 --speaker 2=李四` |
| 有三个人在说话 | 「有三个人」 | `--speaker-count 3` |
| 专有名词老是转错 | 「用热词表 vocab-xxx」 | `--vocab vocab-xxx` |
| 一批文件，跳过转过的 | 「跳过已经转过的」 | `--skip-existing` |

**词级时间戳有什么用**：每个词都带置信度，能一眼看出哪个词机器没听准。比如置信度 0.038 的词，八成是错的，值得回去听一遍。

---

## 支持的格式

- **视频**：mp4、mov、m4v、3gp、mkv、webm
- **音频**：wav、mp3、m4a、aac、ogg、flac

单个文件、整个文件夹都行。超过 2 小时的自动切段处理再合并，时间戳会对齐。

---

## 它是怎么工作的（不关心可以跳过）

1. **ffmpeg 在你本机**把音轨抽出来，压成小体积音频（16k 单声道）
2. 压缩后的音频传到阿里云百炼做识别（**只传音频，不传视频**）
3. 用 Fun-ASR 模型转写，开启说话人分离
4. 生成 Markdown

临时音频 48 小时后自动过期，不用清理。**你只需要一把 API key，不需要配任何云存储。**

---

## 直接用命令行

```bash
~/.video2md-cli/bin/video2md --out-dir ./out ./meeting.mp4      # 单个文件
~/.video2md-cli/bin/video2md --out-dir ./out ./videos/          # 整个文件夹
~/.video2md-cli/bin/video2md --out-dir ./out --plain ./talk.mp3 # 只要纯文本
```

---

## 出问题了？

| 症状 | 原因 / 怎么办 |
|---|---|
| 提示缺 ffmpeg | `brew install ffmpeg` |
| 提示缺 DASHSCOPE_API_KEY | key 没填对，检查 `~/.video2md-cli.env` |
| 转出来是空的，报 `ASR_RESPONSE_HAVE_NO_WORDS` | 视频里没有可识别的人声。用 `ffprobe 文件名` 确认它真的有音轨 |
| 人名/专有名词老转错 | 用热词表（`--vocab`），或事后手工改 |
| AI 认不出这个 skill | 装完要重启 AI 工具 |

源码：https://github.com/wangjialiang678/video2md-cli （需要能访问 GitHub，用不上也不影响）
