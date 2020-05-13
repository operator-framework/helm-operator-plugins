package annotation

import (
	"strconv"

	"helm.sh/helm/v3/pkg/action"

	helmclient "github.com/joelanford/helm-operator/pkg/client"
)

type Install interface {
	Name() string
	InstallOption(string) helmclient.InstallOption
}

type Upgrade interface {
	Name() string
	UpgradeOption(string) helmclient.UpgradeOption
}

type Uninstall interface {
	Name() string
	UninstallOption(string) helmclient.UninstallOption
}

type InstallDisableHooks struct {
	CustomName string
}

var _ Install = &InstallDisableHooks{}

const (
	DefaultDomain                    = "helm.sdk.operatorframework.io"
	DefaultInstallDisableHooksName   = DefaultDomain + "/install-disable-hooks"
	DefaultUpgradeDisableHooksName   = DefaultDomain + "/upgrade-disable-hooks"
	DefaultUninstallDisableHooksName = DefaultDomain + "/uninstall-disable-hooks"

	DefaultUpgradeForceName = DefaultDomain + "/upgrade-force"
)

func (i InstallDisableHooks) Name() string {
	if i.CustomName != "" {
		return i.CustomName
	}
	return DefaultInstallDisableHooksName
}

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
