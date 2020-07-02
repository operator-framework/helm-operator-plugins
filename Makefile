
# Image URL to use all building/pushing image targets
IMG ?= quay.io/joelanford/helm-operator

SHELL=/bin/bash
# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager

# Run tests
test: fmt vet testbin
	go test -race -covermode atomic -coverprofile cover.out ./...

# Build manager binary
build: fmt vet
	go build -o bin/helm-operator main.go

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

lint: golangci-lint
	$(GOLANGCI_LINT) run

# Build the docker image
docker-build: test
	docker build . -t ${IMG}

# Push the docker image
docker-push:
	docker push ${IMG}

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

K8S_VER ?= v1.18.2
ETCD_VER ?= v3.4.3
OS=$(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(shell uname -m | sed 's/x86_64/amd64/')

.PHONY: testbin
testbin:
	mkdir -p testbin
	[[ -x testbin/etcd ]] || curl -L https://storage.googleapis.com/etcd/${ETCD_VER}/etcd-${ETCD_VER}-${OS}-${ARCH}.tar.gz | tar zx -C testbin --strip-components=1 etcd-${ETCD_VER}-${OS}-${ARCH}/etcd
	[[ -x testbin/kube-apiserver && -x testbin/kubectl ]] || curl -L https://dl.k8s.io/${K8S_VER}/kubernetes-server-${OS}-${ARCH}.tar.gz | tar zx -C testbin --strip-components=3 kubernetes/server/bin/kube-apiserver kubernetes/server/bin/kubectl
