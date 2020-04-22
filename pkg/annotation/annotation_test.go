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
				Expect(a.InstallOption("true")(&install)).NotTo(HaveOccurred())
				Expect(install.DisableHooks).To(BeTrue())
			})

			It("should enable hooks", func() {
				install.DisableHooks = true
				Expect(a.InstallOption("false")(&install)).NotTo(HaveOccurred())
				Expect(install.DisableHooks).To(BeFalse())
			})

			It("should default to false with invalid value", func() {
				install.DisableHooks = true
				Expect(a.InstallOption("invalid")(&install)).NotTo(HaveOccurred())
				Expect(install.DisableHooks).To(BeFalse())
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
				Expect(a.UpgradeOption("true")(&upgrade)).NotTo(HaveOccurred())
				Expect(upgrade.DisableHooks).To(BeTrue())
			})

			It("should enable hooks", func() {
				upgrade.DisableHooks = true
				Expect(a.UpgradeOption("false")(&upgrade)).NotTo(HaveOccurred())
				Expect(upgrade.DisableHooks).To(BeFalse())
			})

			It("should default to false with invalid value", func() {
				upgrade.DisableHooks = true
				Expect(a.UpgradeOption("invalid")(&upgrade)).NotTo(HaveOccurred())
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
				Expect(a.UpgradeOption("true")(&upgrade)).NotTo(HaveOccurred())
				Expect(upgrade.Force).To(BeTrue())
			})

			It("should not force upgrades", func() {
				upgrade.Force = true
				Expect(a.UpgradeOption("false")(&upgrade)).NotTo(HaveOccurred())
				Expect(upgrade.Force).To(BeFalse())
			})

			It("should default to not forcing upgrades", func() {
				upgrade.Force = true
				Expect(a.UpgradeOption("invalid")(&upgrade)).NotTo(HaveOccurred())
				Expect(upgrade.Force).To(BeFalse())
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
				Expect(a.UninstallOption("true")(&uninstall)).NotTo(HaveOccurred())
				Expect(uninstall.DisableHooks).To(BeTrue())
			})

			It("should enable hooks", func() {
				uninstall.DisableHooks = true
				Expect(a.UninstallOption("false")(&uninstall)).NotTo(HaveOccurred())
				Expect(uninstall.DisableHooks).To(BeFalse())
			})

			It("should default to false with invalid value", func() {
				uninstall.DisableHooks = true
				Expect(a.UninstallOption("invalid")(&uninstall)).NotTo(HaveOccurred())
				Expect(uninstall.DisableHooks).To(BeFalse())
			})
		})
	})
})
