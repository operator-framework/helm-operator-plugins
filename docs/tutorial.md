# Hybrid Helm Operator Tutorial

An in-depth walkthrough of building and running a Hybrid Helm based operator.

## Prerequisites
- [Operator SDK][operator-sdk] version v1.16 or higher
   - Refer to [installation guide][sdk_install_guide] on how to install Operator SDK
- User authorized with `cluster-admin` permissions

## Overview
We will create a sample project to let you know how it works and this sample will:

- Create a Memcached Deployment through a helm chart if it doesn't exist
- Ensure that the deployment size is the same as specified by Memcached CR spec
- Create a MemcachedBackup deployment using the Go API

## Create a new project

Use the Operator SDK CLI to create a new memcached-operator project:

```sh
mkdir -p $HOME/github.com/example/memcached-operator
cd $HOME/github.com/example/memcached-operator

# we will use a domain of example.com
# so all API groups will be <group>.example.com
operator-sdk init --plugins=hybrid.helm.sdk.operatorframework.io --project-version="3" --repo=github.com/example/memcached-operator
```

The `init` command generates the RBAC rules in `config/rbac/role.yaml` based on the resources that would be deployed by the chart's default manifests. Be sure to double check that the rules generated in `config/rbac/role.yaml` meet the operator's permission requirements.

**Note:**
This creates a project structure that is compatible with both Helm and Go APIs. To learn more about the project directory structure, see the [project layout][[project_layout]] doc.

Hybrid-Helm has not been tested against webhooks and cert manager. Hence, there is no scaffolding for webhooks and cert manager. Go APIs support Webhooks in general. If users, would like to experiment doing so, they can refer to [Operator SDK](https://sdk.operatorframework.io/docs/building-operators/golang/webhook/) or [Kubebuilder](https://book.kubebuilder.io/reference/webhook-overview.html?highlight=webhook#webhook) documentation.

## Create a new Helm API

Use the following command to create a new Helm API:

```sh
operator-sdk create api --plugins helm.sdk.operatorframework.io/v1 --group cache --version v1alpha1 --kind Memcached
```

This configures the operator project for watching the `Memcached` resource with API version `v1alpha1` and also scaffolds a boilerplate Helm chart. Instead of creating the project from the boilerplate Helm chart scaffolded with SDK, you can also use an existing chart from your local filesystem or remote chart repository. To do so, refer to the steps [here][sdk_existing_chart].

**Note**
For more details and examples for creating Helm API based on existing or new charts, run `operator-sdk create api --plugins helm.sdk.operatorframework.io/v1 --help`

### Customizing the operator logic for Helm API

By default, the scaffolded operator watches `Memcached` resource events as shown in `watches.yaml` and executes Helm releases using the specified chart.

```yaml
# Use the 'create api' subcommand to add watches to this file.
- group: cache.my.domain
  version: v1alpha1
  kind: Memcached
  chart: helm-charts/memcached
#+kubebuilder:scaffold:watch
```

For detailed documentation on customizing the helm operator logic through the chart, refer to the documentation [here][helm_customize_doc].

### Customize Helm reconciler configurations using the APIs provided in the library

One of the drawbacks of existing helm operators in the inability to configure the helm reconciler as it is abstracted from users.  For creating a [Level II+][operator_capabilities] that reuses an already existing Helm chart, a [hybrid][hybrid_issue] between the Go and Helm operator types adds value.

The APIs provided in the [helm-operator-plugins][helm_lib_repo] library, allow users to:

// TODO: Add examples for all
- customize value mapping based on cluster state
- execute code in specific events by configuring reconciler's eventrecorder
- customize reconciler's logger
- setup Install, Upgrade, and Uninstall annotations to enable Helm's actions to be configured based on the annotations found in custom resource watched by the reconciler 
- Configure reconciler to run with Pre and Post Hooks

The above configurations to the reconciler can be done in `main.go`. Example:

```go
// Operator's main.go
// With the help of helpers provided in the library, the reconciler can be
// configured here before starting the controller with this reconciler.
reconciler := reconciler.New(
 reconciler.WithChart(*chart),
 reconciler.WithGroupVersionKind(gvk),
)

if err := reconciler.SetupWithManager(mgr); err != nil {
 panic(fmt.Sprintf("unable to create reconciler: %s", err))
}
```

## Create a new Go API

Use the command below to create a new Custom Resource Definition (CRD) API with group `cache`, version `v1` and kind `MemcachedBackup`. When prompted, you can enter `yes` (or `y`) for creating both resource and controller:

```sh
operator-sdk create api --group=cache --version v1 --kind MemcachedBackup --resource --controller --plugins=go/v3
```

This will scaffold the MemcachedBackup resource API at `api/v1/memcachedbackup_types.go` and the controller at `controllers/memcachedbackup_controller.go`.

### Understanding Kubernetes API

For in-depth understanding and explanation of Kubernetes API and the group-version-kind model, check out the [docs here][sdk_go_api].

### Define the API

To begin, we will represent this Go API by defining the `MemcachedBackup` type which will have a `MemcachedBackupSpec.Size` field to set the quantity of memcached backup instances (CRs) to be deployed, and a `MemcachedBackupStatus.Nodes` field to store a CR's Pod names.

**Node**: The Node field is just to illustrate an example of a Status field.

Define the API for the MemcachedBackup Custom Resource(CR) by modifying the Go type definitions at `api/v1/memcachedbackup_types.go` to have the following spec and status:

```go
// MemcachedBackupSpec defines the desired state of MemcachedBackup
type MemcachedBackupSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	//+kubebuilder:validation:Minimum=0
	// Size is the size of the memcached deployment
	Size int32 `json:"size"`
}

// MemcachedBackupStatus defines the observed state of MemcachedBackup
type MemcachedBackupStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// Nodes are the names of the memcached pods
	Nodes []string `json:"nodes"`
}
```

After modifying the *_types.go file always run the following command to update the generated code for that resource type:

```sh
make generate
```
Once the API is defined with spec/status fields and CRD validation markers, the CRD manifests can be generated and updated with the following command:

```sh
make manifests
```

This makefile target will invoke [`controller-gen`][controller-gen] to generate the CRD manifests at `config/crd/bases/cache.my.domain_memcachedbackups.yaml`.

### Implement the controller

The controller in this example will perform the following actions:
1. Create a MemcachedBackup deployment if it doesn't exist
2. Ensure that the Deployment size is the same as specified by the CR spec
3. Update the MemcachedBackup CR status using the status writer with the names of the CR's pods

A detailed explanation on how to configure the controller to perform the above mentioned actions is present [here][controller_implementation_go].

### What's different in `main.go`?

In addition to scaffolding the initialization and running of [`Manager`][c-r_manager] for go API, the logic for loading `watches.yaml` and configuring the Helm reconciler is now exposed to the users though `main.go`.

```go
...
	for _, w := range ws {
		// Register controller with the factory
		reconcilePeriod := defaultReconcilePeriod
		if w.ReconcilePeriod != nil {
			reconcilePeriod = w.ReconcilePeriod.Duration
		}

		maxConcurrentReconciles := defaultMaxConcurrentReconciles
		if w.MaxConcurrentReconciles != nil {
			maxConcurrentReconciles = *w.MaxConcurrentReconciles
		}

		r, err := reconciler.New(
			reconciler.WithChart(*w.Chart),
			reconciler.WithGroupVersionKind(w.GroupVersionKind),
			reconciler.WithOverrideValues(w.OverrideValues),
			reconciler.SkipDependentWatches(w.WatchDependentResources != nil && !*w.WatchDependentResources),
			reconciler.WithMaxConcurrentReconciles(maxConcurrentReconciles),
			reconciler.WithReconcilePeriod(reconcilePeriod),
			reconciler.WithInstallAnnotations(annotation.DefaultInstallAnnotations...),
			reconciler.WithUpgradeAnnotations(annotation.DefaultUpgradeAnnotations...),
			reconciler.WithUninstallAnnotations(annotation.DefaultUninstallAnnotations...),
		)
...
```
The manager is initialized with both `Helm` and `Go` reconcilers.

```go
...
// Setup manager with Go API
   if err = (&controllers.MemcachedBackupReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MemcachedBackup")
		os.Exit(1)
	}

   ...
// Setup manager with Helm API
	for _, w := range ws {
		
      ...
		if err := r.SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Helm")
			os.Exit(1)
		}
		setupLog.Info("configured watch", "gvk", w.GroupVersionKind, "chartPath", w.ChartPath, "maxConcurrentReconciles", maxConcurrentReconciles, "reconcilePeriod", reconcilePeriod)
	}

// Start the manager
   if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
```
### Specify permissions and generate RBAC manifests

The controller needs certain [RBAC][rbac] permissions to interact with the resources it manages. For Go API, these are specified via [RBAC markers][rbac_marker] like the following:

**Note**:
For the Helm API, the permissions are scaffolded by default in `roles.yaml`. Currently, when Go API is scaffolded, these permissions are overwritten. Hence, please make sure to double check if the permissions defined in `roles.yaml` matches your needs. An example of `role.yaml` for memcached-operator is [here][memcached_sample_rbac]. An issue related to this is being tracked [here][rbac_bug]. 

## Run the operator

There are two ways to run the operator:

- As a Go program outside a cluster
- As a Deployment inside a Kubernetes cluster

1. Run locally outside the cluster

The following steps will show how to deploy the operator on the Cluster. However, to run locally for development purposes and outside of a Cluster use the target `make install run`.

2. Run as a Deployment inside the cluster

By default, a new namespace is created with name <project-name>-system, ex. memcached-operator-system, and will be used for the deployment.

Run the following to deploy the operator. This will also install the RBAC manifests from `config/rbac`.

```
make deploy
```

Verify that the `memcached-operator` is up and running:

```sh
$ kubectl get deployment -n memcached-operator-system
NAME                                    READY   UP-TO-DATE   AVAILABLE   AGE
memcached-operator-controller-manager   1/1     1            1           22m
```

### Create a Memcached CR

Update the sample Memcached CR manifest at `config/samples/cache_v1alpha1_memcached.yaml` and define the spec as the following. Here, we will update the `replicaCount` to be `3` :

```yaml
apiVersion: cache.my.domain/v1alpha1
kind: Memcached
metadata:
  name: memcached-sample
spec:
  # Default values copied from <project_dir>/helm-charts/memcached/values.yaml
  affinity: {}
  autoscaling:
    enabled: false
    maxReplicas: 100
    minReplicas: 1
    targetCPUUtilizationPercentage: 80
  fullnameOverride: ""
  image:
    pullPolicy: IfNotPresent
    repository: nginx
    tag: ""
  imagePullSecrets: []
  ingress:
    annotations: {}
    className: ""
    enabled: false
    hosts:
    - host: chart-example.local
      paths:
      - path: /
        pathType: ImplementationSpecific
    tls: []
  nameOverride: ""
  nodeSelector: {}
  podAnnotations: {}
  podSecurityContext: {}
  replicaCount: 3
  resources: {}
  securityContext: {}
  service:
    port: 80
    type: ClusterIP
  serviceAccount:
    annotations: {}
    create: true
    name: ""
  tolerations: []
```

Create the CR:

```
kubectl apply -f config/samples/cache_v1alpha1_memcached.yaml
```

Ensure that the memcached operator creates the deployment for the sample CR with the correct size:

```sh
$ kubectl get pods
NAME                                  READY     STATUS    RESTARTS   AGE
memcached-sample-6fd7c98d8-7dqdr      1/1       Running   0          18m
memcached-sample-6fd7c98d8-g5k7v      1/1       Running   0          18m
memcached-sample-6fd7c98d8-m7vn7      1/1       Running   0          18m
```

### Create a MemcachedBackup CR

Update the sample Memcached CR manifest at `config/samples/cache_v1_memcachedbackup.yaml` and define the spec as the following. Here, we will update the `size` to be `2` :

```yaml
apiVersion: cache.my.domain/v1
kind: MemcachedBackup
metadata:
  name: memcachedbackup-sample
spec:
  size: 2
```

Create the CR:

```
kubectl apply -f config/samples/cache_v1_memcachedbackup.yaml
```

Ensure that the count of memcachedbackup pods are the same as specified in the CR:

```sh
$ kubectl get pods
NAME                                        READY     STATUS    RESTARTS   AGE
memcachedbackup-sample-8649699989-4bbzg     1/1       Running   0          22m
memcachedbackup-sample-8649699989-mq6mx     1/1       Running   0          22m
```

You can update the `spec` in each of the above CRs and apply then again. The controller will reconcile again and ensure that the size of the pods is as specified in the `spec` of the respective CRs.

## Cleanup

Run the following to delete all deployed resources:

```
kubectl delete -f config/samples/cache_v1alpha1_memcached.yaml
kubectl delete -f config/samples/cache_v1_memcachedbackup.yaml
cache_v1_memcachedbackup.yaml
make undeploy
```


[operator-sdk]: https://github.com/operator-framework/operator-sdk
[sdk_install_guide]: https://sdk.operatorframework.io/docs/building-operators/helm/installation/
[sdk_existing_chart]: https://sdk.operatorframework.io/docs/building-operators/helm/tutorial/#use-an-existing-chart
[helm_customize_doc]: https://sdk.operatorframework.io/docs/building-operators/helm/tutorial/#customize-the-operator-logic
[operator_capabilities]: https://operatorframework.io/operator-capabilities/
[hybrid_issue]: https://github.com/operator-framework/operator-sdk/issues/670
[helm_lib_repo]: https://github.com/operator-framework/helm-operator-plugins
[sdk_go_api]: https://sdk.operatorframework.io/docs/building-operators/golang/tutorial/#understanding-kubernetes-apis
[controller-gen]: https://github.com/kubernetes-sigs/controller-tools
[controller_implementation_go]: https://sdk.operatorframework.io/docs/building-operators/golang/tutorial/#implement-the-controller
[rbac]: https://kubernetes.io/docs/reference/access-authn-authz/rbac/
[rbac_marker]: https://book.kubebuilder.io/reference/markers/rbac.html
[rbac_bug]: https://github.com/operator-framework/helm-operator-plugins/issues/142
[project_layout]: docs/project_layout.md
[c-r_manager]: https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/manager#Manager
[memcached_sample_rbac]: https://github.com/varshaprasad96/hybrid-memcached-example/blob/main/config/rbac/role.yaml
