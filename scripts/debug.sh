#!/bin/bash
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
GOBIN="$DIR/gobin.sh"

# shellcheck source=./host_env.sh
source "$DIR/host_env.sh"

export GOPROXY=https://proxy.golang.org
export GOPRIVATE="github.com/getoutreach/*"
export CGO_ENABLED=1

set -ex

exec "$GOBIN" github.com/go-delve/delve/cmd/dlv debug --build-flags="-tags=or_dev" "$DIR/../cmd/gobox"
