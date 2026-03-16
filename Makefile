PKG := github.com/sergeii/swat4master/cmd/swat4master

BUILD_TIME ?= -
BUILD_COMMIT ?= -
BUILD_VERSION ?= -

OUTPUT ?= swat4master
GOOS ?=
GOARCH ?=

LDFLAGS := \
	-X '$(PKG)/build.Time=$(BUILD_TIME)' \
	-X '$(PKG)/build.Commit=$(BUILD_COMMIT)' \
	-X '$(PKG)/build.Version=$(BUILD_VERSION)'

SWAG := github.com/swaggo/swag/cmd/swag@v1.16.6

.PHONY: generate-api-docs
generate-api-docs:
	go run $(SWAG) init \
		-g schema.go \
		-o api/docs/ \
		-d api/,internal/rest/

.PHONY: build
build:
	CGO_ENABLED=0 GOEXPERIMENT=loopvar \
		GOOS=$(GOOS) GOARCH=$(GOARCH) \
		go build -v \
		-ldflags="$(LDFLAGS)" \
		-o $(OUTPUT) \
		./cmd/swat4master
