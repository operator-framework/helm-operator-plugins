# helm-operator

![Build Status](https://github.com/operator-framework/helm-operator-plugins/workflows/CI/badge.svg?branch=main)
[![Coverage Status](https://coveralls.io/repos/github/operator-framework/helm-operator-plugins/badge.svg?branch=main)](https://coveralls.io/github/operator-framework/helm-operator-plugins?branch=main)

Reimplementation of the helm operator to enrich the Helm operator's reconciliation with custom Go code to create a 
hybrid operator.

## Introduction

The Helm operator type automates Helm chart operations
by mapping the [values](https://helm.sh/docs/chart_template_guide/values_files/) of a Helm chart exactly to a 
`CustomResourceDefinition` and defining its watched resources in a `watches.yaml` 
[configuration](https://sdk.operatorframework.io/docs/building-operators/helm/tutorial/#watch-the-nginx-cr) file.

For creating a [Level II+](https://sdk.operatorframework.io/docs/advanced-topics/operator-capabilities/operator-capabilities/) operator 
that reuses an already existing Helm chart, a [hybrid](https://github.com/operator-framework/operator-sdk/issues/670)
between the Go and Helm operator types is necessary.

The hybrid approach allows adding customizations to the Helm operator, such as:
- value mapping based on cluster state, or 
- executing code in specific events.

## Quick start

### Creating a Helm reconciler

```go
// Operator's main.go
chart, err := loader.Load("path/to/chart")
if err != nil {
 panic(err)
}

reconciler := reconciler.New(
 reconciler.WithChart(*chart),
 reconciler.WithGroupVersionKind(gvk),
)

if err := reconciler.SetupWithManager(mgr); err != nil {
 panic(fmt.Sprintf("unable to create reconciler: %s", err))
}
```

### Creating a Helm reconciler with multiple namespace installation

Add the WATCH_NAMESPACE to the manager files to restrict the namespace to observe where the operator is installed

```json
name: manager
env:
  - name: WATCH_NAMESPACE
        valueFrom:
          fieldRef:
            fieldPath: metadata.namespace
```

Filter the events to the namespace where the operator is installed

```go
// Operator's main.go
watchNamespace = os.Getenv("WATCH_NAMESPACE")

var cacheOpts cache.Options
if watchNamespace != "" {
	setupLog.Info("Watching specific namespace", "namespace", watchNamespace)
	cacheOpts = cache.Options{
		DefaultNamespaces: map[string]cache.Config{
			watchNamespace: {},
		},
	}
} else {
	setupLog.Info("Watching all namespaces")
}

mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
        ...
		Cache:                  cacheOpts,
})

...
chart, err := loader.Load("path/to/chart")
if err != nil {
 panic(err)
}

reconciler := reconciler.New(
 reconciler.WithChart(*chart),
 reconciler.WithGroupVersionKind(gvk),
)

if err := reconciler.SetupWithManager(mgr); err != nil {
 panic(fmt.Sprintf("unable to create reconciler: %s", err))
}
```
