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

	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/action"

	helmclient "github.com/joelanford/helm-operator/pkg/client"
)

var (
	DefaultInstallAnnotations   = []Install{InstallDescription{}, InstallDisableHooks{}}
	DefaultUpgradeAnnotations   = []Upgrade{UpgradeDescription{}, UpgradeDisableHooks{}, UpgradeForce{}}
	DefaultUninstallAnnotations = []Uninstall{UninstallDescription{}, UninstallDisableHooks{}}
)

type Install interface {
	Name() string
	InstallOption(string, logr.Logger) helmclient.InstallOption
}

type Upgrade interface {
	Name() string
	UpgradeOption(string, logr.Logger) helmclient.UpgradeOption
}

type Uninstall interface {
	Name() string
	UninstallOption(string, logr.Logger) helmclient.UninstallOption
}

type InstallDisableHooks struct {
	CustomName string
}

var _ Install = &InstallDisableHooks{}

const (
	DefaultDomain                    = "helm.operator-sdk"
	DefaultInstallDisableHooksName   = DefaultDomain + "/install-disable-hooks"
	DefaultUpgradeDisableHooksName   = DefaultDomain + "/upgrade-disable-hooks"
	DefaultUninstallDisableHooksName = DefaultDomain + "/uninstall-disable-hooks"

	DefaultUpgradeForceName = DefaultDomain + "/upgrade-force"

	DefaultInstallDescriptionName   = DefaultDomain + "/install-description"
	DefaultUpgradeDescriptionName   = DefaultDomain + "/upgrade-description"
	DefaultUninstallDescriptionName = DefaultDomain + "/uninstall-description"
)

func (i InstallDisableHooks) Name() string {
	if i.CustomName != "" {
		return i.CustomName
	}
	return DefaultInstallDisableHooksName
}

func (i InstallDisableHooks) InstallOption(val string, log logr.Logger) helmclient.InstallOption {
	disableHooks := false
	v, err := strconv.ParseBool(val)
	logAnnotationWith(err, log, i.Name(), val)
	if err == nil {
		disableHooks = v
	}
	return func(install *action.Install) error {
		install.DisableHooks = disableHooks
		return nil
	}
}

type UpgradeDisableHooks struct {
	CustomName string
}

var _ Upgrade = &UpgradeDisableHooks{}

func (u UpgradeDisableHooks) Name() string {
	if u.CustomName != "" {
		return u.CustomName
	}
	return DefaultUpgradeDisableHooksName
}

func (u UpgradeDisableHooks) UpgradeOption(val string, log logr.Logger) helmclient.UpgradeOption {
	disableHooks := false
	v, err := strconv.ParseBool(val)
	logAnnotationWith(err, log, u.Name(), val)
	if err == nil {
		disableHooks = v
	}
	return func(upgrade *action.Upgrade) error {
		upgrade.DisableHooks = disableHooks
		return nil
	}
}

type UpgradeForce struct {
	CustomName string
}

var _ Upgrade = &UpgradeForce{}

func (u UpgradeForce) Name() string {
	if u.CustomName != "" {
		return u.CustomName
	}
	return DefaultUpgradeForceName
}

func (u UpgradeForce) UpgradeOption(val string, log logr.Logger) helmclient.UpgradeOption {
	force := false
	v, err := strconv.ParseBool(val)
	logAnnotationWith(err, log, u.Name(), val)
	if err == nil {
		force = v
	}
	return func(upgrade *action.Upgrade) error {
		upgrade.Force = force
		return nil
	}
}

type UninstallDisableHooks struct {
	CustomName string
}

var _ Uninstall = &UninstallDisableHooks{}

func (u UninstallDisableHooks) Name() string {
	if u.CustomName != "" {
		return u.CustomName
	}
	return DefaultUninstallDisableHooksName
}

func (u UninstallDisableHooks) UninstallOption(val string, log logr.Logger) helmclient.UninstallOption {
	disableHooks := false
	v, err := strconv.ParseBool(val)
	logAnnotationWith(err, log, u.Name(), val)
	if err == nil {
		disableHooks = v
	}
	return func(uninstall *action.Uninstall) error {
		uninstall.DisableHooks = disableHooks
		return nil
	}
}

var _ Install = &InstallDescription{}

type InstallDescription struct {
	CustomName string
}

func (i InstallDescription) Name() string {
	if i.CustomName != "" {
		return i.CustomName
	}
	return DefaultInstallDescriptionName
}
func (i InstallDescription) InstallOption(v string, log logr.Logger) helmclient.InstallOption {
	logAnnotationWith(nil, log, i.Name(), v)
	return func(i *action.Install) error {
		i.Description = v
		return nil
	}
}

var _ Upgrade = &UpgradeDescription{}

type UpgradeDescription struct {
	CustomName string
}

func (u UpgradeDescription) Name() string {
	if u.CustomName != "" {
		return u.CustomName
	}
	return DefaultUpgradeDescriptionName
}
func (u UpgradeDescription) UpgradeOption(v string, log logr.Logger) helmclient.UpgradeOption {
	logAnnotationWith(nil, log, u.Name(), v)
	return func(upgrade *action.Upgrade) error {
		upgrade.Description = v
		return nil
	}
}

var _ Uninstall = &UninstallDescription{}

type UninstallDescription struct {
	CustomName string
}

func (u UninstallDescription) Name() string {
	if u.CustomName != "" {
		return u.CustomName
	}
	return DefaultUninstallDescriptionName
}
func (u UninstallDescription) UninstallOption(v string, log logr.Logger) helmclient.UninstallOption {
	logAnnotationWith(nil, log, u.Name(), v)
	return func(uninstall *action.Uninstall) error {
		uninstall.Description = v
		return nil
	}
}

// logAnnotationWith will log the error or an info with the annotation set and its values
func logAnnotationWith(err error, log logr.Logger, name string, val string) {
	if err != nil {
		log.Error(err, "error to set annotation",
			"name", name, "value", val)
	} else {
		if log.V(1).Enabled() {
			log.Info("setting annotation ", "name", name, "value", val)
		}
	}
}
