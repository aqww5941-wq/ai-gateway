#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$repo_root"

config_path="${1:-config/gateway.yaml}"
if [[ ! -f "$config_path" ]]; then
  printf 'config file does not exist: %s\n' "$config_path" >&2
  exit 1
fi

go run ./cmd/build -target backend
exec ./bin/gateway -config "$config_path"
