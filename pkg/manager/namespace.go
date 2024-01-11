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
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	WatchNamespaceEnvVar = "WATCH_NAMESPACE"
)

func ConfigureWatchNamespaces(options *manager.Options, log logr.Logger) {
	namespaces := splitNamespaces(os.Getenv(WatchNamespaceEnvVar))

	var namespaceConfigs map[string]cache.Config
	if len(namespaces) != 0 {
		log.Info("watching namespaces", "namespaces", namespaces)
		namespaceConfigs = make(map[string]cache.Config)
		for _, namespace := range namespaces {
			namespaceConfigs[namespace] = cache.Config{}
		}
	} else {
		log.Info("watching all namespaces")
	}

	options.Cache.DefaultNamespaces = namespaceConfigs
}

func splitNamespaces(namespaces string) []string {
	list := strings.Split(namespaces, ",")
	var out []string
	for _, ns := range list {
		trimmed := strings.TrimSpace(ns)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
