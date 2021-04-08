#!/usr/bin/env bash
# This is a wrapper around gobin.sh to run shellcheck.
# Useful for using the correct version of shellcheck
# with your editor.

SCRIPTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
BIN_DIR="$SCRIPTS_DIR/../bin"
SHELLCHECK_VERSION="0.7.1"
GOOS=$(go env GOOS)
ARCH=$(uname -m)
binPath="$BIN_DIR/shellcheck-$SHELLCHECK_VERSION"

# shellcheck source=./lib/logging.sh
source "$SCRIPTS_DIR/lib/logging.sh"

# Ensure $BIN_DIR exists, since GOBIN makes it, but
# we may run before it.
mkdir -p "$BIN_DIR"

tmp_dir=$(mktemp -d)

# Always set the correct script directory.
args=("-P" "SCRIPTDIR" "-x" "$@")

if [[ ! -e $binPath ]]; then
  {
    info "downloading shellcheck@$SHELLCHECK_VERSION to '$binPath'"

    # JIT download shellcheck
    curl --location --output "$tmp_dir/shellcheck.tar.xz" --silent \
      "https://github.com/koalaman/shellcheck/releases/download/v$SHELLCHECK_VERSION/shellcheck-v$SHELLCHECK_VERSION.$GOOS.$ARCH.tar.xz"

    pushd "$tmp_dir" >/dev/null || exit 1
    tar xf shellcheck.tar.xz
    mv "shellcheck-v$SHELLCHECK_VERSION/shellcheck" "$binPath"
    chmod +x "$binPath"
    popd >/dev/null || exit 1
    rm -rf "$tmp_dir"
  } >&2
fi

exec "$binPath" "${args[@]}"
