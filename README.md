# video2md-cli

把本地视频/音频转成带说话人区分的 Markdown 文字稿。转码在本地跑，识别走阿里云百炼 Fun-ASR。

**只需要一个 `DASHSCOPE_API_KEY`。不需要 OSS，不需要自建 bucket，不需要阿里云 AccessKey。**

**默认一次出两个文件**，正文保持干净，时间戳单独放一份：

`meeting.md` —— 正文，带说话人、无时间戳：

```markdown
# 团队会议
来源：`meeting.mp4`

**说话人1**: 你好，我想问一下这个工具怎么用？

**说话人2**: 很简单，把视频拖进来就行。
```

`meeting.timestamped.md` —— 句级时间戳，用来回溯原视频位置：

```markdown
**说话人1** `[00:00:00.160 → 00:00:04.480]`: 你好，我想问一下这个工具怎么用？
```

`--timestamps word` 则在时间戳版里为每句附一张逐词表（含置信度，可定位识别不准的词）；
`--timestamps none` 只出正文。`--plain` 去掉说话人前缀。知道真名时用 `--speaker 1=张三 --speaker 2=李四`。

---

## 快速开始

装好 CLI 和 skill（见下面「从源码安装」），然后配好这两样：

```bash
brew install ffmpeg                                       # 本地转码要用（Windows: winget install Gyan.FFmpeg）
echo 'export DASHSCOPE_API_KEY=sk-你的key' > ~/.video2md-cli.env
chmod 600 ~/.video2md-cli.env
```

`DASHSCOPE_API_KEY` 去 [百炼控制台](https://bailian.console.aliyun.com/) 右上角
API-KEY 申请，这是唯一需要的凭证。

装好后直接跟 AI 说「把这个视频转成文字」即可，不用记命令。

> 超脑团队成员：内部 SkillHub 上已有打包好的 `video2md` skill（含 macOS/Windows 预编译二进制，
> **免 clone、免构建、无需装 Go**），安装命令找 Michael 要。Windows 上 agent 走 PowerShell 入口
> `scripts/video2md.ps1`，参数与 `.sh` 相同；平台矩阵与排障见包内 `INSTALL.md`。

---

## 工作原理

1. `ffmpeg` 在**本地**把音轨抽出来，压成 16k 单声道 m4a。
2. 压缩后的音频上传到 DashScope 自带的临时文件空间，换取一个 `oss://` 临时地址。
3. 调用 DashScope Fun-ASR 录音文件识别，开启说话人分离。
4. 渲染成 Markdown。

临时音频 48 小时后由 DashScope 侧自动过期，无需清理。超过 2 小时的文件自动切段识别再合并。

> **为什么不需要 OSS？** Fun-ASR 的录音文件识别接口只接受公网可访问的 URL，早期版本因此
> 要求用户自建 OSS 来临时托管音频。实际上 DashScope 提供了自己的临时文件空间
> （[官方文档](https://help.aliyun.com/zh/model-studio/get-temporary-file-url)），
> 配合请求头 `X-DashScope-OssResourceResolve: enable` 就能直接用，凭证从 5 个降到 1 个。
> 自建 OSS 仍然支持（见下），但已不是必需。

## 从源码安装（能访问 GitHub 的话）

```bash
git clone https://github.com/wangjialiang678/video2md-cli.git
cd video2md-cli
./scripts/install-mac.sh          # Windows: powershell -ExecutionPolicy Bypass -File .\scripts\install-windows.ps1
```

安装器会写入：

- CLI 二进制：`~/.video2md-cli/bin/mp4-md`
- 读 env 的包装脚本：`~/.video2md-cli/bin/video2md`
- skill：装到所有已存在的 AI 环境（`~/.claude/skills`、`~/.codex/skills`、`~/.agents/skills`）
- 私有 env 文件：`~/.video2md-cli.env`（权限 0600，永不入库）

## 命令行用法

选项要放在输入路径前面：

```bash
~/.video2md-cli/bin/video2md --out-dir ./out ./meeting.mp4       # 单个文件
~/.video2md-cli/bin/video2md --out-dir ./out ./videos/           # 整个文件夹
~/.video2md-cli/bin/video2md --out-dir ./out --plain ./talk.mp3  # 只要纯文本
```

| 选项 | 作用 |
|---|---|
| `--out-dir DIR` | Markdown 输出目录 |
| `--timestamps none\|sentence\|word` | 时间戳版文件的粒度，默认 `sentence`；`word` 附逐词时间与置信度；`none` 不产出该文件 |
| `--plain` | 纯文本输出，去掉「说话人N」前缀 |
| `--speaker-count N` | 预期说话人数，默认 2 |
| `--speaker N=Name` | 指定说话人名字。`1=Name` 对应 DashScope 的 `speaker_id=0` |
| `--vocab ID` | 热词表 ID，提升专有名词准确率 |
| `--workers N` | 批量处理并发数，默认 2 |
| `--skip-existing` | 跳过已有产出的文件 |
| `--emit-json` | 额外产出结构化 `<名字>.transcript.json`（段/词级时间戳、置信度、说话人），供程序消费 |

支持的输入：

- 视频 `.mp4` `.mov` `.m4v` `.3gp` `.mkv` `.webm`
- 音频 `.wav` `.mp3` `.m4a` `.aac` `.ogg` `.flac`

## 凭证

本仓库不含任何密钥。每台机器在 `~/.video2md-cli.env` 里配置：

```bash
export DASHSCOPE_API_KEY=sk-xxx     # 唯一必填
```

**可选**：大批量/高并发场景想改用自建 OSS 托管音频，把下面四项填全即可自动切换回 OSS 通道
（缺一项则仍走 DashScope 临时空间）：

```bash
export OSS_ACCESS_KEY_ID=xxx
export OSS_ACCESS_KEY_SECRET=xxx
export OSS_BUCKET=xxx
export OSS_ENDPOINT=oss-cn-shanghai.aliyuncs.com
```

分发给同事时，只发 `DASHSCOPE_API_KEY` 就够了——它可以在百炼后台随时禁用重建，
不像阿里云 AccessKey 一旦泄露牵连整个账号。不要把密钥发到 GitHub、公开频道或截图里。

## 发布 skill 到 SkillHub

改完代码后一条命令重新发布（构建三平台二进制 → 打包 → 上传）：

```bash
./scripts/pack-skill.sh                 # 需要 HUB_URL / HUB_WRITE_TOKEN
./scripts/pack-skill.sh --no-upload     # 只打包不上传
```

记得先把 `skills/video2md/SKILL.md` 里的 `version:` 递增。

## 开发

```bash
go test ./...
go build -o dist/local/mp4-md-darwin-arm64 ./cmd/mp4-md
```

需要本地测试素材时：

```bash
./scripts/create-fixtures.sh
```

## License

MIT
