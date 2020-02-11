module github.com/operator-framework/helm-operator

go 1.13

require (
	github.com/DATA-DOG/go-sqlmock v1.4.1 // indirect
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v0.1.0
	github.com/helm/helm-2to3 v0.2.2
	github.com/jmoiron/sqlx v1.2.0 // indirect
	github.com/lib/pq v1.3.0 // indirect
	github.com/rubenv/sql-migrate v0.0.0-20200119084958-8794cecc920c // indirect
	github.com/stretchr/testify v1.4.0
	go.uber.org/zap v1.10.0
	gomodules.xyz/jsonpatch/v2 v2.0.1
	gopkg.in/yaml.v2 v2.2.5
	helm.sh/helm/v3 v3.1.0-rc.2
	k8s.io/api v0.17.2
	k8s.io/apimachinery v0.17.2
	k8s.io/cli-runtime v0.17.2
	k8s.io/client-go v0.17.2
	k8s.io/helm v2.16.1+incompatible
	k8s.io/klog v1.0.0
	k8s.io/kubectl v0.17.2
	sigs.k8s.io/controller-runtime v0.4.0
)

replace (
	// github.com/Azure/go-autorest/autorest has different versions for the Go
	// modules than it does for releases on the repository. Note the correct
	// version when updating.
	github.com/Azure/go-autorest/autorest => github.com/Azure/go-autorest/autorest v0.9.0
	github.com/docker/docker => github.com/moby/moby v0.7.3-0.20190826074503-38ab9da00309

	// Kubernetes imports github.com/miekg/dns at a newer version but it is used
	// by a package Helm does not need. Go modules resolves all packages rather
	// than just those in use (like Glide and dep do). This sets the version
	// to the one oras needs. If oras is updated the version should be updated
	// as well.
	github.com/miekg/dns => github.com/miekg/dns v0.0.0-20181005163659-0d29b283ac0f

	github.com/pkg/errors => github.com/pkg/errors v0.9.1
	gopkg.in/inf.v0 v0.9.1 => github.com/go-inf/inf v0.9.1
	gopkg.in/square/go-jose.v2 v2.3.0 => github.com/square/go-jose v2.3.0+incompatible

	rsc.io/letsencrypt => github.com/dmcgowan/letsencrypt v0.0.0-20160928181947-1847a81d2087
)
