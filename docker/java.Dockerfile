ARG JAVA=21
# MARK: Manager compile
FROM golang:1.25.3-alpine AS manager

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
RUN CGO=ENABLED=0 go build -o /manager -ldflags "-s -w" ./

# MARK: Java resources
ARG JAVA
FROM eclipse-temurin:${JAVA}-jdk-alpine AS jdk

# MARK: Base image
FROM alpine:latest

#> Arguments
ARG USER_NAME=minecraft
ARG GROUP_NAME=minecraft
ARG UID=1000
ARG GID=1000

#> Copy manager
COPY --from=manager --chown=${UID}:${GID} /manager ./usr/local/bin/
#> Copy java resource
ENV JAVA_HOME=/opt/java/openjdk
COPY --from=jdk --chown=${UID}:${GID} ${JAVA_HOME} ${JAVA_HOME}
ENV PATH="${JAVA_HOME}/bin:${PATH}"

#> Set Timezone
ENV TZ=Asia/Tokyo

#> Scripts
# 1. install packages
# 2. add group
# 3. add user
RUN apk update \
  && apk add --no-cache tzdata shadow \
  && groupadd --force -g ${GID} ${GROUP_NAME} \
  && useradd -o -M -u ${UID} -g ${GID} ${USER_NAME} \
  && mkdir /resource \
  && chown -R ${USER_NAME}:${GROUP_NAME} /resource

#> Mount volume
VOLUME /resource
#> Change work dir
WORKDIR /resource
#> Change work user
USER ${USER_NAME}

#> Expose port
# Minecraft
EXPOSE 25565/tcp
# Minecraft rcon
EXPOSE 25575/tcp
# Simple voice chat
EXPOSE 24454/udp
# JVM manager
EXPOSE 80

ENTRYPOINT ["/usr/local/bin/manager"]