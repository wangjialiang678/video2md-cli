#!/usr/bin/env bash
set -euo pipefail

ENV_FILE="${VIDEO2MD_ENV_FILE:-$HOME/.video2md-cli.env}"

prompt_secret() {
  local label="$1"
  local value
  read -r -s -p "$label: " value
  echo >&2
  printf "%s" "$value"
}

prompt_value() {
  local label="$1"
  local default="${2:-}"
  local value
  if [[ -n "$default" ]]; then
    read -r -p "$label [$default]: " value
    printf "%s" "${value:-$default}"
  else
    read -r -p "$label: " value
    printf "%s" "$value"
  fi
}

shell_quote() {
  local value="$1"
  printf "'%s'" "$(printf "%s" "$value" | sed "s/'/'\\\\''/g")"
}

echo "This writes local credentials to $ENV_FILE"
echo "The file is private to this machine and should not be committed."

dashscope="$(prompt_secret "DashScope API key")"
oss_id="$(prompt_value "OSS access key id")"
oss_secret="$(prompt_secret "OSS access key secret")"
oss_bucket="$(prompt_value "OSS bucket")"
oss_endpoint="$(prompt_value "OSS endpoint" "oss-cn-shanghai.aliyuncs.com")"
oss_prefix="$(prompt_value "OSS object prefix" "asr-temp/video2md/")"

umask 077
cat > "$ENV_FILE" <<EOF
export DASHSCOPE_API_KEY=$(shell_quote "$dashscope")
export OSS_ACCESS_KEY_ID=$(shell_quote "$oss_id")
export OSS_ACCESS_KEY_SECRET=$(shell_quote "$oss_secret")
export OSS_BUCKET=$(shell_quote "$oss_bucket")
export OSS_ENDPOINT=$(shell_quote "$oss_endpoint")
export OSS_OBJECT_KEY_PREFIX=$(shell_quote "$oss_prefix")
export OSS_READ_URL_TTL=2h
EOF
chmod 0600 "$ENV_FILE"

echo "Wrote $ENV_FILE"
