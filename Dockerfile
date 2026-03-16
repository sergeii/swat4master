FROM golang:1.26.1-alpine AS build

ARG build_commit_sha="-"
ARG build_version="-"
ARG build_time="-"

RUN apk add --no-cache make

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
RUN make generate-api-docs

# Build the actual binary
RUN make build \
    OUTPUT=/app/bin/swat4master \
    BUILD_TIME=$build_time \
    BUILD_COMMIT=$build_commit_sha \
    BUILD_VERSION=$build_version

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
