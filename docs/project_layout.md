# Project Layout

The Hybrid Helm project's scaffolding is customized to be compatible with both Helm and Go APIs.

| File/Directory | Description | 
| ------ | ----- |
| `Dockerfile` | The Dockerfile of your operator project, used to build the image with `make docker-build`. |
| `Makefile` | Build file with helper targets to help you work with your project. |
| `PROJECT` | This file represents the project's configuration and is used to track useful information for the CLI and plugins. |
| `bin/` | This directory contains useful binaries such as the `manager` which is used to run your project locally and  the `kustomize` utility used for the project configuration. |
| `config/` | Contains configuration files to launch your project on a cluster. Plugins might use it to provide functionality. For example, for the CLI  to help create your operator bundle it will look for the CRD's and CR's which are scaffolded in this directory. You will also find all [Kustomize][Kustomize] YAML definitions as well. |
| `config/crd/` | Contains the [Custom Resources Definitions][k8s-crd-doc]. |
| `config/default/` | Contains a [Kustomize base][kustomize-base] for launching the controller in a standard configuration. |
| `config/manager/` | Contains the manifests to launch your operator project as pods on the cluster. |
| `config/manifests/` | Contains the base to generate your OLM manifests in the bundle directory. |
| `config/prometheus/` | Contains the manifests required to enable project to serve metrics to [Prometheus][kb-metrics] such as the `ServiceMonitor` resource. |
| `config/scorecard/` | Contains the manifests required to allow you test your project with [Scorecard][scorecard]. |
| `config/rbac/` | Contains the [RBAC][k8s-rbac] permissions required to run your project. |
| `config/samples/` | Contains the [Custom Resources][k8s-cr-doc]. |
|`api/` | Contains the Go api definition |
|`controllers` |  Contains the controllers for Go API |
| `hack/` | Contains utility files, e.g. the file used to scaffold the license header for your project files. |
|`main.go` | Implements the project initialization |
|`helm-charts` | Contains the Helm charts which can be specified using `create api` command of helm plugin |
|`watches.yaml` | Contains Group, Version, Kind, and Helm chart location. Used to configure the [Helm watches][helm-watches]. |
