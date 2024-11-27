#!/bin/bash
# 移動
cd $(dirname "$0")

# 変数
readonly SERVER_NAME=${1}

# 鯖確認
if [ ! -e "../config/${SERVER_NAME}.env" ]; then
  echo "[ERROR] Not Found ../config/${SERVER_NAME}.env"
  exit 1
fi

# 実行分岐
if [ "$(docker ps -a | grep "${SERVER_NAME}_mc")" == "" ]; then
  # 起動
  source ../config/${SERVER_NAME}.env
  echo "Java       : mc_java:${Java}"
  echo "Server     : ${SERVER_NAME}"
  echo "DataDir    : ${Dir}"
  echo "schemaDir  : ${schemaDir}"
  echo "structDir  : ${structDir}"
  echo "Boot       : ${JVM} ${Jar} ${subOps}"
  Ops="-id --rm --name=${SERVER_NAME}_mc --network=host -v ${Dir}:/MC -v ${schemaDir}:/schematics -v ${structDir}:/structures"
  # Java Image Check
  if [ "$(docker images mc_java:${Java})" == "" ]; then
    echo "[Error] Docker Image Has Not Found: \"mc_java:${Java}\""
    exit 0
  fi

  echo "[INFO] Server Starting"
  echo "docker run ${Ops} mc_java:${Java} ${Jar}"
  docker run ${Ops} mc_java:${Java} ${JVM} -jar ${Jar} ${subOps}
else
  echo "[ERROR] Server Is Booted!"
fi
