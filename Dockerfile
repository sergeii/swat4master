FROM golang:1.22-alpine AS build

ARG build_commit_sha="-"
ARG build_version="-"
ARG build_time="-"

ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64
ENV GOEXPERIMENT=loopvar
ENV PATH=/go/bin/linux_amd64:$PATH

ARG _pkg="github.com/sergeii/swat4master/cmd/swat4master"

RUN adduser \
    --disabled-password \
    --uid=9999 \
    scratch

RUN mkdir -p /app/src \
    && mkdir -p /app/bin \
    && chown -R scratch:scratch /app

USER scratch

RUN go install github.com/swaggo/swag/cmd/swag@v1.8.10

WORKDIR /app/src

COPY go.mod go.sum ./
RUN go mod download && \
    go mod verify

COPY --chown=scratch:scratch . .
RUN swag init -g schema.go -o api/docs/ -d api/,internal/rest/
RUN go build  \
    -v \
    -ldflags="-X '$_pkg/build.Time=$build_time' -X '$_pkg/build.Commit=$build_commit_sha' -X '$_pkg/build.Version=$build_version'" \
    -o /app/bin/swat4master \
    /app/src/cmd/swat4master

FROM scratch

ENV GIN_MODE=release

WORKDIR /

COPY --from=build /app/bin/swat4master /swat4master
COPY --from=build /etc/passwd /etc/passwd

USER scratch

EXPOSE 27900/udp
EXPOSE 28910
EXPOSE 3000
EXPOSE 9000

ENTRYPOINT ["/swat4master"]
