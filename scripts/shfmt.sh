#!/usr/bin/env bash
# This is a wrapper around gobin.sh to run shfmt.
# Useful for using the correct version of shfmt
# with your editor.

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
GOBIN="$DIR/gobin.sh"
SHFMT_VERSION="3.1.2"

# Always set simplify mode.
args=("-s" "$@")
exec "$GOBIN" "mvdan.cc/sh/v3/cmd/shfmt@v$SHFMT_VERSION" "${args[@]}"
