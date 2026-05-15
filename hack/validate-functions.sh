#!/usr/bin/env bash

set -euo pipefail

# Validates all function modules: gofmt, mod-tidy, golangci-lint, govulncheck, unit tests.
# Run hack/lint-fix-functions.sh first to auto-fix formatting and tidy go.mod/go.sum.
# You can pass a single function name as an argument to validate only that module.

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

if ! command -v govulncheck >/dev/null 2>&1; then
  echo "error: govulncheck is not installed or not on PATH" >&2
  echo "install it with: go install golang.org/x/vuln/cmd/govulncheck@latest" >&2
  exit 1
fi

verify_function() {
  local function_dir="$1"
  local function_name
  function_name=$(basename "$function_dir")

  echo "==> $function_name: gofmt"
  (
    cd "$function_dir"
    unformatted=$(gofmt -l .)
    if [[ -n "$unformatted" ]]; then
      echo "error: the following files are not gofmt-formatted:" >&2
      echo "$unformatted" >&2
      exit 1
    fi
  )

  echo "==> $function_name: go mod tidy"
  (
    cd "$function_dir"
    go mod tidy
    if ! git diff --exit-code -- go.mod go.sum; then
      echo "error: go.mod/go.sum are not tidy, run 'go mod tidy'" >&2
      exit 1
    fi
  )

  echo "==> $function_name: golangci-lint"
  (
    cd "$function_dir"
    golangci-lint run --config "$golangci_config"
  )

  echo "==> $function_name: govulncheck"
  (
    cd "$function_dir"
    govulncheck ./...
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
