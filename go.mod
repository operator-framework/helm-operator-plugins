module github.com/joelanford/helm-operator

go 1.13

require (
	github.com/go-logr/logr v0.1.0
	github.com/iancoleman/strcase v0.0.0-20191112232945-16388991a334
	github.com/kr/text v0.1.0
	github.com/onsi/ginkgo v1.12.0
	github.com/onsi/gomega v1.9.0
	github.com/prometheus/client_golang v1.5.1
	github.com/sirupsen/logrus v1.5.0
	github.com/spf13/afero v1.2.2
	github.com/spf13/cobra v1.0.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.5.1
	github.com/xenolf/lego v2.7.2+incompatible
	go.uber.org/zap v1.14.1
	golang.org/x/tools v0.0.0-20200403190813-44a64ad78b9b
	gomodules.xyz/jsonpatch/v2 v2.0.1
	helm.sh/helm/v3 v3.2.0
	k8s.io/api v0.18.2
	k8s.io/apiextensions-apiserver v0.18.2
	k8s.io/apimachinery v0.18.2
	k8s.io/cli-runtime v0.18.2
	k8s.io/client-go v0.18.2
	k8s.io/klog v1.0.0
	k8s.io/kubectl v0.18.2
	rsc.io/letsencrypt v0.0.3 // indirect
	sigs.k8s.io/controller-runtime v0.6.0
	sigs.k8s.io/kubebuilder v1.0.9-0.20200618125005-36aa113dbe99
	sigs.k8s.io/yaml v1.2.0
)
