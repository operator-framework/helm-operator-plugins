module github.com/operator-framework/helm-operator-plugins

go 1.16

require (
	github.com/blang/semver/v4 v4.0.0
	github.com/go-logr/logr v1.2.0
	github.com/go-task/slim-sprig v0.0.0-20210107165309-348f09dbbbc0
	github.com/iancoleman/strcase v0.1.2
	github.com/kr/text v0.2.0
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.17.0
	github.com/operator-framework/operator-lib v0.3.0
	github.com/prometheus/client_golang v1.11.0
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/afero v1.6.0
	github.com/spf13/cobra v1.2.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	gomodules.xyz/jsonpatch/v2 v2.2.0
	helm.sh/helm/v3 v3.6.2
	k8s.io/api v0.23.1
	k8s.io/apiextensions-apiserver v0.23.1
	k8s.io/apimachinery v0.23.1
	k8s.io/cli-runtime v0.23.1
	k8s.io/client-go v0.23.1
	k8s.io/kubectl v0.23.1
	sigs.k8s.io/controller-runtime v0.11.0
	sigs.k8s.io/kubebuilder/v3 v3.3.0
	sigs.k8s.io/yaml v1.3.0
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible
	github.com/docker/distribution => github.com/docker/distribution v0.0.0-20191216044856-a8371794149d
)
