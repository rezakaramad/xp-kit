#!/usr/bin/env bash

set -euo pipefail

# This script checks all functions in the functions/ directory by running
#   "go mod tidy",
#   "golangci-lint",
#   and "go test"
# in each function directory.
# You can also specify a single function name as an argument to check only that function.

# Variables
repo_root=$(cd "$(dirname "$0")/.." && pwd)
functions_dir="$repo_root/functions"
golangci_config="$repo_root/.golangci.yml"

# Check if golangci-lint is installed
# Holds the resolved path to golangci-lint if found, or empty string if not found
golangci_lint_bin=""
if command -v golangci-lint >/dev/null 2>&1; then
  golangci_lint_bin=$(command -v golangci-lint)
elif [[ -x "$HOME/go/bin/golangci-lint" ]]; then
  golangci_lint_bin="$HOME/go/bin/golangci-lint"
fi

# Exit if golangci-lint is not found
if [[ -z "$golangci_lint_bin" ]]; then
  echo "error: golangci-lint is not installed or not on PATH" >&2
  echo "install it from https://golangci-lint.run/welcome/install/" >&2
  exit 1
fi

# Check a single function directory
# Takes the path to the function directory as an argument
check_function() {
  local function_dir="$1"
  local function_name
  # E.g., for functions/xtenant-validate, this will be "xtenant-validate"
  function_name=$(basename "$function_dir")

  # Check that go.mod and go.sum are tidy before we run golangci-lint,
  # which can be very slow if there are many unused dependencies
  echo "==> $function_name: go mod tidy"
  (
    cd "$function_dir"
    go mod tidy
  )

  # Check that go.mod and go.sum are tidy by verifying that 
  # there are no changes after running "go mod tidy".
  echo "==> $function_name: verify go.mod/go.sum are tidy"
  (
    cd "$repo_root"
    git diff --exit-code -- "$function_dir/go.mod" "$function_dir/go.sum"
  )

  # Run golangci-lint and go test in the function directory
  echo "==> $function_name: golangci-lint"
  (
    cd "$function_dir"
    "$golangci_lint_bin" run --config "$golangci_config"
  )

  # Run go test in the function directory
  echo "==> $function_name: go test"
  (
    cd "$function_dir"
    go test ./...
  )
}

# If a function name is provided as an argument, check only that function.
# Otherwise, check all functions.
if [[ $# -gt 0 ]]; then
  target="$functions_dir/$1"
  if [[ ! -d "$target" ]]; then
    echo "error: function '$1' not found under $functions_dir" >&2
    exit 1
  fi
  check_function "$target"
  exit 0
fi

# Check all functions in the functions directory.
# We use find to get all subdirectories
while IFS= read -r -d '' dir; do
  check_function "$dir"
done < <(find "$functions_dir" -mindepth 1 -maxdepth 1 -type d -print0 | sort -z)
