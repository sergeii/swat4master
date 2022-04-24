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

RUN mkdir -p /application/src \
    && mkdir -p /application/bin \
    && chown -R scratch:scratch /application

USER scratch
WORKDIR /app/src

COPY go.mod go.sum ./
RUN go mod download && \
    go mod verify

COPY . .
RUN go build  \
    -v \
    -ldflags="-X '$_pkg/build.Time=$build_time' -X '$_pkg/build.Commit=$build_commit_sha' -X '$_pkg/build.Version=$build_version'" \
    -o /application/bin/swat4master \
    /application/src/cmd/swat4master

FROM scratch

WORKDIR /

COPY --from=build /app/bin/swat4master /swat4master
COPY --from=build /etc/passwd /etc/passwd

USER scratch

EXPOSE 27900/udp
EXPOSE 28910

ENTRYPOINT ["/swat4master"]
