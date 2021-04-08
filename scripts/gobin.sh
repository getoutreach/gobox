#!/usr/bin/env bash
#
# Run a golang binary using gobin

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
GOBINVERSION=v1.0.1
GOBINBOOTSTRAPVERSION=v0.0.14
GOBINBOOTSTRAPPATH="$DIR/../bin/gobin-go-1.15"
GOOS=$(go env GOOS)
GOARCH=$(go env GOARCH)

# Allow people who don't have GOPRIVATE set to use this
export GOPRIVATE='github.com/getoutreach/*'

# shellcheck source=./lib/logging.sh
source "$DIR/lib/logging.sh"

PRINT_PATH=false
if [[ $1 == "-p" ]]; then
  PRINT_PATH=true
  shift
fi

if [[ -z $1 ]] || [[ $1 =~ ^(--help|-h) ]]; then
  echo "Usage: $0 [-p|-h|--help] <package> [args]" >&2
  exit 1
fi

# TODO: When we move to go 1.16, remove this and replace with `go install`
if [[ ! -e $GOBINBOOTSTRAPPATH ]]; then
  {
    mkdir -p "$(dirname "$GOBINBOOTSTRAPPATH")"
    curl --location --output "$GOBINBOOTSTRAPPATH" --silent "https://github.com/getoutreach/gobin-fork/releases/download/$GOBINBOOTSTRAPVERSION/$GOOS-$GOARCH"
    chmod +x "$GOBINBOOTSTRAPPATH"
  } >&2
fi

if [[ $1 == "download-only" ]]; then
  exit 0
fi

# Fetch outreach's gobin using the OSS gobin until we can use go install
# gobin picks up the Go binary from the path and runs it from within
# the temp directory while building things.  This has the unfortunate
# side effect that the version of Go used depends on what was used
# with `asdf global golang <ver>` command.  To make this
# deterministic, we create a temp directory and fill it in with the
# current .tool-versions file and then convince gobin to use it.
# shellcheck disable=SC2155
gobin_tmpdir="$(mktemp -d -t gobin-XXXXXXXX)"
trap 'rm -rf "$gobin_tmpdir"' EXIT INT TERM
cp "$DIR/../.tool-versions" "$gobin_tmpdir/.tool-versions"

# Change into the temporary directory
pushd "$gobin_tmpdir" >/dev/null || exit 1
GOBINPATH=$(/usr/bin/env bash -c "export TMPDIR='$gobin_tmpdir'; unset GOFLAGS; '$GOBINBOOTSTRAPPATH' -p 'github.com/getoutreach/gobin/cmd/gobin@$GOBINVERSION'")
popd >/dev/null || exit 1
if [[ -z $GOBINPATH ]]; then
  echo "Error: Failed to bootstrap gobin" >&2
  exit 1
fi

BIN_PATH=$("$GOBINPATH" --skip-update -p "$1")
if [[ -z $BIN_PATH ]]; then
  echo "Error: Failed to run $1" >&2
  exit 1
fi

# Remove the module
shift

if [[ $PRINT_PATH == "true" ]]; then
  echo "$BIN_PATH"
  exit
fi

exec "$BIN_PATH" "$@"
