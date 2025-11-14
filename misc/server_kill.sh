#!/bin/bash
cd $(dirname "$0")

# Arg
readonly SERVER_NAME=${1}

# 実行分岐
if [ "$(docker ps -a | grep "${SERVER_NAME}_mc")" != "" ]; then
  # 起動
  echo "[INFO]: Server             : ${SERVER_NAME}"

  DOCKER_COMMAND="docker stop -t 60 ${SERVER_NAME}_mc"
  echo "${DOCKER_COMMAND}"
  ${DOCKER_COMMAND}
  sleep 10s

  DOCKER_COMMAND="docker kill ${SERVER_NAME}_mc"
  echo "${DOCKER_COMMAND}"
  ${DOCKER_COMMAND}
else
  echo "[ERROR]: Server is not running!"
fi
