#!/usr/bin/env pwsh
# video2md skill 的 Windows 入口:定位随包二进制 → 加载凭证 → 前置检查 → 执行。
# 与 video2md.sh 等价,供不带 Git Bash 的 Windows agent 直接调用。
$ErrorActionPreference = "Stop"

$SkillDir = Split-Path -Parent $PSScriptRoot
$EnvFile  = if ($env:VIDEO2MD_ENV_FILE) { $env:VIDEO2MD_ENV_FILE } else { Join-Path $env:USERPROFILE ".video2md-cli.env" }

function Write-Err([string]$msg) { [Console]::Error.WriteLine($msg) }

# 1) 选二进制:环境变量指定的 → 随包携带的 → 系统已装的
function Get-Mp4mdBinary {
  if ($env:VIDEO2MD_CLI_BIN -and (Test-Path $env:VIDEO2MD_CLI_BIN)) { return $env:VIDEO2MD_CLI_BIN }
  $bundled = Join-Path $SkillDir "bin\mp4-md-windows-amd64.exe"
  if (Test-Path $bundled) { return $bundled }
  $installed = Join-Path $env:USERPROFILE ".video2md-cli\bin\mp4-md.exe"
  if (Test-Path $installed) { return $installed }
  $onPath = Get-Command mp4-md -ErrorAction SilentlyContinue
  if ($onPath) { return $onPath.Source }
  return $null
}

$Bin = Get-Mp4mdBinary
if (-not $Bin) {
  Write-Err "找不到可用的 mp4-md 二进制(Windows)。"
  Write-Err "此 skill 包已内置 windows-amd64 二进制,正常无需装 Go;若包不完整可从源码构建:"
  Write-Err "  https://github.com/wangjialiang678/video2md-cli"
  exit 127
}

# 2) 前置依赖:ffmpeg(本地抽音频用,必须有)
if (-not $env:MP4MD_FFMPEG -and -not (Get-Command ffmpeg -ErrorAction SilentlyContinue)) {
  Write-Err "缺少 ffmpeg —— 本工具在本地抽取音频需要它。"
  Write-Err "  Windows: winget install Gyan.FFmpeg"
  exit 127
}

# 3) 凭证:加载 env 文件(兼容 shell 的 `export KEY=value` 写法),只需要一个 DASHSCOPE_API_KEY
if (Test-Path $EnvFile) {
  Get-Content $EnvFile | ForEach-Object {
    $line = $_.Trim()
    if ($line -eq "" -or $line.StartsWith("#")) { return }
    if ($line -match '^(export\s+)?([^=]+)=(.*)$') {
      $name  = $Matches[2].Trim()
      $value = $Matches[3].Trim()
      if (($value.StartsWith("'") -and $value.EndsWith("'")) -or ($value.StartsWith('"') -and $value.EndsWith('"'))) {
        $value = $value.Substring(1, $value.Length - 2)
      }
      [Environment]::SetEnvironmentVariable($name, $value, "Process")
    }
  }
}

if (-not $env:DASHSCOPE_API_KEY) {
  Write-Err "缺少 DASHSCOPE_API_KEY。写入 $EnvFile 即可(一行):"
  Write-Err "  export DASHSCOPE_API_KEY=sk-你的key"
  Write-Err "Key 来源:1) 找 Michael 要(团队共用);2) https://bailian.console.aliyun.com/ 右上角 API-KEY"
  exit 1
}

& $Bin @args
exit $LASTEXITCODE
