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

package annotation

import (
	"strconv"

	"helm.sh/helm/v3/pkg/action"

	helmclient "github.com/joelanford/helm-operator/pkg/client"
)

var (
	// DefaultInstallAnnotations //TODO
	DefaultInstallAnnotations = []Install{InstallDescription{}, InstallDisableHooks{}}
	// DefaultUpgradeAnnotations //TODO
	DefaultUpgradeAnnotations = []Upgrade{UpgradeDescription{}, UpgradeDisableHooks{}, UpgradeForce{}}
	// DefaultUninstallAnnotations //TODO
	DefaultUninstallAnnotations = []Uninstall{UninstallDescription{}, UninstallDisableHooks{}}
)

// Install //TODO
type Install interface {
	Name() string
	InstallOption(string) helmclient.InstallOption
}

// Upgrade //TODO
type Upgrade interface {
	Name() string
	UpgradeOption(string) helmclient.UpgradeOption
}

// Uninstall //TODO
type Uninstall interface {
	Name() string
	UninstallOption(string) helmclient.UninstallOption
}

// InstallDisableHooks //TODO
type InstallDisableHooks struct {
	CustomName string
}

var _ Install = &InstallDisableHooks{}

const (
	defaultDomain                    = "helm.operator-sdk"
	defaultInstallDisableHooksName   = defaultDomain + "/install-disable-hooks"
	defaultUpgradeDisableHooksName   = defaultDomain + "/upgrade-disable-hooks"
	defaultUninstallDisableHooksName = defaultDomain + "/uninstall-disable-hooks"

	defaultUpgradeForceName = defaultDomain + "/upgrade-force"

	defaultInstallDescriptionName   = defaultDomain + "/install-description"
	defaultUpgradeDescriptionName   = defaultDomain + "/upgrade-description"
	defaultUninstallDescriptionName = defaultDomain + "/uninstall-description"
)

// Name //TODO
func (i InstallDisableHooks) Name() string {
	if i.CustomName != "" {
		return i.CustomName
	}
	return defaultInstallDisableHooksName
}

// InstallOption //TODO
func (i InstallDisableHooks) InstallOption(val string) helmclient.InstallOption {
	disableHooks := false
	if v, err := strconv.ParseBool(val); err == nil {
		disableHooks = v
	}
	return func(install *action.Install) error {
		install.DisableHooks = disableHooks
		return nil
	}
}

// UpgradeDisableHooks  //TODO
type UpgradeDisableHooks struct {
	CustomName string
}

var _ Upgrade = &UpgradeDisableHooks{}

// Name  //TODO
func (u UpgradeDisableHooks) Name() string {
	if u.CustomName != "" {
		return u.CustomName
	}
	return defaultUpgradeDisableHooksName
}

// UpgradeOption  //TODO
func (u UpgradeDisableHooks) UpgradeOption(val string) helmclient.UpgradeOption {
	disableHooks := false
	if v, err := strconv.ParseBool(val); err == nil {
		disableHooks = v
	}
	return func(upgrade *action.Upgrade) error {
		upgrade.DisableHooks = disableHooks
		return nil
	}
}

// UpgradeForce  //TODO
type UpgradeForce struct {
	CustomName string
}

var _ Upgrade = &UpgradeForce{}

// Name  //TODO
func (u UpgradeForce) Name() string {
	if u.CustomName != "" {
		return u.CustomName
	}
	return defaultUpgradeForceName
}

// UpgradeOption  //TODO
func (u UpgradeForce) UpgradeOption(val string) helmclient.UpgradeOption {
	force := false
	if v, err := strconv.ParseBool(val); err == nil {
		force = v
	}
	return func(upgrade *action.Upgrade) error {
		upgrade.Force = force
		return nil
	}
}

// UninstallDisableHooks //TODO
type UninstallDisableHooks struct {
	CustomName string
}

var _ Uninstall = &UninstallDisableHooks{}

// Name //TODO
func (u UninstallDisableHooks) Name() string {
	if u.CustomName != "" {
		return u.CustomName
	}
	return defaultUninstallDisableHooksName
}

// UninstallOption //TODO
func (u UninstallDisableHooks) UninstallOption(val string) helmclient.UninstallOption {
	disableHooks := false
	if v, err := strconv.ParseBool(val); err == nil {
		disableHooks = v
	}
	return func(uninstall *action.Uninstall) error {
		uninstall.DisableHooks = disableHooks
		return nil
	}
}

var _ Install = &InstallDescription{}

// InstallDescription //TODO
type InstallDescription struct {
	CustomName string
}

// Name //TODO
func (i InstallDescription) Name() string {
	if i.CustomName != "" {
		return i.CustomName
	}
	return defaultInstallDescriptionName
}

// InstallOption //TODO
func (i InstallDescription) InstallOption(v string) helmclient.InstallOption {
	return func(i *action.Install) error {
		i.Description = v
		return nil
	}
}

var _ Upgrade = &UpgradeDescription{}

// UpgradeDescription //TODO
type UpgradeDescription struct {
	CustomName string
}

// Name //TODO
func (u UpgradeDescription) Name() string {
	if u.CustomName != "" {
		return u.CustomName
	}
	return defaultUpgradeDescriptionName
}

// UpgradeOption //TODO
func (u UpgradeDescription) UpgradeOption(v string) helmclient.UpgradeOption {
	return func(upgrade *action.Upgrade) error {
		upgrade.Description = v
		return nil
	}
}

var _ Uninstall = &UninstallDescription{}

// UninstallDescription //TODO
type UninstallDescription struct {
	CustomName string
}

// Name //TODO
func (u UninstallDescription) Name() string {
	if u.CustomName != "" {
		return u.CustomName
	}
	return defaultUninstallDescriptionName
}

// UninstallOption //TODO
func (u UninstallDescription) UninstallOption(v string) helmclient.UninstallOption {
	return func(uninstall *action.Uninstall) error {
		uninstall.Description = v
		return nil
	}
}
