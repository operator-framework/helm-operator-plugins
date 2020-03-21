// Copyright 2020 The Operator-SDK Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package watches

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Watch struct {
	GroupVersionKind schema.GroupVersionKind `yaml:",inline"`
	ChartPath        string                  `yaml:"chart"`
	Chart            *chart.Chart            `yaml:"-"`

	WatchDependentResources bool              `yaml:"watchDependentResources"`
	OverrideValues          map[string]string `yaml:"overrideValues"`
	ReconcilePeriod         *time.Duration    `yaml:"reconcilePeriod"`
	MaxConcurrentReconciles *int              `yaml:"maxConcurrentReconciles"`
}

func (w *Watch) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// by default, the operator will watch dependent resources
	w.WatchDependentResources = true

	// hide watch data in plain struct to prevent unmarshal from calling
	// UnmarshalYAML again
	type plain Watch

	return unmarshal((*plain)(w))
}

// Load loads a slice of Watches from the watch file at `path`. For each entry
// in the watches file, it verifies the configuration. If an error is
// encountered loading the file or verifying the configuration, it will be
// returned.
func Load(path string) ([]Watch, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	watches := []Watch{}
	err = yaml.Unmarshal(b, &watches)
	if err != nil {
		return nil, err
	}

	watchesMap := make(map[schema.GroupVersionKind]Watch)
	for i, w := range watches {
		if err := verifyGVK(w.GroupVersionKind); err != nil {
			return nil, fmt.Errorf("invalid GVK: %s: %w", w.GroupVersionKind, err)
		}

		cl, err := loader.Load(w.ChartPath)
		if err != nil {
			return nil, fmt.Errorf("invalid chart %s: %w", w.ChartPath, err)
		}
		w.Chart = cl
		w.OverrideValues = expandOverrideEnvs(w.OverrideValues)

		if _, ok := watchesMap[w.GroupVersionKind]; ok {
			return nil, fmt.Errorf("duplicate GVK: %s", w.GroupVersionKind)
		}

		watchesMap[w.GroupVersionKind] = w
		watches[i] = w
	}
	return watches, nil
}

func expandOverrideEnvs(in map[string]string) map[string]string {
	out := make(map[string]string)
	for k, v := range in {
		out[k] = os.ExpandEnv(v)
	}
	return out
}

func verifyGVK(gvk schema.GroupVersionKind) error {
	// A GVK without a group is valid. Certain scenarios may cause a GVK
	// without a group to fail in other ways later in the initialization
	// process.
	if gvk.Version == "" {
		return errors.New("version must not be empty")
	}
	if gvk.Kind == "" {
		return errors.New("kind must not be empty")
	}
	return nil
}
