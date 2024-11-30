#!/bin/bash
cd $(dirname "$0")
readonly SCRIPT_PATH="${PWD}"

# Arg
readonly SERVER_NAME="${1}"
# Const
readonly DOCKER_SOCK="/var/run/docker.sock"
readonly SSH_IDENTITY="${HOME}/.ssh/minecraft-manager"
readonly CONFIG_DIR="$(cd ${SCRIPT_PATH}/../config ; echo ${PWD})"

# SSHkey check
if [ ! -e ${SSH_IDENTITY} ]; then
  echo "[INFO]: Create minecraft-manager ssh-key"
  ssh-keygen -t ed25519 -f ${HOME}/.ssh/minecraft-manager -N ""
  echo -e "#minecraft-manager\n$(cat ${HOME}/.ssh/minecraft-manager.pub)" >>${HOME}/.ssh/authorized_keys
  echo "[INFO]: Finish minecraft-manager ssh-key"
fi

# Docker image check
if [ "${SERVER_NAME}" == "" ]; then
  echo "[INFO]: Docker image remove"
  docker rmi mc_chat
  echo "[INFO]: Docker image build start"
  docker build --no-cache -f ../docker/bot.dockerfile -t mc_chat:latest ../
  echo "[INFO]: Docker image build finish"
  exit 0
fi

# Load Environment
source ../config/${SERVER_NAME}.env

# Container check
if [ "$(docker ps -a -q --filter name=^${SERVER_NAME}_chat | wc -l)" == "0" ]; then
  echo "[INFO]: Docker \"${SERVER_NAME}_chat\" container start"
  docker run -itd --name ${SERVER_NAME}_chat -v ${SSH_IDENTITY}:/identity -v ${CONFIG_DIR}:/config -v ${server_dir}/logs:/logs -v ${DOCKER_SOCK}:/var/run/docker.sock --env-file="${CONFIG_DIR}/${SERVER_NAME}.env" --network=host mc_chat:latest --name="${SERVER_NAME}" 
else
  echo "[INFO]: Docker \"${SERVER_NAME}_chat\" container stop"
  docker stop ${SERVER_NAME}_chat
  echo "[INFO]: Docker \"${SERVER_NAME}_chat\" container remove"
  docker rm ${SERVER_NAME}_chat
fi
