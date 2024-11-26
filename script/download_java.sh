#!/bin/bash
# 移動
cd $(dirname "$0")

# 変数
readonly VERSION=${1}

echo "[INFO] Docker Image ReBuild : Deleting Image"
docker rmi mc_java:${VERSION}
echo "[INFO] Docker Image ReBuild : Building Image"
docker build --no-cache -f ../docker/java.Dockerfile --build-arg JAVA="${VERSION}" --build-arg UID="`id -u`"  --build-arg GID="`id -g`" -t mc_java:${VERSION} .
echo "[INFO] Docker Image ReBuild : End RebuildImage"

