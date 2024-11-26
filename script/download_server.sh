#!/bin/bash
# 移動
cd $(dirname "$0")

# 変数
readonly VERSION=${1}
readonly GITHUB_API_TOKEN=${2}
readonly USER_ID=$(id -u)
readonly GROUP_ID=$(id -g)

# Image削除
if [ "${VERSION}" == "" ]
 then
  echo "[INFO] Docker Image : Delete Image"
  docker rmi mc_downloader:latest
fi

# 新規ビルド
if [ "$(docker images | grep "mc_downloader")" == "" ]
 then
  echo "[INFO] Docker Image Build : mc_downloader"
  docker build --no-cache -f ../docker/downloader.Dockerfile -t mc_downloader:latest ../

  exit 0
fi

cd ../download
rm -rd ${VERSION}
mkdir ${VERSION}/
docker run --name=download_server --rm --env Version=${VERSION} --env Token=${GITHUB_API_TOKEN} --env UserID=${USER_ID} --env GroupID=${GROUP_ID} -v ${PWD}/${VERSION}:/mcData mc_downloader:latest /download_server.sh
mkdir ${VERSION}/mods
docker run --name=download_mods --rm --env Version=${VERSION} --env Token=${GITHUB_API_TOKEN} --env UserID=${USER_ID} --env GroupID=${GROUP_ID} -v ${PWD}/${VERSION}/mods:/mods mc_downloader:latest /download_mods.sh