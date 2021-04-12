#!/usr/bin/env bash

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"

retcode=0

for filename in "$@"; do
  if "$DIR"/clang-format.sh -style=file -output-replacements-xml "$filename" | grep -q '<replacement\>'; then
    echo "'$filename' needs formatting" 1>&2
    retcode=1
  fi
done

exit $retcode
