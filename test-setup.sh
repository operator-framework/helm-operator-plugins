#!/usr/bin/env bash

# */
# Copyright 2020 The Operator-SDK Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
# */

set -eu

# */
# To use envtest is required etcd, kube-apiserver and kubetcl binaries in the testbin directory.
# This sript will perform this setup for linux or mac os x envs.
# */

K8S_VER=v1.18.2
ETCD_VER=v3.4.3
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m | sed 's/x86_64/amd64/')
ETCD_EXT="tar.gz"

rm -rf testbin
mkdir -p testbin

# install etcd bin
# the extension for linux env is not equals for mac os x
if [ $OS == "darwin" ]; then
   ETCD_EXT="zip"
fi
[[ -x testbin/etcd ]] || curl -L https://storage.googleapis.com/etcd/${ETCD_VER}/etcd-${ETCD_VER}-${OS}-${ARCH}.${ETCD_EXT} | tar zx -C testbin --strip-components=1 etcd-${ETCD_VER}-${OS}-${ARCH}/etcd

# install kube-apiserver and kubetcl bin
if [ $OS == "darwin" ]
then
  # kubernetes do not provide the kubernetes-server for darwin, so to have the kube-apiserver is rquired to build it locally
  # if the project is already cloned locally do nothing
  if [ ! -d $GOPATH/src/k8s.io/kubernetes ]; then
    git clone https://github.com/kubernetes/kubernetes $GOPATH/src/k8s.io/kubernetes --depth=1 -b v1.18.2
  fi

  # if the kube-apiserve is alredy built just copy
  if [ ! -f $GOPATH/src/k8s.io/kubernetes/_output/local/bin/darwin/amd64/kube-apiserver ]; then
    DIR=$(pwd)
    cd $GOPATH/src/k8s.io/kubernetes
    # Build for linux first otherwise it won't work for darwin - :(
    export KUBE_BUILD_PLATFORMS="linux/amd64"
    make WHAT=cmd/kube-apiserver
    export KUBE_BUILD_PLATFORMS="darwin/amd64"
    make WHAT=cmd/kube-apiserver
    cd ${DIR}
  fi
  cp $GOPATH/src/k8s.io/kubernetes/_output/local/bin/darwin/amd64/kube-apiserver testbin/

  # now let's get the kubectl
  curl -LO https://storage.googleapis.com/kubernetes-release/release/${K8S_VER}/bin/darwin/amd64/kubectl
  chmod +x kubectl
  mv kubectl testbin/
else
  [[ -x testbin/kube-apiserver && -x testbin/kubectl ]] || curl -L https://dl.k8s.io/${K8S_VER}/kubernetes-server-${OS}-${ARCH}.tar.gz | tar zx -C testbin --strip-components=3 kubernetes/server/bin/kube-apiserver kubernetes/server/bin/kubectl
fi
