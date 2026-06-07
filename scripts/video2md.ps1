$ErrorActionPreference = "Stop"

$EnvFile = if ($env:VIDEO2MD_ENV_FILE) { $env:VIDEO2MD_ENV_FILE } else { Join-Path $env:USERPROFILE ".video2md-cli.env" }
$Bin = if ($env:VIDEO2MD_CLI_BIN) { $env:VIDEO2MD_CLI_BIN } else { Join-Path $env:USERPROFILE ".video2md-cli\bin\mp4-md.exe" }

if (Test-Path $EnvFile) {
  Get-Content $EnvFile | ForEach-Object {
    $line = $_.Trim()
    if ($line -eq "" -or $line.StartsWith("#")) {
      return
    }
    if ($line -match '^export\s+([^=]+)=(.*)$') {
      $name = $Matches[1].Trim()
      $value = $Matches[2].Trim()
      if (($value.StartsWith("'") -and $value.EndsWith("'")) -or ($value.StartsWith('"') -and $value.EndsWith('"'))) {
        $value = $value.Substring(1, $value.Length - 2)
      }
      [Environment]::SetEnvironmentVariable($name, $value, "Process")
    }
  }
}

& $Bin @args
exit $LASTEXITCODE
