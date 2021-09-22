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

##@ Development

.PHONY: generate
generate: build # Generate CLI docs and samples
	rm -rf testdata/
	go run ./hack/generate/samples/generate_testdata.go
	go generate ./...

.PHONY: all
all: test lint build

# Run tests
.PHONY: test
# Use envtest based on the version of kubernetes/client-go configured in the go.mod file.
# If this version of envtest is not available yet, submit a PR similar to
# https://github.com/kubernetes-sigs/kubebuilder/pull/2287 targeting the kubebuilder
# "tools-releases" branch. Make sure to look up the appropriate etcd version in the
# kubernetes release notes for the minor version you're building tools for.
ENVTEST_VERSION = $(shell go list -m k8s.io/client-go | cut -d" " -f2 | sed 's/^v0\.\([[:digit:]]\+\)\.[[:digit:]]\+$$/1.\1.x/')
TESTPKG ?= ./...
# TODO: Modify this to use setup-envtest binary
test: build
	go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
	source <(setup-envtest use -p env $(ENVTEST_VERSION)) && go test -race -covermode atomic -coverprofile cover.out $(TESTPKG)

.PHONY: test-sanity
test-sanity: generate ## Test repo formatting, linting, etc.
	git diff --exit-code # fast-fail if generate or fix produced changes
	go vet ./...
	$(SCRIPTS_DIR)/fetch golangci-lint 1.31.0 && $(TOOLS_BIN_DIR)/golangci-lint run
	git diff --exit-code # diff again to ensure other checks don't change repo

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
	fetch goreleaser 0.177.0 && goreleaser $(GORELEASER_ARGS)

.PHONY: clean
clean:
	rm -rf $(TOOLS_BIN_DIR) $(BUILD_DIR)
