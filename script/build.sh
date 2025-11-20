#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

# MARK: Functions
remove_image_if_exists() {
  local IMAGE="$1"

  if docker image inspect "${IMAGE}" >/dev/null 2>&1; then
    echo "Remove image: ${IMAGE}"
    docker rmi "${IMAGE}"
  fi
}

build_server() {
  local JAVA_IMAGE_PREFIX="mc_java"
  readonly JAVA_IMAGE_PREFIX

  local JAVA_VERSION="${1:-}"
  local JAVA_IMAGE="${JAVA_IMAGE_PREFIX}:dev-${JAVA_VERSION}"

  if [ -z "${JAVA_VERSION}" ]; then
    echo "Error: Java version not specified." >&2
    echo "Usage: $0 server <java version>"
    exit 1
  fi

  remove_image_if_exists "${JAVA_IMAGE}"

  echo "Build Java server image: ${JAVA_IMAGE}"
  docker build \
    --no-cache \
    -f ../docker/server/Dockerfile \
    --build-arg JAVA="${JAVA_VERSION}" \
    --build-arg UID="$(id -u)" \
    --build-arg GID="$(id -g)" \
    -t "${JAVA_IMAGE}" \
    ../docker/server
  echo "Built Java server image: ${JAVA_IMAGE}"
}

build_discord() {
  local DISCORD_IMAGE="mc_chat:develop"
  readonly DISCORD_IMAGE

  remove_image_if_exists "${DISCORD_IMAGE}"

  echo "Build discord bot image: ${DISCORD_IMAGE}"
  docker build \
    --no-cache \
    -f ../docker/discord/Dockerfile \
    --build-arg UID="$(id -u)" \
    --build-arg GID="$(id -g)" \
    -t "${DISCORD_IMAGE}" \
    ../docker/discord
  echo "Built discord bot image: ${DISCORD_IMAGE}"
}

show_usage() {
  echo "Usage:"
  echo "  $0 server <java version>"
  echo "  > build Java image with <java version>"
  echo "  $0 discord"
  echo "  > build discord bot image"
  echo "  $0 bot"
  echo "  > alias \"discord\""
  exit 1
}

# MARK: Main 
readonly SERVICE="${1:-}"
readonly ARG="${2:-}"

case "${SERVICE}" in
  server|jvm)
    build_server "${ARG}"
    ;;
  discord|bot)
    build_discord
    ;;
  *)
    show_usage
    ;;
esac