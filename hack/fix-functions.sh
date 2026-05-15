#!/usr/bin/env bash

set -euo pipefail

# Auto-fixes function modules: tidies go.mod/go.sum and applies golangci-lint
# auto-fixes (formatting, import ordering, and linters marked auto-fix).
# Run this locally before verify-functions.sh to clear mechanical issues.
# You can pass a single function name as an argument to fix only that module.

repo_root=$(cd "$(dirname "$0")/.." && pwd)
functions_dir="$repo_root/functions"
golangci_config="$repo_root/.golangci.yml"
export GOWORK="$repo_root/go.work"

if ! command -v golangci-lint >/dev/null 2>&1; then
  echo "error: golangci-lint is not installed or not on PATH" >&2
  echo "install it from https://golangci-lint.run/welcome/install/" >&2
  exit 1
fi

fix_function() {
  local function_dir="$1"
  local function_name
  function_name=$(basename "$function_dir")

  echo "==> $function_name: go mod tidy"
  (
    cd "$function_dir"
    go mod tidy
  )

  echo "==> $function_name: golangci-lint --fix"
  (
    cd "$function_dir"
    golangci-lint run --fix --config "$golangci_config"
  )
}

if [[ $# -gt 0 ]]; then
  target="$functions_dir/$1"
  if [[ ! -d "$target" ]]; then
    echo "error: function '$1' not found under $functions_dir" >&2
    exit 1
  fi
  fix_function "$target"
  exit 0
fi

while IFS= read -r -d '' dir; do
  fix_function "$dir"
done < <(find "$functions_dir" -mindepth 1 -maxdepth 1 -type d -print0 | sort -z)
