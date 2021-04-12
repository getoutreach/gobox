#!/usr/bin/env bash
# This is a wrapper around gobin.sh to run golangci-lint.
# Useful for using the correct version of golangci-lint
# with your editor.

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
GOBIN="$DIR/gobin.sh"
GOLANGCI_LINT_VERSION="1.36.0"

if [[ -z $workspaceFolder ]]; then
  workspaceFolder="$DIR/.."
fi

# Enable only fast linters, and always use the correct config.
args=("--config=${workspaceFolder}/scripts/golangci.yml" "$@" "--fast" "--allow-parallel-runners")

exec "$GOBIN" "github.com/golangci/golangci-lint/cmd/golangci-lint@v$GOLANGCI_LINT_VERSION" "${args[@]}"
