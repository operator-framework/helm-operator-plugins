/*
Copyright 2020 The Kubernetes Authors.

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

package v1

import (
	"fmt"
	"strings"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/kubebuilder/pkg/model/config"
	"sigs.k8s.io/kubebuilder/pkg/model/resource"
	"sigs.k8s.io/kubebuilder/pkg/plugin"
	"sigs.k8s.io/kubebuilder/pkg/plugin/scaffold"

	"github.com/joelanford/helm-operator/pkg/internal/kubebuilder/cmdutil"
	"github.com/joelanford/helm-operator/pkg/plugin/internal/chartutil"
	"github.com/joelanford/helm-operator/pkg/plugin/v1/scaffolds"
)

type createAPIPlugin struct {
	config *config.Config

	createOptions chartutil.CreateOptions
	gvk           schema.GroupVersionKind
}

var (
	_ plugin.CreateAPI   = &createAPIPlugin{}
	_ cmdutil.RunOptions = &createAPIPlugin{}
)

func (p createAPIPlugin) UpdateContext(ctx *plugin.Context) {
	ctx.Description = `Scaffold a Kubernetes API that is backed by a Helm chart.
`
	ctx.Examples = fmt.Sprintf(`  $ %s create api \
      --group=apps --version=v1alpha1 \
      --kind=AppService

  $ %s create api \
      --group=apps --version=v1alpha1 \
      --kind=AppService \
      --helm-chart=myrepo/app

  $ %s create api \
      --helm-chart=myrepo/app

  $ %s create api \
      --helm-chart=myrepo/app \
      --helm-chart-version=1.2.3

  $ %s create api \
      --helm-chart=app \
      --helm-chart-repo=https://charts.mycompany.com/

  $ %s create api \
      --helm-chart=app \
      --helm-chart-repo=https://charts.mycompany.com/ \
      --helm-chart-version=1.2.3

  $ %s create api \
      --helm-chart=/path/to/local/chart-directories/app/

  $ %s create api \
      --helm-chart=/path/to/local/chart-archives/app-1.2.3.tgz
`,
		ctx.CommandName,
		ctx.CommandName,
		ctx.CommandName,
		ctx.CommandName,
		ctx.CommandName,
		ctx.CommandName,
		ctx.CommandName,
		ctx.CommandName,
	)
}

const (
	group            = "group"
	version          = "version"
	kind             = "kind"
	helmChart        = "helm-chart"
	helmChartRepo    = "helm-chart-repo"
	helmChartVerison = "helm-chart-version"

	crdVersion        = "crd-version"
	crdVersionV1      = "v1"
	crdVersionV1beta1 = "v1beta1"
)

func (p *createAPIPlugin) BindFlags(fs *pflag.FlagSet) {
	p.createOptions = chartutil.CreateOptions{}
	fs.SortFlags = false

	fs.StringVar(&p.createOptions.GVK.Group, group, "", "resource Group")
	fs.StringVar(&p.createOptions.GVK.Version, version, "", "resource Version")
	fs.StringVar(&p.createOptions.GVK.Kind, kind, "", "resource Kind")

	fs.StringVar(&p.createOptions.Chart, helmChart, "", "helm chart")
	fs.StringVar(&p.createOptions.Repo, helmChartRepo, "", "helm chart repository")
	fs.StringVar(&p.createOptions.Version, helmChartVerison, "", "helm chart version (default: latest)")

	fs.StringVar(&p.createOptions.CRDVersion, crdVersion, crdVersionV1, "crd version to generate")
}

func (p *createAPIPlugin) InjectConfig(c *config.Config) {
	p.config = c
}

func (p *createAPIPlugin) Run() error {
	return cmdutil.Run(p)
}

func (p *createAPIPlugin) Validate() error {
	if p.createOptions.CRDVersion != "v1" && p.createOptions.CRDVersion != "v1beta1" {
		return fmt.Errorf("value of --%s must be either %q or %q", crdVersion, crdVersionV1, crdVersionV1beta1)
	}

	if len(strings.TrimSpace(p.createOptions.Chart)) == 0 {
		if len(strings.TrimSpace(p.createOptions.Repo)) != 0 {
			return fmt.Errorf("value of --%s can only be used with --%s", helmChartRepo, helmChart)
		} else if len(p.createOptions.Version) != 0 {
			return fmt.Errorf("value of --%s can only be used with --%s", helmChartVerison, helmChart)
		}
	}

	if len(strings.TrimSpace(p.createOptions.Chart)) == 0 {
		if len(strings.TrimSpace(p.createOptions.GVK.Group)) == 0 {
			return fmt.Errorf("value of --%s must not have empty value", group)
		}
		if len(strings.TrimSpace(p.createOptions.GVK.Version)) == 0 {
			return fmt.Errorf("value of --%s must not have empty value", version)
		}
		if len(strings.TrimSpace(p.createOptions.GVK.Kind)) == 0 {
			return fmt.Errorf("value of --%s must not have empty value", kind)
		}

		// Validate the resource.
		r := resource.Options{
			Namespaced: true,
			Group:      p.createOptions.GVK.Group,
			Version:    p.createOptions.GVK.Version,
			Kind:       p.createOptions.GVK.Kind,
		}
		if err := r.Validate(); err != nil {
			return err
		}
	}

	return nil
}

func (p *createAPIPlugin) GetScaffolder() (scaffold.Scaffolder, error) {
	return scaffolds.NewAPIScaffolder(p.config, p.createOptions), nil
}

func (p *createAPIPlugin) PostScaffold() error {
	return nil
}
