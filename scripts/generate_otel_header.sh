#!/usr/bin/env bash
set -euo pipefail

# Generates AGENT_OTEL_EXPORTER_OTLP_HEADERS value from Langfuse keys.
# Usage:
#   ./scripts/generate_otel_header.sh <LANGFUSE_PUBLIC_KEY> <LANGFUSE_SECRET_KEY>
#   ./scripts/generate_otel_header.sh <path-to-.env>
# Example:
#   ./scripts/generate_otel_header.sh pk-lf-xxxx sk-lf-yyyy
#   ./scripts/generate_otel_header.sh ./local-custom/config/.env

usage() {
  echo "Usage:" >&2
  echo "  $0 <LANGFUSE_PUBLIC_KEY> <LANGFUSE_SECRET_KEY>" >&2
  echo "  $0 <path-to-.env>" >&2
}

read_env_value() {
  local key="$1"
  local env_file="$2"
  local raw

  raw=$(grep -E "^${key}=" "$env_file" | tail -1 | cut -d'=' -f2- || true)
  raw=$(echo "$raw" | tr -d '\r\n')
  raw=${raw#\"}
  raw=${raw%\"}
  raw=${raw#"'"}
  raw=${raw%"'"}
  echo "$raw"
}

if [ "$#" -eq 2 ]; then
  PUBLIC_KEY="$1"
  SECRET_KEY="$2"
elif [ "$#" -eq 1 ]; then
  ENV_FILE="$1"
  if [ ! -f "$ENV_FILE" ]; then
    echo "Error: env file not found: $ENV_FILE" >&2
    exit 1
  fi
  PUBLIC_KEY="$(read_env_value "LANGFUSE_PUBLIC_KEY" "$ENV_FILE")"
  SECRET_KEY="$(read_env_value "LANGFUSE_SECRET_KEY" "$ENV_FILE")"
else
  usage
  exit 1
fi

if [ -z "$PUBLIC_KEY" ] || [ -z "$SECRET_KEY" ]; then
  echo "Error: LANGFUSE_PUBLIC_KEY and LANGFUSE_SECRET_KEY must be non-empty" >&2
  exit 1
fi

if command -v base64 >/dev/null 2>&1; then
  # Remove any potential line wrapping from base64 output.
  B64="$(printf '%s' "${PUBLIC_KEY}:${SECRET_KEY}" | base64 | tr -d '\n\r')"
else
  echo "Error: base64 command not found" >&2
  exit 1
fi

echo "AGENT_OTEL_EXPORTER_OTLP_HEADERS=\"Authorization=Basic ${B64}\""
