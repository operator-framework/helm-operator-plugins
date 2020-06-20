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
	// DefaultInstallAnnotations are the default annotations that can be used to enable custom action.Install fields
	// to be set during release installations. More info: https://helm.sh/docs/topics/charts_hooks/.
	DefaultInstallAnnotations   = []Install{InstallDescription{}, InstallDisableHooks{}}
	// DefaultUpgradeAnnotations are the default annotations that can be used to enable custom action.Upgrade fields
	// to be set during release installations. More info: https://helm.sh/docs/topics/charts_hooks/.
	DefaultUpgradeAnnotations   = []Upgrade{UpgradeDescription{}, UpgradeDisableHooks{}, UpgradeForce{}}
	// DefaultUninstallAnnotations are the default annotations that can be used to enable custom action.Uninstall fields
	// to be set during release installations. More info: https://helm.sh/docs/topics/charts_hooks/.
	DefaultUninstallAnnotations = []Uninstall{UninstallDescription{}, UninstallDisableHooks{}}
)

// Install is the interface that is used to customize action.Install fields based on a Kubernetes annotation.
type Install interface {
	Name() string
	InstallOption(string) helmclient.InstallOption
}

// Upgrade is the interface that is used to customize action.Upgrade fields based on a Kubernetes annotation.
type Upgrade interface {
	Name() string
	UpgradeOption(string) helmclient.UpgradeOption
}

// Uninstall is the interface that is used to customize action.Uninstall fields based on a Kubernetes annotation.
type Uninstall interface {
	Name() string
	UninstallOption(string) helmclient.UninstallOption
}

// InstallDisableHooks is an install annotation that disables chart hooks during release installation.
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

// Name returns the annotation name for the InstallDisableHooks annotation. If the annotation's CustomName is set,
// it is returned. Otherwise the default name is returned.
func (i InstallDisableHooks) Name() string {
	if i.CustomName != "" {
		return i.CustomName
	}
	return defaultInstallDisableHooksName
}
// InstallOption returns a client.InstallOption that disables chart hooks based on the value of the annotation.
// The annotation value is parsed using strconv.Parse().
//
// By default (or if there is an error parsing the annotation value), the install.DisableHooks field is set to false.
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

// UpgradeDisableHooks represents the annotation to disable upgrade hooks
type UpgradeDisableHooks struct {
	CustomName string
}

var _ Upgrade = &UpgradeDisableHooks{}

// Name returns the annotation name for the UpgradeDisableHooks annotation. If the annotation's CustomName is set,
// it is returned. Otherwise the default name is returned.
func (u UpgradeDisableHooks) Name() string {
	if u.CustomName != "" {
		return u.CustomName
	}
	return defaultUpgradeDisableHooksName
}

// UpgradeOption returns a client.UpgradeOption that disables chart hooks based on the value of the annotation.
// The annotation value is parsed using strconv.Parse().
//
// By default (or if there is an error parsing the annotation value), the upgrade.DisableHooks field is set to false.
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

// UpgradeForce represents the annotation to set upgrade.Force (helm upgrade --force)
type UpgradeForce struct {
	CustomName string
}

var _ Upgrade = &UpgradeForce{}

// Name will return the custom or the default annotation name for an UpgradeForce annotation
func (u UpgradeForce) Name() string {
	if u.CustomName != "" {
		return u.CustomName
	}
	return defaultUpgradeForceName
}

// UpgradeOption returns a client.UpgradeOption that will be used to run the upgrade release with the
// flag --force. For more info check helm upgrade --force
// The annotation value is parsed using strconv.ParseBool().
//
// By default (or if there is an error parsing the annotation value), the upgrade.Force field is set to false.
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

// UninstallDisableHooks represents the annotation to disable uninstall hooks
type UninstallDisableHooks struct {
	CustomName string
}

var _ Uninstall = &UninstallDisableHooks{}

// Name returns the annotation name for the UninstallDisableHooks annotation. If the annotation's CustomName is set,
// it is returned. Otherwise the default name is returned.
func (u UninstallDisableHooks) Name() string {
	if u.CustomName != "" {
		return u.CustomName
	}
	return defaultUninstallDisableHooksName
}

// UninstallOption returns a client.UninstallOption that disables chart hooks based on the value of the annotation.
// The annotation value is parsed using strconv.ParseBool().
//
// By default (or if there is an error parsing the annotation value), the uninstall.DisableHooks field is set to false.
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

// InstallDescription represents the annotation to set an i.Description
type InstallDescription struct {
	CustomName string
}

// Name returns the annotation name for the InstallDescription annotation. If the annotation's CustomName is set,
// it is returned. Otherwise the default name is returned.
func (i InstallDescription) Name() string {
	if i.CustomName != "" {
		return i.CustomName
	}
	return defaultInstallDescriptionName
}

// InstallOption returns a client.InstallOption that set a description chart hooks based on the value of the annotation.
func (i InstallDescription) InstallOption(v string) helmclient.InstallOption {
	return func(i *action.Install) error {
		i.Description = v
		return nil
	}
}

var _ Upgrade = &UpgradeDescription{}

// UpgradeDescription represents the annotation to set an upgrade.Description
type UpgradeDescription struct {
	CustomName string
}

// Name returns the annotation name for the UpgradeDescription annotation. If the annotation's CustomName is set,
// it is returned. Otherwise the default name is returned.
func (u UpgradeDescription) Name() string {
	if u.CustomName != "" {
		return u.CustomName
	}
	return defaultUpgradeDescriptionName
}

// UpgradeOption returns a client.UpgradeOption that set a upgrade.Description chart hooks based on the
// value of the annotation.
func (u UpgradeDescription) UpgradeOption(v string) helmclient.UpgradeOption {
	return func(upgrade *action.Upgrade) error {
		upgrade.Description = v
		return nil
	}
}

var _ Uninstall = &UninstallDescription{}

// UninstallDescription represents the annotation to set an uninstall.Description
type UninstallDescription struct {
	CustomName string
}

// Name returns the annotation name for the UninstallDescription annotation. If the annotation's CustomName is set,
// it is returned. Otherwise the default name is returned.
func (u UninstallDescription) Name() string {
	if u.CustomName != "" {
		return u.CustomName
	}
	return defaultUninstallDescriptionName
}

// UninstallOption returns a client.UninstallOption that set a uninstall.Description chart hooks based on the
// value of the annotation.
func (u UninstallDescription) UninstallOption(v string) helmclient.UninstallOption {
	return func(uninstall *action.Uninstall) error {
		uninstall.Description = v
		return nil
	}
}
