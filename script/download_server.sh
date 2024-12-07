#!/bin/bash
cd $(dirname "$0")

# Arg
readonly VERSION=${1}
readonly GITHUB_API_TOKEN=${2}
# Const
readonly USER_ID=$(id -u)
readonly GROUP_ID=$(id -g)

# Image Delete
if [ "${VERSION}" == "" ]; then
  echo "[INFO]: Delete docker image: \`mc_downloader:latest\`"
  docker rmi mc_downloader:latest
fi

# Image Build
if [ "$(docker images | grep "mc_downloader")" == "" ]; then
  echo "[INFO]: Build start docker image: \`mc_downloader:latest\`"
  docker build --no-cache -f ../docker/downloader.Dockerfile -t mc_downloader:latest ../
  echo "[INFO]: Build end docker image: \`mc_downloader:latest\`"

  exit 0
fi

cd ../download
rm -rd ${VERSION}
echo "[INFO]: Download start server.jar"
mkdir ${VERSION}/
docker run --name=download_server --rm --env Version=${VERSION} --env Token=${GITHUB_API_TOKEN} --env UserID=${USER_ID} --env GroupID=${GROUP_ID} -v ${PWD}/${VERSION}:/mcData mc_downloader:latest /download_server.sh
echo "[INFO]: Download end server.jar"
echo "[INFO]: Download start fabric mods"
mkdir ${VERSION}/mods
docker run --name=download_mods --rm --env Version=${VERSION} --env Token=${GITHUB_API_TOKEN} --env UserID=${USER_ID} --env GroupID=${GROUP_ID} -v ${PWD}/${VERSION}/mods:/mods mc_downloader:latest /download_mods.sh
echo "[INFO]: Download end fabric mods"
