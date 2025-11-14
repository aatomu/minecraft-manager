# app build
FROM golang:latest AS build

#go系統
RUN mkdir /app
WORKDIR /app
ADD ./* /app/
RUN CGO_ENABLED=0 go build -o /chat .

# 最小イメージ
FROM alpine
COPY --from=build /chat /usr/bin/
# TimeZoneの指定
ENV TZ=Asia/Tokyo
ENV LANG=ja_JP.UTF-8

# 引き取り変数
ARG USER_NAME=minecraft
ARG GROUP_NAME=minecraft
ARG UID=1000
ARG GID=1000

# install
RUN chmod +x /usr/bin/chat \
  && apk add --update-cache --no-cache \
  tzdata \
  openssh-client \
  && touch identity \
  && mkdir /root/.ssh \
  && chmod 700 /root/.ssh \
  && echo -e "host localhost\n  StrictHostKeyChecking no" > /root/.ssh/config \
  && chmod 600 /root/.ssh/config \
  && mkdir /config \
  && addgroup -g $GID $GROUP_NAME \
  && adduser -u $UID -G $GROUP_NAME -D -H $USER_NAME

  # User名 変更
USER minecraft

# 起動
ENTRYPOINT ["chat"]
