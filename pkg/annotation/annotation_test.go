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

package annotation_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/action"

	"github.com/joelanford/helm-operator/pkg/annotation"
)

var _ = Describe("Annotation", func() {
	Describe("Install", func() {
		var install action.Install

		BeforeEach(func() {
			install = action.Install{}
		})

		Describe("DisableHooks", func() {
			var a annotation.InstallDisableHooks

			BeforeEach(func() {
				a = annotation.InstallDisableHooks{}
			})

			It("should return a default name", func() {
				Expect(a.Name()).To(Equal(annotation.DefaultInstallDisableHooksName))
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
			var a annotation.InstallDescription

			BeforeEach(func() {
				a = annotation.InstallDescription{}
			})

			It("should return a default name", func() {
				Expect(a.Name()).To(Equal(annotation.DefaultInstallDescriptionName))
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
			var a annotation.UpgradeDisableHooks

			BeforeEach(func() {
				a = annotation.UpgradeDisableHooks{}
			})

			It("should return a default name", func() {
				Expect(a.Name()).To(Equal(annotation.DefaultUpgradeDisableHooksName))
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
			var a annotation.UpgradeForce

			BeforeEach(func() {
				a = annotation.UpgradeForce{}
			})

			It("should return a default name", func() {
				Expect(a.Name()).To(Equal(annotation.DefaultUpgradeForceName))
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
			var a annotation.UpgradeDescription

			BeforeEach(func() {
				a = annotation.UpgradeDescription{}
			})

			It("should return a default name", func() {
				Expect(a.Name()).To(Equal(annotation.DefaultUpgradeDescriptionName))
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
			var a annotation.UninstallDisableHooks

			BeforeEach(func() {
				a = annotation.UninstallDisableHooks{}
			})

			It("should return a default name", func() {
				Expect(a.Name()).To(Equal(annotation.DefaultUninstallDisableHooksName))
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
			var a annotation.UninstallDescription

			BeforeEach(func() {
				a = annotation.UninstallDescription{}
			})

			It("should return a default name", func() {
				Expect(a.Name()).To(Equal(annotation.DefaultUninstallDescriptionName))
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
