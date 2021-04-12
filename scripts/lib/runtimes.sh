#!/usr/bin/env bash
# runtimes for various languages, etc

APPNAME="gobox"

DOCKER_NODE="gcr.io/outreach-docker/node:12-alpine"

uid=$(id -u)
gid=$(id -g)

docker_opts=()
if [[ -t 1 ]]; then
  docker_opts+=("--interactive" "--tty")
fi

# Run a Node.JS command inside of a docker container, keeping node_modules
# inside of the container for optimal performance across Docker VM runtimes.
run_node_command() {
  local dir="$1"
  shift

  if [[ -n $CI ]]; then
    # Can't run in Docker right now when in CircleCI
    pushd "$dir" || exit
    if [[ $1 == -* ]]; then
      node "$@"
    else
      "$@"
    fi
    popd || exit
    return
  fi

  # Fallback if uid/gid is somehow empty
  if [[ -z $uid ]] || [[ -z $gid ]]; then
    echo "Error: Failed to determine the uid/gid of the current user. Defaulting to standard 1000." >&2
    uid=1000
    gid=1000
  fi

  groupName="localuser"
  userName="localuser"

  # node has the group/user we want in this docker image
  # TODO: add logic to replace / use the user with our id/gid?
  if [[ $gid == "1000" ]]; then
    groupName="node"
  fi
  if [[ $uid == "1000" ]]; then
    userName="node"
  fi

  if [[ ! -e "$HOME/.npmrc" ]] || [[ -d "$HOME/.npmrc" ]]; then
    # Cleanup the file, in case it was a directory
    rm -rf "$HOME/.npmrc"
    error "Please configure npm: https://outreach-io.atlassian.net/wiki/spaces/EN/pages/696255333/Setup+NPM+account"
    exit 1
  fi

  # Create the host container
  CONTAINER_ID=$(docker run --user root "${docker_opts[@]}" --rm -d \
    -v "$dir:/src" -v "$APPNAME-node-modules:/src/node_modules" -v "home-dot-cache:/home/$userName/.cache" \
    -v "$HOME/.npmrc:/home/$userName/.npmrc" \
    "$DOCKER_NODE" tail -f /dev/null)

  # Why: We want it to expand now.
  # shellcheck disable=SC2064
  trap "docker stop -t0 $CONTAINER_ID >/dev/null 2>&1 || true" EXIT

  # We need to make a group/user for our UID/GID on Linux/WSL
  docker exec "$CONTAINER_ID" sh \
    -c "addgroup -g $gid $groupName && adduser -u $uid -G $groupName $userName" 2>/dev/null || true

  # This is needed on macOS for permissions to work properly
  docker exec "$CONTAINER_ID" sh \
    -c "chown -R '$uid:$gid' /src/node_modules /home/$userName/.cache"

  # Exec the command the user wanted
  docker exec "${docker_opts[@]}" --user "$uid:$gid" -w "/src" --env "HOME=/home/$userName" "$CONTAINER_ID" /usr/local/bin/docker-entrypoint.sh "$@" || true
  exitCode=$?

  # cleanup the container, just in case it still exists
  docker stop -t0 "$CONTAINER_ID" >/dev/null 2>&1 || true

  return $exitCode
}
