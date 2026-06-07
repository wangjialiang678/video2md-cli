$ErrorActionPreference = "Stop"

$Repo = if ($env:VIDEO2MD_GITHUB_REPO) { $env:VIDEO2MD_GITHUB_REPO } else { "wangjialiang678/video2md-cli" }
$InstallDir = if ($env:VIDEO2MD_INSTALL_DIR) { $env:VIDEO2MD_INSTALL_DIR } else { Join-Path $env:USERPROFILE ".video2md-cli" }
$BinDir = Join-Path $InstallDir "bin"
$SkillDest = Join-Path (if ($env:CODEX_HOME) { $env:CODEX_HOME } else { Join-Path $env:USERPROFILE ".codex" }) "skills\video2md-cli"
$RootDir = Resolve-Path (Join-Path $PSScriptRoot "..")

New-Item -ItemType Directory -Force -Path $BinDir | Out-Null

$zipUrl = "https://github.com/$Repo/releases/latest/download/video2md-cli-windows-amd64.zip"
$tmp = Join-Path $env:TEMP ("video2md-install-" + [Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force -Path $tmp | Out-Null

try {
  $LocalBin = Join-Path $RootDir "dist\build\windows-amd64\mp4-md.exe"
  if (Test-Path $LocalBin) {
    Write-Host "Installing local binary $LocalBin"
    Copy-Item -Force $LocalBin (Join-Path $BinDir "mp4-md.exe")
  } else {
    Write-Host "Downloading $zipUrl"
    Invoke-WebRequest -Uri $zipUrl -OutFile (Join-Path $tmp "video2md.zip")
    Expand-Archive -Path (Join-Path $tmp "video2md.zip") -DestinationPath $tmp -Force
    Copy-Item -Force (Join-Path $tmp "mp4-md.exe") (Join-Path $BinDir "mp4-md.exe")
  }
} catch {
  if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw "Release download failed and Go is not installed. Install from GitHub Releases or install Go, then rerun."
  }
  Write-Host "Building local CLI with Go"
  Push-Location $RootDir
  try {
    & go build -o (Join-Path $BinDir "mp4-md.exe") ./cmd/mp4-md
  } finally {
    Pop-Location
  }
} finally {
  Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}

$SkillSource = Join-Path $RootDir "skills\video2md-cli"
if (Test-Path $SkillSource) {
  Remove-Item -Recurse -Force $SkillDest -ErrorAction SilentlyContinue
  New-Item -ItemType Directory -Force -Path (Split-Path $SkillDest) | Out-Null
  Copy-Item -Recurse -Force $SkillSource $SkillDest
}

Copy-Item -Force (Join-Path $RootDir "scripts\video2md.ps1") (Join-Path $BinDir "video2md.ps1")

$EnvFile = Join-Path $env:USERPROFILE ".video2md-cli.env"
if (-not (Test-Path $EnvFile)) {
  Copy-Item (Join-Path $RootDir ".env.example") $EnvFile
}

Write-Host "Installed mp4-md to $(Join-Path $BinDir "mp4-md.exe")"
Write-Host "Installed wrapper to $(Join-Path $BinDir "video2md.ps1")"
Write-Host "Installed Codex skill to $SkillDest"
Write-Host "Configure secrets in $EnvFile"
Write-Host "Restart Codex to pick up the skill."
