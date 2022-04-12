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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/action"
)

var _ = Describe("Annotation", func() {
	Describe("Install", func() {
		var install action.Install

		BeforeEach(func() {
			install = action.Install{}
		})

		Describe("DisableHooks", func() {
			var a InstallDisableHooks

			BeforeEach(func() {
				a = InstallDisableHooks{}
			})

			It("should return a default name", func() {
				Expect(a.Name()).To(Equal(defaultInstallDisableHooksName))
			})

			It("should return a custom name", func() {
				const customName = "custom.domain/custom-name"
				a.CustomName = customName
				Expect(a.Name()).To(Equal(customName))
			})

			It("should disable hooks", func() {
				Expect(a.InstallOption("true")(&install)).To(Succeed())
				Expect(install.DisableHooks).To(BeTrue())
			})

			It("should enable hooks", func() {
				install.DisableHooks = true
				Expect(a.InstallOption("false")(&install)).To(Succeed())
				Expect(install.DisableHooks).To(BeFalse())
			})

			It("should default to false with invalid value", func() {
				install.DisableHooks = true
				Expect(a.InstallOption("invalid")(&install)).To(Succeed())
				Expect(install.DisableHooks).To(BeFalse())
			})
		})

		Describe("Description", func() {
			var a InstallDescription

			BeforeEach(func() {
				a = InstallDescription{}
			})

			It("should return a default name", func() {
				Expect(a.Name()).To(Equal(defaultInstallDescriptionName))
			})

			It("should return a custom name", func() {
				const customName = "custom.domain/custom-name"
				a.CustomName = customName
				Expect(a.Name()).To(Equal(customName))
			})

			It("should set a description", func() {
				Expect(a.InstallOption("test description")(&install)).To(Succeed())
				Expect(install.Description).To(Equal("test description"))
			})
		})
	})

	Describe("Upgrade", func() {
		var upgrade action.Upgrade

		BeforeEach(func() {
			upgrade = action.Upgrade{}
		})

		Describe("DisableHooks", func() {
			var a UpgradeDisableHooks

			BeforeEach(func() {
				a = UpgradeDisableHooks{}
			})

			It("should return a default name", func() {
				Expect(a.Name()).To(Equal(defaultUpgradeDisableHooksName))
			})

			It("should return a custom name", func() {
				const customName = "custom.domain/custom-name"
				a.CustomName = customName
				Expect(a.Name()).To(Equal(customName))
			})

			It("should disable hooks", func() {
				Expect(a.UpgradeOption("true")(&upgrade)).To(Succeed())
				Expect(upgrade.DisableHooks).To(BeTrue())
			})

			It("should enable hooks", func() {
				upgrade.DisableHooks = true
				Expect(a.UpgradeOption("false")(&upgrade)).To(Succeed())
				Expect(upgrade.DisableHooks).To(BeFalse())
			})

			It("should default to false with invalid value", func() {
				upgrade.DisableHooks = true
				Expect(a.UpgradeOption("invalid")(&upgrade)).To(Succeed())
				Expect(upgrade.DisableHooks).To(BeFalse())
			})
		})

		Describe("Force", func() {
			var a UpgradeForce

			BeforeEach(func() {
				a = UpgradeForce{}
			})

			It("should return a default name", func() {
				Expect(a.Name()).To(Equal(defaultUpgradeForceName))
			})

			It("should return a custom name", func() {
				const customName = "custom.domain/custom-name"
				a.CustomName = customName
				Expect(a.Name()).To(Equal(customName))
			})

			It("should force upgrades", func() {
				Expect(a.UpgradeOption("true")(&upgrade)).To(Succeed())
				Expect(upgrade.Force).To(BeTrue())
			})

			It("should not force upgrades", func() {
				upgrade.Force = true
				Expect(a.UpgradeOption("false")(&upgrade)).To(Succeed())
				Expect(upgrade.Force).To(BeFalse())
			})

			It("should default to not forcing upgrades", func() {
				upgrade.Force = true
				Expect(a.UpgradeOption("invalid")(&upgrade)).To(Succeed())
				Expect(upgrade.Force).To(BeFalse())
			})
		})

		Describe("Description", func() {
			var a UpgradeDescription

			BeforeEach(func() {
				a = UpgradeDescription{}
			})

			It("should return a default name", func() {
				Expect(a.Name()).To(Equal(defaultUpgradeDescriptionName))
			})

			It("should return a custom name", func() {
				const customName = "custom.domain/custom-name"
				a.CustomName = customName
				Expect(a.Name()).To(Equal(customName))
			})

			It("should set a description", func() {
				Expect(a.UpgradeOption("test description")(&upgrade)).To(Succeed())
				Expect(upgrade.Description).To(Equal("test description"))
			})
		})
	})

	Describe("Uninstall", func() {
		var uninstall action.Uninstall

		BeforeEach(func() {
			uninstall = action.Uninstall{}
		})

		Describe("DisableHooks", func() {
			var a UninstallDisableHooks

			BeforeEach(func() {
				a = UninstallDisableHooks{}
			})

			It("should return a default name", func() {
				Expect(a.Name()).To(Equal(defaultUninstallDisableHooksName))
			})

			It("should return a custom name", func() {
				const customName = "custom.domain/custom-name"
				a.CustomName = customName
				Expect(a.Name()).To(Equal(customName))
			})

			It("should disable hooks", func() {
				Expect(a.UninstallOption("true")(&uninstall)).To(Succeed())
				Expect(uninstall.DisableHooks).To(BeTrue())
			})

			It("should enable hooks", func() {
				uninstall.DisableHooks = true
				Expect(a.UninstallOption("false")(&uninstall)).To(Succeed())
				Expect(uninstall.DisableHooks).To(BeFalse())
			})

			It("should default to false with invalid value", func() {
				uninstall.DisableHooks = true
				Expect(a.UninstallOption("invalid")(&uninstall)).To(Succeed())
				Expect(uninstall.DisableHooks).To(BeFalse())
			})
		})

		Describe("Description", func() {
			var a UninstallDescription

			BeforeEach(func() {
				a = UninstallDescription{}
			})

			It("should return a default name", func() {
				Expect(a.Name()).To(Equal(defaultUninstallDescriptionName))
			})

			It("should return a custom name", func() {
				const customName = "custom.domain/custom-name"
				a.CustomName = customName
				Expect(a.Name()).To(Equal(customName))
			})

			It("should set a description", func() {
				Expect(a.UninstallOption("test description")(&uninstall)).To(Succeed())
				Expect(uninstall.Description).To(Equal("test description"))
			})
		})
	})
})
