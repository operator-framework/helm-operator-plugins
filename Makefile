# GO_BUILD_ARGS should be set when running 'go build' or 'go install'.
VERSION_PKG = "$(shell go list -m)/internal/version"
SCAFFOLD_VERSION = $(shell git describe --abbrev=0)
GIT_VERSION = $(shell git describe --dirty --tags --always)
GIT_COMMIT = $(shell git rev-parse HEAD)
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
CR_VERSION=$(shell go list -m sigs.k8s.io/controller-runtime | cut -d" " -f2 | sed 's/^v//')
test:
	fetch envtest $(CR_VERSION) && go test -race -covermode atomic -coverprofile cover.out ./...

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

.PHONY: clean
clean:
	rm -rf $(TOOLS_BIN_DIR) $(BUILD_DIR)
