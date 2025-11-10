FROM golang:1.23-alpine AS build

ARG build_commit_sha="-"
ARG build_version="-"
ARG build_time="-"

ARG _pkg="github.com/sergeii/swat4master/cmd/swat4master"

# Create dedicated non-root user for both build and runtime
RUN adduser \
    --disabled-password \
    --uid=9999 \
    --shell /sbin/nologin \
    app

RUN mkdir -p /app/src \
    && chown -R app:app /app

USER app

WORKDIR /app/src

# Download dependencies
COPY go.mod go.sum ./
RUN go mod download && \
    go mod verify

COPY --chown=app:app . .

# Generate swagger docs
RUN go run github.com/swaggo/swag/cmd/swag@v1.8.10 \
     init -g schema.go -o api/docs/ -d api/,internal/rest/

# Build the actual binary
RUN CGO_ENABLED=0 GOEXPERIMENT=loopvar \
    go build  \
    -v \
    -ldflags="-X '$_pkg/build.Time=$build_time' -X '$_pkg/build.Commit=$build_commit_sha' -X '$_pkg/build.Version=$build_version'" \
    -o /app/bin/swat4master \
    /app/src/cmd/swat4master

FROM scratch

# Gin runs in release mode
ENV GIN_MODE=release

COPY --from=build /app/bin/swat4master /swat4master
COPY --from=build /etc/passwd /etc/passwd

USER app

EXPOSE 27900/udp
EXPOSE 28910
EXPOSE 3000
EXPOSE 9000

ENTRYPOINT ["/swat4master"]
