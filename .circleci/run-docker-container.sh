#!/usr/bin/env bash
# Usage: run-docker-container <srcDir:srcDirInContainer> <outputDirInContainer:localOutputDir>
# This scripts runs a docker container, with the given directory being
# mounted into the container as $1 and an output directory to be stored
# back onto the host $2.
#
# Note: The syntax for srcDir and outputDir is expected to follow the
# same format as `docker run -v ./path/to/local:/path/in/container`.

set -e

CIRCLE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
SCRIPTS_DIR="$CIRCLE_DIR/../scripts"
LIB_DIR="$SCRIPTS_DIR/lib"

localDirArg="$1"
outputDirArg="$2"

help() {
  echo "Usage: run-docker-container <srcDir> <outputDir/--> <docker run command>"
}

if [[ -z $localDirArg ]]; then
  help
  exit 1
fi

# shellcheck source=../scripts/lib/logging.sh
source "$LIB_DIR/logging.sh"

# local directories
localDir=""
localDirInContainer=""

# output directories, this is files we bring back from the container
outputEnabled=false
outputDir=""
outputDirInContainer=""

if grep -q ":" <<<"$localDirArg"; then
  localDir="$(awk -F ':' '{ print $1 }' <<<"$localDirArg")"
  localDirInContainer="$(awk -F ':' '{ print $2 }' <<<"$localDirArg")"
else
  localDir="$localDirArg"
  localDirInContainer="$localDir"
fi

# skip outputDir when --
if [[ -n $outputDirArg ]] && [[ $outputDirArg != "--" ]]; then
  outputEnabled=true
  if grep -q ":" <<<"$outputDirArg"; then
    outputDirInContainer="$(awk -F ':' '{ print $1 }' <<<"$outputDirArg")"
    outputDir="$(awk -F ':' '{ print $2 }' <<<"$outputDirArg")"
  else
    outputDir="$outputDirArg"
    outputDirInContainer="$outputDir"
  fi
fi

# cleanup docker images
cleanup() {
  info_sub "cleaning up docker container(s)"
  docker stop -t0 data-producer || true
  docker rm data data-producer || true
}
trap cleanup EXIT INT TERM

info "Creating Docker container"

info_sub "creating docker volume"
docker create -v "$localDirInContainer" --name data gcr.io/outreach-docker/alpine:3.12 /bin/true

info_sub "copying data into volume"
docker cp "$localDir" data:"$localDirInContainer"

shift
shift

info "Running supplied Docker command"
# NOTE: any container with --rm will fail, need to check for that
docker run --name data-producer --volumes-from data "$@"

# if we have an output directory, then use it
if [[ $outputEnabled == "true" ]]; then
  info "Grabbing files from container"
  docker cp data-producer:"$outputDirInContainer" "$outputDir"
fi
