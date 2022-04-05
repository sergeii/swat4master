FROM golang:1.18-alpine AS build

ARG build_commit_sha="-"
ARG build_version="-"
ARG build_time="-"

ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

RUN adduser \
    --disabled-password \
    --uid=9999 \
    scratch

RUN mkdir -p /app/src \
    && mkdir -p /app/bin \
    && chown -R scratch:scratch /app

USER scratch
WORKDIR /app/src

COPY go.mod go.sum ./
RUN go mod download && \
    go mod verify

COPY . .
RUN go build  \
    -v \
    -ldflags="-X 'main.BuildTime=$build_time' -X 'main.BuildCommit=$build_commit_sha' -X 'main.BuildVersion=$build_version'" \
    -o /app/bin/swat4master \
    /app/src/cmd/swat4master

FROM scratch

WORKDIR /

COPY --from=build /app/bin/swat4master /swat4master
COPY --from=build /etc/passwd /etc/passwd

USER scratch

EXPOSE 27900/udp
EXPOSE 28910

ENTRYPOINT ["/swat4master"]
