#!/usr/bin/env bash
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"

# shellcheck source=./logging.sh
source "$DIR/logging.sh"

level="$1"
shift

"$level" "$@"
