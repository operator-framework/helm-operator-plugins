
# Image URL to use all building/pushing image targets
IMG ?= quay.io/joelanford/helm-operator

SHELL=/bin/bash
# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# GO_BUILD_ARGS should be set when running 'go build' or 'go install'.
VERSION_PKG = "$(shell go list -m)/internal/version"
SCAFFOLD_VERSION = $(shell git describe --abbrev=0)
GIT_VERSION = $(shell git describe --dirty --tags --always)
GIT_COMMIT = $(shell git rev-parse HEAD)
GO_BUILD_ARGS = \
  -gcflags "all=-trimpath=$(shell dirname $(shell pwd))" \
  -asmflags "all=-trimpath=$(shell dirname $(shell pwd))" \
  -ldflags " \
    -X '$(VERSION_PKG).ScaffoldVersion=$(SCAFFOLD_VERSION)' \
    -X '$(VERSION_PKG).GitVersion=$(GIT_VERSION)' \
    -X '$(VERSION_PKG).GitCommit=$(GIT_COMMIT)' \
  " \

export GO111MODULE = on

# Run tests
ENVTEST_ASSETS_DIR=$(shell pwd)/testbin
test: fmt vet
	mkdir -p ${ENVTEST_ASSETS_DIR}
	test -f ${ENVTEST_ASSETS_DIR}/setup-envtest.sh || curl -sSLo ${ENVTEST_ASSETS_DIR}/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/master/hack/setup-envtest.sh
	source ${ENVTEST_ASSETS_DIR}/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); go test -race -covermode atomic -coverprofile cover.out ./...

# Build manager binary
build: fmt vet
	CGO_ENABLED=0 go build $(GO_BUILD_ARGS) -o bin/helm-operator main.go

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

lint: golangci-lint
	$(GOLANGCI_LINT) run
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	$(GOLANGCI_LINT) run --fix

# find or download controller-gen
# download controller-gen if necessary
golangci-lint:
ifeq (, $(shell which golangci-lint))
	@{ \
	set -e ;\
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v1.27.0 ;\
	}
GOLANGCI_LINT=$(shell go env GOPATH)/bin/golangci-lint
else
GOLANGCI_LINT=$(shell which golangci-lint)
endif
