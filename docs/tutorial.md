# Hybrid Helm Operator Tutorial

An in-depth walkthrough of building and running a hybrid helm based operator.

## Prerequisites
- [Operator SDK][operator-sdk] version v1.16 or higher
   - Refer to [installation guide][sdk_install_guide] on how to install Operator SDK
- User authorized with `cluster-admin` permissions

## Overview
We will create a sample project to let you know how it works and this sample will:

- Create a Memcached Deployment through helm chart if it doesn't exist
- Ensure that the deployment size is the same as specified by Memcached CR spec
- Create a MemcachedBackup deployment through Go API

## Create a new project

Use the Operator SDK cli to create a new memcached-operator project:

```sh
mkdir -p $HOME/github.com/example/memcached-operator
cd $HOME/github.com/example/memcached-operator

# we will use a domain of example.com
# so all API groups will be <group>.example.com
operator-sdk init --plugins=hybrid.helm.sdk.operatorframework.io --project-version="3" --repo=github.com/example/memcached-operator
```



[operator-sdk]: https://github.com/operator-framework/operator-sdk
[sdk_install_guide]: https://sdk.operatorframework.io/docs/building-operators/helm/installation/