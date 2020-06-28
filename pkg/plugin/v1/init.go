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
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"
	"sigs.k8s.io/kubebuilder/pkg/model/config"
	"sigs.k8s.io/kubebuilder/pkg/plugin"
	"sigs.k8s.io/kubebuilder/pkg/plugin/scaffold"

	"github.com/joelanford/helm-operator/pkg/internal/kubebuilder/cmdutil"
	"github.com/joelanford/helm-operator/pkg/internal/kubebuilder/validation"
	"github.com/joelanford/helm-operator/pkg/plugin/internal/chartutil"
	"github.com/joelanford/helm-operator/pkg/plugin/v1/scaffolds"
)

type initPlugin struct {
	config        *config.Config
	doAPIScaffold bool
	apiPlugin     createAPIPlugin

	// For help text.
	commandName string
}

var (
	_ plugin.Init        = &initPlugin{}
	_ cmdutil.RunOptions = &initPlugin{}
)

// UpdateContext define plugin context
func (p *initPlugin) UpdateContext(ctx *plugin.Context) {
	ctx.Description = `Initialize a new Helm-based operator project.

Writes the following files:
- a helm-charts directory with the chart(s) to build releases from
- a watches.yaml file that defines the mapping between your API and a Helm chart
- a PROJECT file with the domain and repo
- a Makefile to build the project
- a Kustomization.yaml for customizating manifests
- a Patch file for customizing image for manager manifests
- a Patch file for enabling prometheus metrics
`
	ctx.Examples = fmt.Sprintf(`  $ %s init --plugins=%s \
      --domain=example.com \
      --group=apps --version=v1alpha1 \
      --kind=AppService

  $ %s init --plugins=%s \
      --domain=example.com \
      --group=apps --version=v1alpha1 \
      --kind=AppService \
      --helm-chart=myrepo/app

  $ %s init --plugins=%s \
      --domain=example.com \
      --helm-chart=myrepo/app

  $ %s init --plugins=%s \
      --domain=example.com \
      --helm-chart=myrepo/app \
      --helm-chart-version=1.2.3

  $ %s init --plugins=%s \
      --domain=example.com \
      --helm-chart=app \
      --helm-chart-repo=https://charts.mycompany.com/

  $ %s init --plugins=%s \
      --domain=example.com \
      --helm-chart=app \
      --helm-chart-repo=https://charts.mycompany.com/ \
      --helm-chart-version=1.2.3

  $ %s init --plugins=%s \
      --domain=example.com \
      --helm-chart=/path/to/local/chart-directories/app/

  $ %s init --plugins=%s \
      --domain=example.com \
      --helm-chart=/path/to/local/chart-archives/app-1.2.3.tgz
`,
		ctx.CommandName, plugin.KeyFor(Plugin{}),
		ctx.CommandName, plugin.KeyFor(Plugin{}),
		ctx.CommandName, plugin.KeyFor(Plugin{}),
		ctx.CommandName, plugin.KeyFor(Plugin{}),
		ctx.CommandName, plugin.KeyFor(Plugin{}),
		ctx.CommandName, plugin.KeyFor(Plugin{}),
		ctx.CommandName, plugin.KeyFor(Plugin{}),
		ctx.CommandName, plugin.KeyFor(Plugin{}),
	)

	p.commandName = ctx.CommandName
}

// BindFlags bind all plugin flags
func (p *initPlugin) BindFlags(fs *pflag.FlagSet) {
	fs.SortFlags = false
	fs.StringVar(&p.config.Domain, "domain", "my.domain",
		"Kubernetes domain for groups. (e.g example.com)")
	p.apiPlugin.BindFlags(fs)
}

// InjectConfig will inject the PROJECT file/config in the plugin
func (p *initPlugin) InjectConfig(c *config.Config) {
	// v3 project configs get a 'layout' value.
	c.Layout = plugin.KeyFor(Plugin{})
	p.config = c
	p.apiPlugin.config = p.config
}

// Run will call the plugin actions
func (p *initPlugin) Run() error {
	return cmdutil.Run(p)
}

// Validate will perform the required checks before to start to scaffold
func (p *initPlugin) Validate() error {
	// Check if the project name is a valid namespace according to k8s
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error to get the current path: %v", err)
	}
	projectName := filepath.Base(dir)
	if err := validation.IsDNS1123Label(strings.ToLower(projectName)); err != nil {
		return fmt.Errorf("project name (%s) is invalid: %v", projectName, err)
	}

	defaultOpts := chartutil.CreateOptions{CRDVersion: "v1"}
	if !p.apiPlugin.apiFlags.gvk.Empty() || p.apiPlugin.apiFlags.createOptions != defaultOpts {
		p.doAPIScaffold = true
		// should not do the create api validation because it has specifics checks that
		// are not valid for the init plugin E.g ensure that the PROJECT/config file is scaffolded
		return p.apiPlugin.apiFlags.Validate()
	}

	// set the repo to provide the operator name
	if p.config.Repo == "" {
		p.config.Repo = projectName
	}
	return nil
}

// GetScaffolder will run the plugin scaffold
func (p *initPlugin) GetScaffolder() (scaffold.Scaffolder, error) {
	var (
		apiScaffolder scaffold.Scaffolder
		err           error
	)
	if p.doAPIScaffold {
		apiScaffolder, err = p.apiPlugin.GetScaffolder()
		if err != nil {
			return nil, err
		}
	}
	return scaffolds.NewInitScaffolder(p.config, apiScaffolder), nil
}

// PostScaffold will run the required actions after the default plugin scaffold
func (p *initPlugin) PostScaffold() error {
	if !p.doAPIScaffold {
		fmt.Printf("Next: define a resource with:\n$ %s create api\n", p.commandName)
	}
	return nil
}
