module github.com/example/memcached-operator

go 1.16

require (
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.15.0
	github.com/operator-framework/helm-operator-plugins v0.0.8-0.20210831184500-f47861f34e36
	k8s.io/apimachinery v0.22.1
	k8s.io/client-go v0.22.1
	rsc.io/letsencrypt v0.0.3 // indirect
	sigs.k8s.io/controller-runtime v0.10.0
)
