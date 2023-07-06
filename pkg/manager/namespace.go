/*
Copyright 2020 The Operator-SDK Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package manager

import (
	"os"
	"strings"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	WatchNamespaceEnvVar = "WATCH_NAMESPACE"
)

func ConfigureWatchNamespaces(options *manager.Options, log logr.Logger) {
	namespaces := lookupEnv()
	var watchNamespaces []string
	if len(namespaces) != 0 {
		log.Info("watching namespaces", "namespaces", namespaces)
		watchNamespaces = namespaces
	} else {
		log.Info("watching all namespaces")
		watchNamespaces = []string{v1.NamespaceAll}
	}

	options.NewCache = func(config *rest.Config, opts cache.Options) (cache.Cache, error) {
		return cache.New(config, cache.Options{
			Namespaces: watchNamespaces,
		})
	}
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
