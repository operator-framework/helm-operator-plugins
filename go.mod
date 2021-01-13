module github.com/joelanford/helm-operator

go 1.15

require (
	github.com/go-logr/logr v0.3.0
	github.com/iancoleman/strcase v0.1.2
	github.com/kr/text v0.1.0
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/operator-framework/operator-lib v0.3.0
	github.com/sirupsen/logrus v1.7.0
	github.com/spf13/afero v1.2.2
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.6.1
	golang.org/x/tools v0.0.0-20200616195046-dc31b401abb5
	gomodules.xyz/jsonpatch/v2 v2.1.0
	helm.sh/helm/v3 v3.4.1
	k8s.io/api v0.20.2
	k8s.io/apiextensions-apiserver v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/cli-runtime v0.20.2
	k8s.io/client-go v0.20.2
	k8s.io/kubectl v0.20.2
	rsc.io/letsencrypt v0.0.3 // indirect
	sigs.k8s.io/controller-runtime v0.8.0
	sigs.k8s.io/kubebuilder/v2 v2.3.2-0.20201214213149-0a807f4e9428
	sigs.k8s.io/yaml v1.2.0
)
