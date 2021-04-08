#!/usr/bin/env bash

set -e

# The linter is flaky in some environments so we allow it to be overridden.
# Also, if your editor already supports linting, you can make your tests run
# faster at little cost with:
# `LINTER=/bin/true make test``
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
LINTER="${LINTER:-"$DIR/golangci-lint.sh"}"

# shellcheck source=./lib/logging.sh
source "$DIR/lib/logging.sh"

if [[ -n $CI ]]; then
  TEST_TAGS=${TEST_TAGS:-or_test,or_int}
  export GOFLAGS="${GOFLAGS} -mod=readonly"
else
  TEST_TAGS=${TEST_TAGS:-or_test}
fi
export TEST_TAGS

if [[ $TEST_TAGS == *"or_int"* ]]; then
  BENCH_FLAGS=${BENCH_FLAGS:--bench=^Bench -benchtime=1x}
fi

if [[ -n $WITH_COVERAGE || -n $CI ]]; then
  COVER_FLAGS=${COVER_FLAGS:- -covermode=atomic -coverprofile=/tmp/coverage.out -cover}
fi

info "Verifying go.{mod,sum} files are up to date"
go mod tidy

# We only ever error on this in CI, since it's updated when we run the above...
# Eventually we can do `go mod tidy -check` or something else:
# https://github.com/golang/go/issues/27005
if [[ -n $CI ]]; then
  git diff --exit-code go.{mod,sum} || fatal "go.{mod,sum} are out of date, please run 'go mod tidy' and commit the result"
fi

# Perform linting and format validations
if [[ -n $SKIP_VALIDATE ]]; then
  info "Skipping linting and format validations"
else
  "$DIR/validate.sh"
fi

if [[ -z $CI && $TEST_TAGS == *"or_int"* ]]; then
  # shellcheck disable=SC2034
  cleanup="true"

  info "creating integration infrastructure"
fi

###Block(testextras)
# Add any additional test setup needed for your project here

###EndBlock(testextras)

info "Running go test ($TEST_TAGS)"
set -ex
# Why: We want these to split. For those wondering about "$@":
# https://stackoverflow.com/questions/5720194/how-do-i-pass-on-script-arguments-that-contain-quotes-spaces
# shellcheck disable=SC2086
go test $BENCH_FLAGS $COVER_FLAGS \
  -ldflags "-X github.com/getoutreach/go-outreach/v2/pkg/app.Version=testing" -tags="$TEST_TAGS" \
  "$@" ./...
