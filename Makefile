# Image URL to use all building/pushing image targets
IMG ?= quay.io/joelanford/helm-operator

SHELL=/bin/bash  # todo(camilamacedo86): why it is required?
# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager

# Run tests
test: generate fmt vet testbin
	# todo(camilamacedo86): why `-race -covermode atomic`?
	go test -race -covermode atomic -coverprofile cover.out ./...

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager main.go

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

lint: golangci-lint
	$(GOLANGCI_LINT) run

# todo(camilamacedo86): do not work
# $ make generate
  #/Users/camilamacedo/go/bin/controller-gen object:headerFile=./hack/boilerplate.go.txt paths="./..."
  #open ./hack/boilerplate.go.txt: no such file or directory
# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile=./hack/boilerplate.go.txt paths="./..."

# Build the docker image
docker-build: test
	docker build . -t ${IMG}

# Push the docker image
docker-push:
	docker push ${IMG}


# todo(camilamacedo86): could we not have it just in the travis?
# IMO: we should have just a target to run the lint with the setup .golangci.yml and we should not download it. Just in the travis.
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


# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.3.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(shell go env GOPATH)/bin/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
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

# $ make testbin
#  mkdir -p testbin
#  [[ -x testbin/etcd ]] || curl -L https://storage.googleapis.com/etcd/v3.4.3/etcd-v3.4.3-darwin-amd64.tar.gz | tar zx -C testbin --strip-components=1 etcd-v3.4.3-darwin-amd64/etcd
#    % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
#                                   Dload  Upload   Total   Spent    Left  Speed
#  100   205  100   205    0     0    510      0 --:--:-- --:--:-- --:--:--   511
#  tar: Unrecognized archive format
#  tar: etcd-v3.4.3-darwin-amd64/etcd: Not found in archive
#  tar: Error exit delayed from previous errors.
#  make: *** [Makefile:118: testbin] Error 1
