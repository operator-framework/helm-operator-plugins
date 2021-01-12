/*
Copyright 2019 The Kubernetes Authors.
Modifications copyright 2020 The Operator-SDK Authors

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

package scaffolds

import (
	"os"

	"sigs.k8s.io/kubebuilder/v2/pkg/model"
	"sigs.k8s.io/kubebuilder/v2/pkg/model/config"

	"github.com/joelanford/helm-operator/internal/version"
	"github.com/joelanford/helm-operator/pkg/plugins/internal/kubebuilder/cmdutil"
	"github.com/joelanford/helm-operator/pkg/plugins/internal/kubebuilder/machinery"
	"github.com/joelanford/helm-operator/pkg/plugins/v1/chartutil"
	"github.com/joelanford/helm-operator/pkg/plugins/v1/scaffolds/internal/templates"
	"github.com/joelanford/helm-operator/pkg/plugins/v1/scaffolds/internal/templates/config/kdefault"
	"github.com/joelanford/helm-operator/pkg/plugins/v1/scaffolds/internal/templates/config/manager"
	"github.com/joelanford/helm-operator/pkg/plugins/v1/scaffolds/internal/templates/config/prometheus"
	"github.com/joelanford/helm-operator/pkg/plugins/v1/scaffolds/internal/templates/config/rbac"
)

const (
	// kustomizeVersion is the sigs.k8s.io/kustomize version to be used in the project
	kustomizeVersion = "v3.5.4"

	imageName = "controller:latest"
)

// helmOperatorVersion is set to the version of helm-operator at compile-time.
var helmOperatorVersion = mustGetScaffoldVersion()
var _ cmdutil.Scaffolder = &initScaffolder{}

type initScaffolder struct {
	config        *config.Config
	apiScaffolder cmdutil.Scaffolder
}

// NewInitScaffolder returns a new Scaffolder for project initialization operations
func NewInitScaffolder(config *config.Config, apiScaffolder cmdutil.Scaffolder) cmdutil.Scaffolder {
	return &initScaffolder{
		config:        config,
		apiScaffolder: apiScaffolder,
	}
}

func (s *initScaffolder) newUniverse() *model.Universe {
	return model.NewUniverse(
		model.WithConfig(s.config),
	)
}

// Scaffold implements Scaffolder
func (s *initScaffolder) Scaffold() error {
	if err := s.scaffold(); err != nil {
		return err
	}
	if s.apiScaffolder != nil {
		return s.apiScaffolder.Scaffold()
	}
	return nil
}

func (s *initScaffolder) scaffold() error {
	if err := os.MkdirAll(chartutil.HelmChartsDir, 0755); err != nil {
		return err
	}
	return machinery.NewScaffold().Execute(
		s.newUniverse(),
		&templates.Dockerfile{
			HelmOperatorVersion: helmOperatorVersion,
		},
		&templates.GitIgnore{},
		&templates.Makefile{
			Image:               imageName,
			KustomizeVersion:    kustomizeVersion,
			HelmOperatorVersion: helmOperatorVersion,
		},
		&templates.Watches{},
		&rbac.AuthProxyRole{},
		&rbac.AuthProxyRoleBinding{},
		&rbac.AuthProxyService{},
		&rbac.ClientClusterRole{},
		&rbac.Kustomization{},
		&rbac.LeaderElectionRole{},
		&rbac.LeaderElectionRoleBinding{},
		&rbac.ManagerRole{},
		&rbac.ManagerRoleBinding{},
		&manager.Kustomization{},
		&manager.Manager{Image: imageName},
		&prometheus.Kustomization{},
		&prometheus.ServiceMonitor{},
		&kdefault.AuthProxyPatch{},
		&kdefault.Kustomization{},
	)
}

func mustGetScaffoldVersion() string {
	if version.ScaffoldVersion == "" || version.ScaffoldVersion == version.Unknown {
		panic("helm-operator scaffold version is unknown; it must be set during build or by importing this plugin via go modules")
	}
	return version.ScaffoldVersion
}
