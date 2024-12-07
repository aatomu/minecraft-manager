#!/bin/bash
cd $(dirname "$0")

# Arg
readonly SERVER_NAME=${1}

# 鯖確認
if [ ! -e "../config/${SERVER_NAME}.env" ]; then
  echo "[ERROR]: Not found \`../config/${SERVER_NAME}.env\`"
  exit 1
fi

# 変数読み込み
source ../config/${SERVER_NAME}.env

# 実行分岐
if [ "$(docker ps -a | grep "${SERVER_NAME}_mc")" == "" ]; then
  # 起動
  echo "[INFO]: Java               : mc_java:${java}"
  echo "[INFO]: Server             : ${SERVER_NAME}"
  echo "[INFO]: Server directory   : ${server_dir}"
  echo "[INFO]: Custom directory   : ${custom_dir}"
  echo "[INFO]: Boot command       : java ${jvm_arg} -jar ${server_jar} ${server_arg}"
  readonly DOCKER_ARGUMENTS="-id --rm --name=${SERVER_NAME}_mc --network=host -v ${server_dir}:/MC -v ${custom_dir}:/custom"

  # Java Image Check
  if [ "$(docker images mc_java:${java})" == "" ]; then
    echo "[Error]: Docker image has not found: \`mc_java:${java}\`"
    exit 0
  fi

  echo "[INFO]: Server starting..."
  readonly DOCKER_COMMAND="docker run ${DOCKER_ARGUMENTS} mc_java:${java} ${jvm_arg} -jar ${server_jar} ${server_arg}"
  echo "${DOCKER_COMMAND}"
  ${DOCKER_COMMAND}
else
  echo "[ERROR]: Server is running!"
fi
