#!/bin/bash
cd $(dirname "$0")
readonly SCRIPT_PATH="${PWD}"

# Arg
readonly SERVER_NAME="${1}"
# Const
readonly SSH_IDENTITY="${HOME}/.ssh/minecraft-manager"
readonly CONFIG_DIR="$(cd ${SCRIPT_PATH}/../config ; echo ${PWD})"

# Image delete
if [ "${SERVER_NAME}" == "" ]; then
  echo "[INFO]: Delete docker image: \`mc_chat:develop\`"
  docker rmi mc_chat:develop
fi

# 新規ビルド
if [ "$(docker images | grep "mc_chat:develop")" == "" ]; then
  echo "[INFO]: Build start docker image: \`mc_chat:develop\`"
  docker build --no-cache -f ../docker/bot.Dockerfile --build-arg UID="$(id -u)" --build-arg GID="$(id -g)" -t mc_chat:develop ../
  echo "[INFO]: Build end docker image: \`mc_chat:develop\`"

  exit 0
fi

# SSHkey check
if [ ! -e ${SSH_IDENTITY} ]; then
  echo "[INFO]: Create minecraft-manager ssh-key"
  ssh-keygen -t ed25519 -f ${HOME}/.ssh/minecraft-manager -N ""
  echo -e "#minecraft-manager\n$(cat ${HOME}/.ssh/minecraft-manager.pub)" >>${HOME}/.ssh/authorized_keys
  echo "[INFO]: Finish minecraft-manager ssh-key"
fi

# Load Environment
source ../config/${SERVER_NAME}.env

# Container check
if [ "$(docker ps -a -q --filter name=^${SERVER_NAME}_chat | wc -l)" == "0" ]; then
  echo "[INFO]: Docker \`${SERVER_NAME}_chat\` container start"
  docker run -itd --name ${SERVER_NAME}_chat -v ${SSH_IDENTITY}:/identity -v ${CONFIG_DIR}:/config -v ${server_dir}/logs:/logs --env-file="${CONFIG_DIR}/${SERVER_NAME}.env" --network=host mc_chat:develop --name="${SERVER_NAME}" 
else
  echo "[INFO]: Docker \`${SERVER_NAME}_chat\` container stop"
  docker stop ${SERVER_NAME}_chat
  echo "[INFO]: Docker \`${SERVER_NAME}_chat\` container remove"
  docker rm ${SERVER_NAME}_chat
fi
