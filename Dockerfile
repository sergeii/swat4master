FROM golang:1.18-alpine AS build

ARG build_commit_sha="-"
ARG build_version="-"
ARG build_time="-"

ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

ARG _pkg="github.com/sergeii/swat4master/cmd/swat4master"

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
    -ldflags="-X '$_pkg/build.Time=$build_time' -X '$_pkg/build.Commit=$build_commit_sha' -X '$_pkg/build.Version=$build_version'" \
    -o /app/bin/swat4master \
    /app/src/cmd/swat4master

FROM scratch

WORKDIR /

COPY --from=build /app/bin/swat4master /swat4master
COPY --from=build /etc/passwd /etc/passwd

USER scratch

EXPOSE 27900/udp
EXPOSE 28910
EXPOSE 3000
EXPOSE 9000

ENTRYPOINT ["/swat4master"]
