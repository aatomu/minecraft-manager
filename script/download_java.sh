#!/bin/bash
cd $(dirname "$0")

# Arg
readonly VERSION=${1}

echo "[INFO]: Delete docker image: \`mc_java:${VERSION}\`"
docker rmi mc_java:${VERSION}
echo "[INFO]: Build start docker image: \`mc_java:${VERSION}\`"
docker build --no-cache -f ../docker/java.Dockerfile --build-arg JAVA="${VERSION}" --build-arg UID="$(id -u)" --build-arg GID="$(id -g)" -t mc_java:${VERSION} ../assets/server
echo "[INFO]: Build end docker image: \`mc_java:${VERSION}\`"
