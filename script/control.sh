#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

# MARK: Constants
readonly DISCORD_IMAGE="mc_chat:develop"
LOCAL_UID="$(id -u)"
readonly LOCAL_UID
LOCAL_GID="$(id -g)"
readonly LOCAL_GID
# MARK: > Log Header
readonly LEVEL_INFO="[INFO ]:"
readonly LEVEL_WARN="[WARN ]:"
readonly LEVEL_ERROR="[ERROR]:"
# MARK: > Suffix
readonly SUFFIX_NETWORK="_net"
readonly SUFFIX_SERVER=".server"
readonly SUFFIX_DISCORD=".discord"

# MARK: Functions
# MARK: > check_host_files()
check_host_files() {
  # check_host_files <FILE 1> <FILE 2> ...
  for file in "$@"; do
    if [ ! -e "$file" ]; then
      echo "${LEVEL_ERROR} Host file/directory '$file' not found. Cannot proceed."
      exit 1
    fi
  done
}

# MARK: > manage_network()
manage_network() {
  local SERVER="${1}"
  local ACTION="${2}"

  local -r NETWORK_NAME="${SERVER}${SUFFIX_NETWORK}"

  case "${ACTION}" in
  # MARK: >> create
  "create")
    if ! docker network ls | grep -q "${NETWORK_NAME}"; then
      echo "${LEVEL_INFO} Creating network '${NETWORK_NAME}'..."
      docker network create "${NETWORK_NAME}"
    else
      echo "${LEVEL_WARN} Network '${NETWORK_NAME}' already exists."
    fi
    ;;
  "remove")
  # MARK: >> remove
    if docker network ls | grep -q "${NETWORK_NAME}"; then
      if ! docker network inspect -f '{{.Containers}}' "${NETWORK_NAME}" | grep -q -v 'map\[\]'; then
        echo "${LEVEL_INFO} Removing network '${NETWORK_NAME}'..."
        docker network rm "${NETWORK_NAME}"
      else
        echo "${LEVEL_WARN} Network '${NETWORK_NAME}' still has attached containers. Skipping removal."
      fi
    fi
    ;;
  esac
}

# MARK: > run_container()
run_container() {
  local -r SERVER="${1}"
  local -r SERVICE="${2}"

  local -r NETWORK_NAME="${SERVER}${SUFFIX_NETWORK}"

  case "${SERVICE}" in
  # MARK: >> server
  "server")
    local -r CONTAINER_NAME="${SERVER}${SUFFIX_SERVER}"

    check_host_files "${SERVER_DATA}" "${SERVER_BACKUP}"

    # ポートマッピング
    local PORTS="-p ${SERVER_PORT_MAP}"
    if [ -n "${RESTAPI_PORT:-}" ]; then
      PORTS="${PORTS} -p ${RESTAPI_PORT}:80"
    fi

    echo "${LEVEL_INFO} Running server container ${CONTAINER_NAME} with ports: ${PORTS}"
    docker run -d \
      --name "${CONTAINER_NAME}" \
      --network "${NETWORK_NAME}" \
      --restart unless-stopped \
      --user "${LOCAL_UID}:${LOCAL_GID}" \
      ${PORTS} \
      -v "${SERVER_DATA}:/mnt/resource:rw" \
      -v "${SERVER_BACKUP}:/mnt/backup:rw" \
      -e "PASSWORD=${API_PASSWORD}" \
      "${SERVER_IMAGE}" \
      ${SERVER_ARGUMENTS}
    ;;

  # MARK: >> discord
  "discord")
    local -r CONTAINER_NAME="${SERVER}${SUFFIX_DISCORD}"

    check_host_files "${LOG_SETTING}"

    echo "${LEVEL_INFO} Running discord container ${CONTAINER_NAME}."
    docker run -d \
      --name "${CONTAINER_NAME}" \
      --network "${NETWORK_NAME}" \
      --restart unless-stopped \
      --user "${LOCAL_UID}:${LOCAL_GID}" \
      -v "${LOG_SETTING}:/mnt/logs.json" \
      -e "PASSWORD=${API_PASSWORD}" \
      -e "BOT_TOKEN=${BOT_TOKEN}" \
      -e "ADMIN_ROLE_ID=${ADMIN_ROLE_ID}" \
      -e "READ_CHANNEL_ID=${READ_CHANNEL_ID}" \
      -e "SEND_WEBHOOK_URL=${SEND_WEBHOOK_URL}" \
      -e "MANAGER_URL=http://${SERVER}.server" \
      "${DISCORD_IMAGE}"
    ;;
  esac
}

# MARK: > container_action()
container_action() {
  local -r SERVER="${1}"
  local -r SERVICE="${2}"
  local -r ACTION="${3}"

  declare CONTAINER_NAME
  case "${SERVICE}" in
  "server") CONTAINER_NAME="${SERVER}${SUFFIX_SERVER}" ;;
  "discord") CONTAINER_NAME="${SERVER}${SUFFIX_DISCORD}" ;;
  esac

  local -r CONTAINER_EXISTS=$(docker ps -a -q -f name="${CONTAINER_NAME}")
  local -r CONTAINER_RUNNING=$(docker ps -q -f name="${CONTAINER_NAME}")

  case "${ACTION}" in
  # MARK: >> up
  up)
    if [ -n "${CONTAINER_EXISTS}" ]; then
      echo "${LEVEL_ERROR} Container '${CONTAINER_NAME}' is already running or exists. Aborting start."
      return 1
    fi
    echo "${LEVEL_INFO} Container '${CONTAINER_NAME}' not running. Starting..."

    manage_network "${SERVER}" create
    run_container "${SERVER}" "${SERVICE}"
    ;;
  # MARK: >> down
  down)
    if [ -z "${CONTAINER_EXISTS}" ]; then
      echo "${LEVEL_INFO} Container '${CONTAINER_NAME}' does not exist. Skipping down."
      return 0
    fi

    if [ -n "${CONTAINER_RUNNING}" ]; then
      echo "${LEVEL_INFO} Stopping and removing ${CONTAINER_NAME}..."
      docker stop "${CONTAINER_NAME}"
    fi

    docker rm "${CONTAINER_NAME}"
    echo "${LEVEL_INFO} Successfully stopped and removed ${CONTAINER_NAME}."
    return 0
    ;;
  # MARK: >> restart
  restart)
    if [ -z "${CONTAINER_EXISTS}" ]; then
      echo "${LEVEL_WARN} Container '${CONTAINER_NAME}' does not exist. Attempting to start (up) instead."
      container_action "${SERVER}" "${SERVICE}" "up"
      return $?
    fi

    echo "${LEVEL_INFO} Restarting ${CONTAINER_NAME}..."
    docker restart "${CONTAINER_NAME}"
    echo "${LEVEL_INFO} Successfully restarted ${CONTAINER_NAME}."
    return 0
    ;;
  esac
}

# MARK: Main
readonly ARG_SERVER="${1:-}"
readonly ENV_FILE="../config/${ARG_SERVER}.env"
readonly ARG_SERVICE="${2:-}"
readonly ARG_ACTION="${3:-}"

# MARK: > Validation
if [ -z "${ARG_SERVER}" ] || [ -z "${ARG_SERVICE}" ] || [ -z "${ARG_ACTION}" ]; then
  echo "Usage: $0 <server_name> <server|discord|all> <up|down|restart>"
  exit 1
fi

# MARK: > Load .env
if [ ! -f "${ENV_FILE}" ]; then
  echo "${LEVEL_ERROR} Environment file '${ENV_FILE}' not found."
  exit 1
fi
# shellcheck source=../config/example.env
source "${ENV_FILE}"

# MARK: > Check service
case "${ARG_SERVICE}" in
"server" | "discord" | "all") ;;
*)
  echo "${LEVEL_ERROR} Invalid service '${ARG_SERVICE}'. Use 'server', 'discord', or 'all'."
  exit 1
  ;;
esac

# MARK: > Check action
case "${ARG_ACTION}" in
"up" | "down" | "restart") ;;
*)
  echo "${LEVEL_ERROR} Invalid action '${ARG_ACTION}'. Use 'up', 'down', or 'restart'."
  exit 1
  ;;
esac

# MARK: > Action to services
declare ACTION_SERVICES
case "${ARG_SERVICE}" in
server) ACTION_SERVICES="server" ;;
discord) ACTION_SERVICES="discord" ;;
all) ACTION_SERVICES="server discord" ;;
esac

for TARGET_SERVICE in ${ACTION_SERVICES}; do
  container_action "${ARG_SERVER}" "${TARGET_SERVICE}" "${ARG_ACTION}"
done

# MARK: Cleanup network
if [ "${ARG_ACTION}" = "down" ] && [ "${ARG_SERVICE}" = "all" ]; then
  manage_network "${ARG_SERVER}" "remove"
fi
