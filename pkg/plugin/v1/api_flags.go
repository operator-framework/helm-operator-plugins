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

package v1

import (
	"fmt"
	"strings"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/kubebuilder/pkg/model/resource"

	"github.com/joelanford/helm-operator/pkg/plugin/internal/chartutil"
)

const (
	group             = "group"
	version           = "version"
	kind              = "kind"
	helmChart         = "helm-chart"
	helmChartRepo     = "helm-chart-repo"
	helmChartVerison  = "helm-chart-version"
	crdVersion        = "crd-version"
	crdVersionV1      = "v1"
	crdVersionV1beta1 = "v1beta1"
)

type APIFlags struct {
	createOptions chartutil.CreateOptions

	gvk schema.GroupVersionKind
}

// AddTo will add the API flags
func (f *APIFlags) AddTo(fs *pflag.FlagSet) {
	fs.StringVar(&f.createOptions.GVK.Group, group, "", "Kubernetes resource Kind Group. (e.g app)")
	fs.StringVar(&f.createOptions.GVK.Version, version, "", "Kubernetes resource Version. (e.g v1alpha1)")
	fs.StringVar(&f.createOptions.GVK.Kind, kind, "", "Kubernetes resource Kind name. (e.g AppService)")

	fs.StringVar(&f.createOptions.Chart, helmChart, "", "helm chart (<URL>, <repo>/<name>, or local path)")
	fs.StringVar(&f.createOptions.Repo, helmChartRepo, "", "Chart repository URL for the requested helm char")
	fs.StringVar(&f.createOptions.Version, helmChartVerison, "", "Specific version of the helm chart (default: latest)")

	fs.StringVar(&f.createOptions.CRDVersion, crdVersion, crdVersionV1, "CRD version to generate")
}

// Validate will verify the helm API flags
func (f *APIFlags) Validate() error {
	if f.createOptions.CRDVersion != "v1" && f.createOptions.CRDVersion != "v1beta1" {
		return fmt.Errorf("value of --%s must be either %q or %q", crdVersion, crdVersionV1, crdVersionV1beta1)
	}

	if len(strings.TrimSpace(f.createOptions.Chart)) == 0 {
		if len(strings.TrimSpace(f.createOptions.Repo)) != 0 {
			return fmt.Errorf("value of --%s can only be used with --%s", helmChartRepo, helmChart)
		} else if len(f.createOptions.Version) != 0 {
			return fmt.Errorf("value of --%s can only be used with --%s", helmChartVerison, helmChart)
		}
	}

	if len(strings.TrimSpace(f.createOptions.Chart)) == 0 {
		if len(strings.TrimSpace(f.createOptions.GVK.Group)) == 0 {
			return fmt.Errorf("value of --%s must not have empty value", group)
		}
		if len(strings.TrimSpace(f.createOptions.GVK.Version)) == 0 {
			return fmt.Errorf("value of --%s must not have empty value", version)
		}
		if len(strings.TrimSpace(f.createOptions.GVK.Kind)) == 0 {
			return fmt.Errorf("value of --%s must not have empty value", kind)
		}

		// Validate the resource.
		r := resource.Options{
			Namespaced: true,
			Group:      f.createOptions.GVK.Group,
			Version:    f.createOptions.GVK.Version,
			Kind:       f.createOptions.GVK.Kind,
		}
		if err := r.Validate(); err != nil {
			return err
		}
	}
	return nil
}
