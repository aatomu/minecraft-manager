# javaを指定
ARG JAVA
FROM eclipse-temurin:${JAVA} as jdk

# ubuntuに設定
FROM ubuntu

# 引き取り変数
ARG USERNAME=minecraft
ARG GROUPNAME=minecraft
ARG UID=1000
ARG GID=1000

# JDKインストール
ENV JAVA_HOME=/opt/java/openjdk
COPY --from=jdk $JAVA_HOME $JAVA_HOME
ENV PATH="${JAVA_HOME}/bin:${PATH}"
# TimeZoneの指定
ENV DEBIAN_FRONTEND=noninteractive
ENV TZ=Asia/Tokyo
ENV LANG=ja_JP.UTF-8

# /MCをマウント
VOLUME /MC
# 権限変更
RUN mkdir /MC \
  && chmod 777 /MC \
  && apt-get update \
  && apt-get install -y --no-install-recommends \
      tzdata \
      locales \
      language-pack-ja-base language-pack-ja \
  && apt-get -y clean \
  && rm -rf /var/lib/apt/lists/* \
  && groupadd --force -g $GID $GROUPNAME \
  && useradd --non-unique -u $UID -g $GID $USERNAME


# 作業ディレクトリを変更
WORKDIR /MC
# User名 変更
USER minecraft

# 起動
ENTRYPOINT ["java"]
CMD ["-Xmx2G","-jar","./server.jar"]