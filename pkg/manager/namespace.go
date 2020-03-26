package manager

import (
	"os"
	"strings"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	watchNamespacesEnvVar = "WATCH_NAMESPACES"
	watchNamespaceEnvVar  = "WATCH_NAMESPACE"
)

func ConfigureWatchNamespaces(options *manager.Options, log logr.Logger) {
	namespaces, found := lookupEnv()
	if found && len(namespaces) != 0 {
		log.Info("watching namespaces", "namespaces", namespaces)
		if len(namespaces) > 1 {
			options.NewCache = cache.MultiNamespacedCacheBuilder(namespaces)
		} else {
			options.Namespace = namespaces[0]
		}
		return
	}
	log.Info("watching all namespaces")
	options.Namespace = v1.NamespaceAll
}

func lookupEnv() ([]string, bool) {

	if watchNamespaces, found := os.LookupEnv(watchNamespacesEnvVar); found {
		return splitNamespaces(watchNamespaces), true
	} else if watchNamespace, found := os.LookupEnv(watchNamespaceEnvVar); found {
		return splitNamespaces(watchNamespace), true
	}
	return nil, false
}

func splitNamespaces(namespaces string) []string {
	list := strings.Split(namespaces, ",")
	out := []string{}
	for _, ns := range list {
		if ns != "" {
			out = append(out, ns)
		}
	}
	return out
}
