# SWAT4 Master Server - Copilot Instructions

## Repository Overview

This repository implements a **GameSpy master server protocol replacement** for SWAT4 multiplayer gaming. When GameSpy shut down in 2013-2014, this project restored multiplayer functionality for SWAT4 by providing compatible server discovery and heartbeat services.

### Technology Stack
- **Language**: Go 1.23 (requires 1.22+)
- **Database**: Redis (required for all operations)
- **Framework**: Gin (HTTP), fx (dependency injection)
- **Size**: ~166 Go files, ~25k lines of code
- **Architecture**: Modular design with use cases, repositories, and entities

## Build & Development Commands

**Always run these commands from the repository root directory.**

### Prerequisites
```bash
# Verify Go version (1.22+ required)
go version

# Download and verify dependencies (always run first)
go mod download && go mod verify
```

### Build Commands
```bash
# Basic build
go build -o swat4master cmd/swat4master/main.go

# Build with version info (production-style)
go build -ldflags="-X 'github.com/sergeii/swat4master/cmd/swat4master/build.Time=$(date -u +'%Y-%m-%dT%H:%M:%SZ')' -X 'github.com/sergeii/swat4master/cmd/swat4master/build.Commit=dev' -X 'github.com/sergeii/swat4master/cmd/swat4master/build.Version=dev'" -o swat4master cmd/swat4master/main.go

# Test built binary
./swat4master --help
./swat4master version
```

### Testing
```bash
# Run all tests (takes ~5-10 seconds, may have 1 flaky browser test)
go test ./...

# Run tests with coverage
go test -v -race -coverprofile=coverage.out -covermode=atomic ./...

# Run specific test package
go test ./internal/core/usecases/addserver
go test ./pkg/gamespy/serverquery/gs1
```

### Linting
```bash
# Install compatible linter version (v2.1.6 from CI config)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.62.2

# Run linter (takes ~30 seconds)
golangci-lint run --timeout 3m
```

### API Documentation
```bash
# Install swagger tool
go install github.com/swaggo/swag/cmd/swag@v1.8.10

# Generate API docs (required before building if REST API changes)
swag init -g schema.go -o api/docs/ -d api/,internal/rest/
```

### Docker
```bash
# Build Docker image
docker build --build-arg build_commit_sha=test --build-arg build_version=test --build-arg build_time=$(date -u +'%Y-%m-%dT%H:%M:%SZ') -t swat4master-test .

# Test Docker image
docker run --rm swat4master-test version
```

## Project Architecture

### Directory Structure
```
├── cmd/swat4master/           # Main application entry point
│   ├── main.go               # Application bootstrap with CLI
│   ├── components/           # Service components (api, browser, prober, etc.)
│   ├── application/          # Dependency injection setup
│   └── container/            # Use case configuration
├── internal/                 # Private application code
│   ├── core/                 # Business logic
│   │   ├── entities/         # Domain entities (server, probe, instance)
│   │   ├── repositories/     # Repository interfaces
│   │   └── usecases/         # Business use cases
│   ├── persistence/redis/    # Redis implementations
│   ├── rest/                 # HTTP API handlers
│   └── prober/               # Server probing logic
├── pkg/                      # Reusable packages
│   ├── gamespy/              # GameSpy protocol implementation
│   ├── http/, tcp/, udp/     # Network server utilities
│   └── slice/, binutils/     # General utilities
├── tests/                    # Integration tests
├── api/                      # API schema and documentation
└── .github/workflows/        # CI/CD pipelines
```

### Key Components
- **API Server** (`run api`): REST API for server browsing
- **Browser Server** (`run browser`): GameSpy browsing protocol handler  
- **Reporter Server** (`run reporter`): Accepts heartbeats from game servers
- **Prober** (`run prober`): Probes servers for details and port discovery
- **Cleaner** (`run cleaner`): Removes stale servers and instances
- **Refresher** (`run refresher`): Refreshes server details periodically
- **Reviver** (`run reviver`): Attempts to revive offline servers

### Dependencies & Configuration
- **Redis**: Required for all persistence operations (default: `redis://localhost:6379`)
- **Ports**: 27900/udp (GameSpy), 28910 (reporter), 3000 (API), 9000 (metrics)
- **Environment**: Set `REDIS_URL` for non-default Redis connection

## CI/CD Validation

The project runs the following checks on every push:

### Test Pipeline
```bash
# Exact commands from CI
go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
```

### Lint Pipeline  
```bash
# Uses golangci-lint v2.1.6 via GitHub Action
golangci-lint run
```

### Build Pipeline
```bash
go mod verify
go build -v -ldflags="..." -o swat4master cmd/swat4master/main.go
./swat4master version
```

### Docker Pipeline
```bash
docker build --load --tags testing --build-args "..." .
docker run --rm testing version
```

## Common Development Patterns

### Adding New Use Cases
1. Create interface in `internal/core/usecases/{name}/`
2. Implement in same package with dependencies injected
3. Add to `cmd/swat4master/container/container.go`
4. Wire in component that needs it

### Adding New Repository Methods
1. Add method to interface in `internal/core/repositories/`
2. Implement in `internal/persistence/redis/repositories/{entity}/`
3. Write tests using `testredis.MakeClient(t)`

### Testing Patterns
- Use `testredis.MakeClient(t)` for Redis-backed tests
- Use factory packages in `internal/testutils/factories/`
- Mock time with `clockwork.NewFakeClock()`
- Test files follow `*_test.go` convention

## Common Issues & Workarounds

### Test Failures
- **Browser test flakiness**: The browser validation test occasionally fails; this is a known issue
- **Redis requirement**: Many tests require Redis; they'll skip if unavailable
- **Timing tests**: Use `clockwork.FakeClock` to avoid time-dependent test failures

### Build Issues
- **API docs**: If REST API changes, regenerate docs with `swag init` before building
- **Go version**: Requires Go 1.22+; the project uses Go 1.23 features
- **CGO**: Disabled in Docker builds (`CGO_ENABLED=0`)

### Development Setup
- **Redis**: Start with `docker run -d -p 6379:6379 redis:alpine` for development
- **Hot reload**: No built-in hot reload; rebuild and restart for changes

## Performance Notes

- **Tests**: Complete test suite runs in ~5-10 seconds
- **Build**: Basic build takes ~5 seconds
- **Lint**: Full lint takes ~30 seconds  
- **Docker build**: Takes ~2-3 minutes for full multi-stage build

## Validation Commands

Always validate changes with these commands before submitting:

```bash
# 1. Verify dependencies and build
go mod verify && go build -o swat4master cmd/swat4master/main.go

# 2. Run tests
go test ./...

# 3. Check formatting and lint
gofmt -d . && golangci-lint run

# 4. Verify binary works
./swat4master --help

# 5. Test Docker if relevant
docker build -t test . && docker run --rm test version
```

## Important Files
- `go.mod` - Go dependencies and version
- `.golangci.yml` - Linter configuration  
- `Dockerfile` - Multi-stage build with API doc generation
- `.github/workflows/ci.yml` - Complete CI pipeline
- `cmd/swat4master/main.go` - Application entry point
- `api/schema.go` - API documentation schema

**Trust these instructions** - they are validated against the actual codebase. Only search for additional information if these instructions are incomplete or appear incorrect.