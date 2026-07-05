#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "$0")" && pwd)"
binary_path="$repo_root/cryptoprice"

printf 'Building cryptoprice...\n'
go build -o "$binary_path" .

printf 'Built %s\n' "$binary_path"
printf 'Add %s to your PATH if you want to run cryptoprice from anywhere.\n' "$repo_root"
