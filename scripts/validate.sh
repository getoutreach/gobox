#!/usr/bin/env bash
set -e

# The linter is flaky in some environments so we allow it to be overridden.
# Also, if your editor already supports linting, you can make your tests run
# faster at little cost with:
# `LINTER=/bin/true make test``
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
LINTER="${LINTER:-"$DIR/golangci-lint.sh"}"
SHELLFMTPATH="$DIR/shfmt.sh"
SHELLCHECKPATH="$DIR/shellcheck.sh"

# shellcheck source=./lib/logging.sh
source "$DIR/lib/logging.sh"
# shellcheck source=./lib/runtimes.sh
source "$DIR/lib/runtimes.sh"

# Run shellcheck on shell-scripts, only if installed.
info "Running shellcheck"
# Make sure to ignore the monitoring/.terraform directory
# shellcheck disable=SC2038
if ! git ls-files '*.sh' | xargs -n40 "${SHELLCHECKPATH}" -x -P SCRIPTDIR; then
  error "shellcheck failed on some files. Run 'make fmt' to fix."
  exit 1
fi

info "Running shfmt"
if ! git ls-files '*.sh' | xargs -n40 "$SHELLFMTPATH" -s -d; then
  error "shfmt failed on some files. Run 'make fmt' to fix."
  exit 1
fi

info "Running clang-format"
if ! git ls-files '*.proto' | xargs -n40 "$DIR"/clang-format-validate.sh; then
  error "clang-format failed on some files. Run 'make fmt' to fix."
  exit 1
fi

info "Running Go linter"
"$LINTER" -c "$(dirname "$0")/golangci.yml" --build-tags "$TEST_TAGS" --timeout 10m run ./...
