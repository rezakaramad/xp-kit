#!/usr/bin/env bash

set -euo pipefail

# Verifies all function modules are tidy, lint-clean, and pass tests.
# Run hack/fix-functions.sh first to auto-fix formatting and tidy go.mod/go.sum.
# You can pass a single function name as an argument to verify only that module.

repo_root=$(cd "$(dirname "$0")/.." && pwd)
functions_dir="$repo_root/functions"
golangci_config="$repo_root/.golangci.yml"
if [[ -f "$repo_root/go.work" ]]; then
  export GOWORK="$repo_root/go.work"
else
  export GOWORK=off
fi

if ! command -v golangci-lint >/dev/null 2>&1; then
  echo "error: golangci-lint is not installed or not on PATH" >&2
  echo "install it from https://golangci-lint.run/welcome/install/" >&2
  exit 1
fi

verify_function() {
  local function_dir="$1"
  local function_name
  function_name=$(basename "$function_dir")

  echo "==> $function_name: verify go.mod/go.sum are tidy"
  (
    cd "$repo_root"
    git diff --exit-code -- "$function_dir/go.mod" "$function_dir/go.sum"
  )

  echo "==> $function_name: golangci-lint"
  (
    cd "$function_dir"
    golangci-lint run --config "$golangci_config"
  )

  echo "==> $function_name: go test"
  (
    cd "$function_dir"
    go test ./...
  )
}

if [[ $# -gt 0 ]]; then
  target="$functions_dir/$1"
  if [[ ! -d "$target" ]]; then
    echo "error: function '$1' not found under $functions_dir" >&2
    exit 1
  fi
  verify_function "$target"
  exit 0
fi

while IFS= read -r -d '' dir; do
  verify_function "$dir"
done < <(find "$functions_dir" -mindepth 1 -maxdepth 1 -type d -print0 | sort -z)
