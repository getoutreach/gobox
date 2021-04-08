#!/usr/bin/env bash
# Configures CircleCI docker authentication
# for setup_remote_docker
set -e

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"

# shellcheck source=./logging.sh
source "$LIB_DIR/logging.sh"

if [[ -n $CIRCLECI ]]; then
  info "setting up docker authn"
  docker login \
    -u _json_key \
    --password-stdin \
    https://gcr.io <<<"${GCLOUD_SERVICE_ACCOUNT}"
fi
