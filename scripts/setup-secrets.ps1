$ErrorActionPreference = "Stop"

$EnvFile = if ($env:VIDEO2MD_ENV_FILE) { $env:VIDEO2MD_ENV_FILE } else { Join-Path $env:USERPROFILE ".video2md-cli.env" }

function Read-SecretText([string]$Prompt) {
  $secure = Read-Host $Prompt -AsSecureString
  $ptr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
  try {
    return [Runtime.InteropServices.Marshal]::PtrToStringBSTR($ptr)
  } finally {
    [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($ptr)
  }
}

function Read-Value([string]$Prompt, [string]$Default = "") {
  if ($Default) {
    $value = Read-Host "$Prompt [$Default]"
    if (-not $value) { return $Default }
    return $value
  }
  return Read-Host $Prompt
}

Write-Host "This writes local credentials to $EnvFile"
Write-Host "The file is private to this machine and should not be committed."

$dashscope = Read-SecretText "DashScope API key"
$ossId = Read-Value "OSS access key id"
$ossSecret = Read-SecretText "OSS access key secret"
$ossBucket = Read-Value "OSS bucket"
$ossEndpoint = Read-Value "OSS endpoint" "oss-cn-shanghai.aliyuncs.com"
$ossPrefix = Read-Value "OSS object prefix" "asr-temp/video2md/"

$content = @"
export DASHSCOPE_API_KEY='$dashscope'
export OSS_ACCESS_KEY_ID='$ossId'
export OSS_ACCESS_KEY_SECRET='$ossSecret'
export OSS_BUCKET='$ossBucket'
export OSS_ENDPOINT='$ossEndpoint'
export OSS_OBJECT_KEY_PREFIX='$ossPrefix'
export OSS_READ_URL_TTL=2h
"@

Set-Content -Path $EnvFile -Value $content -Encoding UTF8
Write-Host "Wrote $EnvFile"
