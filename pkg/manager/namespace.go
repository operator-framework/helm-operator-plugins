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
	WatchNamespaceEnvVar = "WATCH_NAMESPACE"
)

func ConfigureWatchNamespaces(options *manager.Options, log logr.Logger) {
	namespaces := lookupEnv()
	if len(namespaces) != 0 {
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

func lookupEnv() []string {
	if watchNamespace, found := os.LookupEnv(WatchNamespaceEnvVar); found {
		return splitNamespaces(watchNamespace)
	}
	return nil
}

func splitNamespaces(namespaces string) []string {
	list := strings.Split(namespaces, ",")
	out := []string{}
	for _, ns := range list {
		if ns != "" {
			out = append(out, strings.TrimSpace(ns))
		}
	}
	return out
}
