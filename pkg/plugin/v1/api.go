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
	"github.com/spf13/pflag"
	"sigs.k8s.io/kubebuilder/pkg/model/config"
	"sigs.k8s.io/kubebuilder/pkg/plugin"
	"sigs.k8s.io/kubebuilder/pkg/plugin/scaffold"

	"github.com/joelanford/helm-operator/pkg/internal/kubebuilder/cmdutil"
	"github.com/joelanford/helm-operator/pkg/plugin/v1/scaffolds"
)

type createAPIPlugin struct {
	config *config.Config

	apiFlags APIFlags
}

var (
	_ plugin.CreateAPI   = &createAPIPlugin{}
	_ cmdutil.RunOptions = &createAPIPlugin{}
)

// UpdateContext define plugin context
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

// BindFlags bind all plugin flags
func (p *createAPIPlugin) BindFlags(fs *pflag.FlagSet) {
	p.apiFlags.AddTo(fs)
	fs.SortFlags = false
}

// InjectConfig will inject the PROJECT file/config in the plugin
func (p *createAPIPlugin) InjectConfig(c *config.Config) {
	p.config = c
}

// Run will call the plugin actions
func (p *createAPIPlugin) Run() error {
	return cmdutil.Run(p)
}

// Validate perform the required validations for this plugin
func (p *createAPIPlugin) Validate() error {
	// ensure that has config and the layout is valid for this plugin
	if !hasPluginConfig(p.config) {
		return fmt.Errorf("missing configuration for the helm plugin layout %v", plugin.KeyFor(Plugin{}))
	}

	// validate the api flags informed
	if err := p.apiFlags.Validate(); err != nil {
		return err
	}

	return nil
}

// GetScaffolder defines the templates that should be scaffold
func (p *createAPIPlugin) GetScaffolder() (scaffold.Scaffolder, error) {
	return scaffolds.NewAPIScaffolder(p.config, p.apiFlags.createOptions), nil
}

// PostScaffold will run the required actions after the default plugin scaffold
func (p *createAPIPlugin) PostScaffold() error {
	return nil
}
