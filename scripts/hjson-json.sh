#!/usr/bin/env bash
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"

# Ensure gobin has been downloaded
"$DIR/gobin.sh" download-only >/dev/null 2>&1

# TODO: use outreach gobin when it supports non-go modules. Or, just rewrite this in Go :)
HJSON_CLI=$("$DIR/../bin/gobin-go-1.15" -p github.com/hjson/hjson-go/hjson-cli)

EXISTING_VERSION="0.0.1"
if [[ -e $2 ]]; then
  EXISTING_VERSION=$(jq -r .version "$2")
fi

CONVERTED_JSON=$("$HJSON_CLI" -c "$1")

WARNING_COMMENT="{\"//\": \"DO NOT EDIT, EDIT $1\"}"
VERSION="{\"version\": \"$EXISTING_VERSION\"}"

jq "$WARNING_COMMENT + . + $VERSION" <<<"$CONVERTED_JSON" >"$2"
