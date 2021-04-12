#!/usr/bin/env bash

if [[ -n $CI ]]; then
  /usr/local/bin/clang-format "$@"
else
  docker run --rm -v "$(pwd):$(pwd)" -w "$(pwd)" gcr.io/outreach-docker/clang-format "$@"
fi
