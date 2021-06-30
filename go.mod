module github.com/operator-framework/helm-operator-plugins

go 1.16

require (
	github.com/blang/semver/v4 v4.0.0
	github.com/containerd/containerd v1.4.3 // indirect
	github.com/go-logr/logr v0.3.0
	github.com/iancoleman/strcase v0.1.2
	github.com/kr/text v0.1.0
	github.com/onsi/ginkgo v1.15.0
	github.com/onsi/gomega v1.10.5
	github.com/operator-framework/operator-lib v0.3.0
	github.com/prometheus/client_golang v1.7.1
	github.com/sirupsen/logrus v1.7.0
	github.com/spf13/afero v1.2.2
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.6.1
	gomodules.xyz/jsonpatch/v2 v2.1.0
	helm.sh/helm/v3 v3.5.0
	k8s.io/api v0.20.4
	k8s.io/apiextensions-apiserver v0.20.4
	k8s.io/apimachinery v0.20.4
	k8s.io/cli-runtime v0.20.4
	k8s.io/client-go v0.20.4
	k8s.io/kubectl v0.20.4
	sigs.k8s.io/controller-runtime v0.8.2
	sigs.k8s.io/kubebuilder/v3 v3.1.0
	sigs.k8s.io/yaml v1.2.0
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible
	github.com/docker/distribution => github.com/docker/distribution v0.0.0-20191216044856-a8371794149d
)
