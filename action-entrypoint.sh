#!/usr/bin/env bash
# Maps GitHub Action inputs (TFU_* env vars) to tf-unlock CLI flags, resolving a
# platform-specific release binary (or building from source as a fallback).
set -euo pipefail

REPO="x7ssss/tf-unlock"

case "${TFU_RUNNER_OS:-$(uname -s)}" in
  Linux)   os="linux";   ext="" ;;
  macOS|Darwin) os="darwin"; ext="" ;;
  Windows|MINGW*|MSYS*|CYGWIN*) os="windows"; ext=".exe" ;;
  *) echo "::error::Unsupported OS: ${TFU_RUNNER_OS:-unknown}" >&2; exit 1 ;;
esac

case "${TFU_RUNNER_ARCH:-$(uname -m)}" in
  X64|x86_64|amd64|AMD64) arch="amd64" ;;
  ARM64|arm64|aarch64)    arch="arm64" ;;
  *) echo "::error::Unsupported architecture: ${TFU_RUNNER_ARCH:-unknown}" >&2; exit 1 ;;
esac

action_path="${TFU_ACTION_PATH:-$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)}"
bin_dir="${RUNNER_TEMP:-${TMPDIR:-/tmp}}/tf-unlock-bin"
mkdir -p "$bin_dir"
bin="$bin_dir/tf-unlock${ext}"

version="${TFU_VERSION:-}"
if [ -z "$version" ] && [[ "${TFU_ACTION_REF:-}" =~ ^v[0-9] ]]; then
  version="$TFU_ACTION_REF"
fi

download() {
  local asset="tf-unlock-${os}-${arch}${ext}" url
  if [ -n "$version" ]; then
    url="https://github.com/${REPO}/releases/download/${version}/${asset}"
  else
    url="https://github.com/${REPO}/releases/latest/download/${asset}"
  fi
  echo "Downloading ${url}"
  curl -fsSL --retry 3 -o "$bin" "$url"
  chmod +x "$bin"
}

build_local() {
  if ! command -v go >/dev/null 2>&1; then
    echo "::error::Release binary unavailable and Go toolchain not found for local build" >&2
    return 1
  fi
  echo "Building tf-unlock from source"
  (cd "$action_path" && CGO_ENABLED=0 go build -ldflags="-s -w" -o "$bin" .)
}

if [ -n "${TFU_USE_LOCAL_BUILD:-}" ]; then
  build_local
else
  download || { echo "::warning::Download failed, falling back to local build"; build_local; }
fi

args=()
[ -n "${TFU_BACKEND_TYPE:-}" ] && args+=(--backend-type "$TFU_BACKEND_TYPE")
[ -n "${TFU_TABLE_NAME:-}" ]   && args+=(--table-name "$TFU_TABLE_NAME")
[ -n "${TFU_S3_BUCKET:-}" ]    && args+=(--s3-bucket "$TFU_S3_BUCKET")
[ -n "${TFU_S3_KEY:-}" ]       && args+=(--s3-key "$TFU_S3_KEY")
[ -n "${TFU_GITHUB_TOKEN:-}" ] && args+=(--github-token "$TFU_GITHUB_TOKEN")
[ -n "${TFU_GITHUB_REPOSITORY:-}" ] && args+=(--github-repo "$TFU_GITHUB_REPOSITORY")
[ -n "${TFU_GITHUB_API_URL:-}" ]    && args+=(--github-api-url "$TFU_GITHUB_API_URL")
[ -n "${TFU_GITHUB_RUN_ID:-}" ]     && args+=(--github-run-id "$TFU_GITHUB_RUN_ID")
[ "${TFU_DRY_RUN:-false}" = "true" ] && args+=(--dry-run)

tolerance="${TFU_TIMEOUT_TOLERANCE:-30m}"

if [ -n "${TFU_LOCK_ID:-}" ]; then
  # Targeted break: lock ID must match and age must exceed tolerance.
  exec "$bin" break --yes --lock-id "$TFU_LOCK_ID" --stale-after "$tolerance" "${args[@]}"
fi
# No lock ID: auto mode only clears locks older than the tolerance.
exec "$bin" auto --stale-after "$tolerance" "${args[@]}"
