#!/usr/bin/env bash

MISSING=()
for tool in "$@"; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    MISSING+=("${tool}")
  fi
done

JOINED_MISSING=$(printf " %s" "${MISSING[@]}")
JOINED_MISSING=${JOINED_MISSING:1}

if [[ -n ${JOINED_MISSING} ]]; then
  echo "Missing executables: ${JOINED_MISSING}" >/dev/stderr
  echo "" >/dev/stderr
  echo "Refer to README.md for info on how to install them." >/dev/stderr
  echo "If you know what you're doing, use SKIP_CHECK_DEPS=1 and try again." >/dev/stderr
  exit 1
fi
