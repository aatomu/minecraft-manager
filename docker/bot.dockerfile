# app build
FROM golang:1.18-alpine AS build

#go系統
RUN mkdir /app
WORKDIR /app
ADD ./assets/discord/* /app/
RUN go build -o /chat .

# 最小イメージ
FROM alpine
COPY --from=build /chat /usr/bin/
# TimeZoneの指定
ENV DEBIAN_FRONTEND=noninteractive
ENV TZ=Asia/Tokyo
ENV LANG=ja_JP.UTF-8

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
  && mkdir /config

# 起動
ENTRYPOINT ["chat"]
