# Helm Operator Plugins

This repository contains the plugins and required APIs for Helm related projects. There are two plugins present here:

1. Traditional Helm Operator Plugin
2. Hybrid Helm Operator plugin

## Motivation behind Hybrid Helm plugin

The traditional [Helm operator][helm_sdk] operator has limited functionality compared to Golang or Ansible operators which have reached [Operator Capability level V][capability_level].  The Hybrid Helm operator enhances the existing Helm operator's abilities through Go APIs. With the hybrid approach operator authors can:

1. Scaffold a Go API in the same project as Helm.
2. Configure the Helm reconciler in `main.go` of the project, through the libraries provided in this repository. 

[helm_sdk]: https://sdk.operatorframework.io/docs/building-operators/helm/
[capability_level]: https://sdk.operatorframework.io/docs/overview/operator-capabilities/