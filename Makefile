# GO_BUILD_ARGS should be set when running 'go build' or 'go install'.
VERSION_PKG = "$(shell go list -m)/internal/version"
export SCAFFOLD_VERSION = $(shell git describe --tags --abbrev=0)
export GIT_VERSION = $(shell git describe --dirty --tags --always)
export GIT_COMMIT = $(shell git rev-parse HEAD)
BUILD_DIR = $(PWD)/bin
GO_BUILD_ARGS = \
  -gcflags "all=-trimpath=$(shell dirname $(shell pwd))" \
  -asmflags "all=-trimpath=$(shell dirname $(shell pwd))" \
  -ldflags " \
    -s \
    -w \
    -X '$(VERSION_PKG).ScaffoldVersion=$(SCAFFOLD_VERSION)' \
    -X '$(VERSION_PKG).GitVersion=$(GIT_VERSION)' \
    -X '$(VERSION_PKG).GitCommit=$(GIT_COMMIT)' \
  " \

# Always use Go modules
export GO111MODULE = on

# Setup project-local paths and build settings
SHELL=/bin/bash
TOOLS_DIR=$(PWD)/tools
TOOLS_BIN_DIR=$(TOOLS_DIR)/bin
SCRIPTS_DIR=$(TOOLS_DIR)/scripts
export PATH := $(BUILD_DIR):$(TOOLS_BIN_DIR):$(SCRIPTS_DIR):$(PATH)

.PHONY: all
all: test lint build

# Run tests
.PHONY: test
export KUBEBUILDER_ASSETS := $(TOOLS_BIN_DIR)
TESTPKG ?= ./...
# TODO: Modify this to use setup-envtest binary
test:
	fetch envtest 0.8.3 && go test -race -covermode atomic -coverprofile cover.out $(TESTPKG)

# Build manager binary
.PHONY: build
build:
	CGO_ENABLED=0 mkdir -p $(BUILD_DIR) && go build $(GO_BUILD_ARGS) -o $(BUILD_DIR) ./

# Run go fmt and go mod tidy, and check for clean git tree
.PHONY: fix
fix:
	go mod tidy
	go fmt ./...
	git diff --exit-code

# Run various checks against code
.PHONY: lint
lint:
	fetch golangci-lint 1.35.2 && golangci-lint run

.PHONY: release
release: GORELEASER_ARGS ?= --snapshot --rm-dist --skip-sign
release:
	fetch goreleaser 0.156.2 && goreleaser $(GORELEASER_ARGS)

.PHONY: clean
clean:
	rm -rf $(TOOLS_BIN_DIR) $(BUILD_DIR)
