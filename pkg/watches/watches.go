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

	"gopkg.in/yaml.v2" // todo: remplace for the k8s lib "sigs.k8s.io/yaml"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Watch struct {
	GroupVersionKind schema.GroupVersionKind `yaml:",inline"` // why use yaml instead of json?
	ChartPath        string                  `yaml:"chart"`

	WatchDependentResources *bool             `yaml:"watchDependentResources,omitempty"`
	OverrideValues          map[string]string `yaml:"overrideValues,omitempty"`

	// the following ones were not in the watch before. However, i agree that it is better here
	// also, it should not cause changes in its behaviour as well.
	ReconcilePeriod         *time.Duration `yaml:"reconcilePeriod,omitempty"`
	MaxConcurrentReconciles *int           `yaml:"maxConcurrentReconciles,omitempty"`

	Chart *chart.Chart `yaml:"-"`
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
		if w.WatchDependentResources == nil {
			trueVal := true
			w.WatchDependentResources = &trueVal
		}

		if _, ok := watchesMap[w.GroupVersionKind]; ok {
			return nil, fmt.Errorf("duplicate GVK: %s", w.GroupVersionKind)
		}

		watchesMap[w.GroupVersionKind] = w
		watches[i] = w
	}
	return watches, nil
}

func expandOverrideEnvs(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
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

// the template is not here. Should we not provide the template as well for SDK consume it?
// Should not be part of the lib provide its specific templates as well?
// This is a point that in my mind has 2 options;

// A) scaffold also be in the libs and libs provide a plugin
// which means e.g SDK lib = go sdk plugin
// then, e2e tests for each in the lib repo
// sdk cli with just commands and calling plugins
// e.g helm lib is the helm plugin with its testdata and a project scaffolded
// e.g helm lib/plugin will use sdk lib to do the base
// we will able to develop and test the changes standalone

// B) scaffolds be part of sdk tool
// then libs has just the unit test
// we will need to bump the new lib version to do the full test and check the changes
// the e2e test are in sdk cli project

// IMO shows that the option A will make easier work with and solve the CI problems
// and keep better the cohesion.
