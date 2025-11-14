# データ準備用
FROM ubuntu

RUN mkdir /mods /mcData \
  && apt-get update \
  && apt-get install -y curl jq

ADD ./assets/script/download_server.sh ./assets/script/download_mods.sh /

RUN chmod 700 /download_server.sh /download_mods.sh

ENTRYPOINT ["/bin/bash"]