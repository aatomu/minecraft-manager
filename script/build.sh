cd $(dirname "$0")

readonly TARGET=${1}

if [ "${TARGET}" == "java" ]; then
  docker rmi mc_java:24
  docker build --no-cache -f ../docker/java.Dockerfile --build-arg JAVA="24" --build-arg UID="$(id -u)" --build-arg GID="$(id -g)" -t mc_java:24 ../assets/server
  exit 1
fi

if [ "${TARGET}" == "bot" ]; then
  docker rmi mc_chat:develop
  docker build --no-cache -f ../docker/bot.Dockerfile --build-arg UID="$(id -u)" --build-arg GID="$(id -g)" -t mc_chat:develop ../
  exit 1
fi