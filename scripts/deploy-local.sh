#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "Usage: $0 -h <host> [-v <version>]"
  echo "  -h  SSH host to deploy to (required)"
  echo "  -v  Image version tag (default: local)"
  exit 1
}

VERSION="local"
HOST=""

while getopts "h:v:" opt; do
  case $opt in
    h) HOST="$OPTARG" ;;
    v) VERSION="$OPTARG" ;;
    *) usage ;;
  esac
done

[[ -z "$HOST" ]] && usage

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

build_and_push() {
  local context="$1"
  local extra_args="${3:-}"
  local tag="folio-${VERSION}"

  echo "==> Building ${tag}..."
  # shellcheck disable=SC2086
  docker buildx build \
    --platform linux/arm64 \
    $extra_args \
    -t "$tag" \
    --load \
    "$context"

  echo "==> Pushing ${tag} to ${HOST}..."
  docker save "$tag" | ssh "$HOST" docker load
}

build_and_push "${REPO_ROOT}"
