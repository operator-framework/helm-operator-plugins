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

package fake

import (
	"errors"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"

	"github.com/joelanford/helm-operator/pkg/client"
)

func NewActionClientGetter(actionClient client.ActionInterface, orErr error) client.ActionClientGetter {
	return &fakeActionClientGetter{
		actionClient: actionClient,
		returnErr:    orErr,
	}
}

type fakeActionClientGetter struct {
	actionClient client.ActionInterface
	returnErr    error
}

var _ client.ActionClientGetter = &fakeActionClientGetter{}

func (hcg *fakeActionClientGetter) ActionClientFor(obj client.Object) (client.ActionInterface, error) {
	if hcg.returnErr != nil {
		return nil, hcg.returnErr
	}
	return hcg.actionClient, nil
}

type ActionClient struct {
	Gets       []GetCall
	Installs   []InstallCall
	Upgrades   []UpgradeCall
	Uninstalls []UninstallCall
	Reconciles []ReconcileCall

	HandleGet       func() (*release.Release, error)
	HandleInstall   func() (*release.Release, error)
	HandleUpgrade   func() (*release.Release, error)
	HandleUninstall func() (*release.UninstallReleaseResponse, error)
	HandleReconcile func() error
}

func NewActionClient() ActionClient {
	relFunc := func(err error) func() (*release.Release, error) {
		return func() (*release.Release, error) { return nil, err }
	}
	uninstFunc := func(err error) func() (*release.UninstallReleaseResponse, error) {
		return func() (*release.UninstallReleaseResponse, error) { return nil, err }
	}
	recFunc := func(err error) func() error {
		return func() error { return err }
	}
	return ActionClient{
		Gets:       make([]GetCall, 0),
		Installs:   make([]InstallCall, 0),
		Upgrades:   make([]UpgradeCall, 0),
		Uninstalls: make([]UninstallCall, 0),
		Reconciles: make([]ReconcileCall, 0),

		HandleGet:       relFunc(errors.New("get not implemented")),
		HandleInstall:   relFunc(errors.New("install not implemented")),
		HandleUpgrade:   relFunc(errors.New("upgrade not implemented")),
		HandleUninstall: uninstFunc(errors.New("uninstall not implemented")),
		HandleReconcile: recFunc(errors.New("reconcile not implemented")),
	}
}

var _ client.ActionInterface = &ActionClient{}

type GetCall struct {
	Name string
	Opts []client.GetOption
}

type InstallCall struct {
	Name      string
	Namespace string
	Chart     *chart.Chart
	Values    map[string]interface{}
	Opts      []client.InstallOption
}

type UpgradeCall struct {
	Name      string
	Namespace string
	Chart     *chart.Chart
	Values    map[string]interface{}
	Opts      []client.UpgradeOption
}

type UninstallCall struct {
	Name string
	Opts []client.UninstallOption
}

type ReconcileCall struct {
	Release *release.Release
}

func (c *ActionClient) Get(name string, opts ...client.GetOption) (*release.Release, error) {
	c.Gets = append(c.Gets, GetCall{name, opts})
	return c.HandleGet()
}

func (c *ActionClient) Install(name, namespace string, chrt *chart.Chart, vals map[string]interface{}, opts ...client.InstallOption) (*release.Release, error) {
	c.Installs = append(c.Installs, InstallCall{name, namespace, chrt, vals, opts})
	return c.HandleInstall()
}

func (c *ActionClient) Upgrade(name, namespace string, chrt *chart.Chart, vals map[string]interface{}, opts ...client.UpgradeOption) (*release.Release, error) {
	c.Upgrades = append(c.Upgrades, UpgradeCall{name, namespace, chrt, vals, opts})
	return c.HandleUpgrade()
}

func (c *ActionClient) Uninstall(name string, opts ...client.UninstallOption) (*release.UninstallReleaseResponse, error) {
	c.Uninstalls = append(c.Uninstalls, UninstallCall{name, opts})
	return c.HandleUninstall()
}

func (c *ActionClient) Reconcile(rel *release.Release) error {
	c.Reconciles = append(c.Reconciles, ReconcileCall{rel})
	return c.HandleReconcile()
}
