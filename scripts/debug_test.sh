#!/bin/bash
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
GOBIN="$DIR/gobin.sh"

# shellcheck source=./host_env.sh
source "$DIR/host_env.sh"

export GOPROXY=https://proxy.golang.org
export GOPRIVATE="github.com/getoutreach/*"
export CGO_ENABLED=1

if [[ -z $1 ]]; then
  echo "Please supply a package argument to '$(basename "$0")'"
  exit 1
fi

set -ex

exec "$GOBIN" github.com/go-delve/delve/cmd/dlv test --build-flags="-tags=or_test,or_int" "$1"
