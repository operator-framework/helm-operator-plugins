package client_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	. "github.com/joelanford/helm-operator/pkg/client"
	"github.com/joelanford/helm-operator/pkg/internal/testutil"
)

var _ = Describe("ActionClient", func() {
	var (
		rm meta.RESTMapper
	)
	BeforeEach(func() {
		var err error
		rm, err = apiutil.NewDiscoveryRESTMapper(cfg)
		Expect(err).To(BeNil())
	})
	var _ = Describe("NewActionClientGetter", func() {
		It("should return a valid ActionConfigGetter", func() {
			actionConfigGetter := NewActionConfigGetter(cfg, rm, nil)
			Expect(NewActionClientGetter(actionConfigGetter)).NotTo(BeNil())
		})
	})

	var _ = Describe("GetActionClient", func() {
		var obj Object
		BeforeEach(func() {
			obj = testutil.BuildTestCR(gvk)
		})
		It("should return a valid ActionClient", func() {
			acg := NewActionClientGetter(NewActionConfigGetter(cfg, rm, nil))
			ac, err := acg.ActionClientFor(obj)
			Expect(err).To(BeNil())
			Expect(ac).NotTo(BeNil())
		})
	})

	var _ = Describe("ActionClient methods", func() {
		var (
			obj  Object
			cl   client.Client
			ac   ActionInterface
			vals chartutil.Values
		)
		BeforeEach(func() {
			obj = testutil.BuildTestCR(gvk)
			obj.SetUID(types.UID("test-uid"))

			var err error
			actionConfigGetter := NewActionConfigGetter(cfg, rm, nil)
			acg := NewActionClientGetter(actionConfigGetter)
			ac, err = acg.ActionClientFor(obj)
			Expect(err).To(BeNil())

			cl, err = client.New(cfg, client.Options{})
			Expect(err).To(BeNil())

			Expect(cl.Create(context.TODO(), obj)).To(Succeed())
		})

		AfterEach(func() {
			Expect(cl.Delete(context.TODO(), obj)).To(Succeed())
		})

		When("release is not installed", func() {
			AfterEach(func() {
				if _, err := ac.Get(obj.GetName()); err == driver.ErrReleaseNotFound {
					return
				}
				resp, err := ac.Uninstall(obj.GetName())
				Expect(err).To(BeNil())
				Expect(resp).NotTo(BeNil())
			})
			var _ = Describe("Install", func() {
				It("should succeed", func() {
					r, err := ac.Install(obj.GetName(), obj.GetNamespace(), &chrt, vals)
					Expect(err).To(BeNil())
					Expect(r).NotTo(BeNil())
				})
			})
			var _ = Describe("Upgrade", func() {
				It("should fail", func() {
					r, err := ac.Upgrade(obj.GetName(), obj.GetNamespace(), &chrt, vals)
					Expect(err).NotTo(BeNil())
					Expect(r).To(BeNil())
				})
			})
			var _ = Describe("Uninstall", func() {
				It("should fail", func() {
					resp, err := ac.Uninstall(obj.GetName())
					Expect(err).NotTo(BeNil())
					Expect(resp).To(BeNil())
				})
			})
		})

		When("release is installed", func() {
			var (
				installedRelease *release.Release
			)
			BeforeEach(func() {
				var err error
				installedRelease, err = ac.Install(obj.GetName(), obj.GetNamespace(), &chrt, vals)
				Expect(err).To(BeNil())
				Expect(installedRelease).NotTo(BeNil())
			})
			AfterEach(func() {
				if _, err := ac.Get(obj.GetName()); err == driver.ErrReleaseNotFound {
					return
				}
				resp, err := ac.Uninstall(obj.GetName())
				Expect(err).To(BeNil())
				Expect(resp).NotTo(BeNil())
			})

			var _ = Describe("Get", func() {
				It("should succeed", func() {
					r, err := ac.Get(obj.GetName())
					Expect(err).To(BeNil())
					Expect(r).NotTo(BeNil())
				})
			})
			var _ = Describe("Install", func() {
				It("should fail", func() {
					r, err := ac.Install(obj.GetName(), obj.GetNamespace(), &chrt, vals)
					Expect(err).NotTo(BeNil())
					Expect(r).To(BeNil())
				})
			})
			var _ = Describe("Upgrade", func() {
				It("should succeed", func() {
					r, err := ac.Upgrade(obj.GetName(), obj.GetNamespace(), &chrt, vals)
					Expect(err).To(BeNil())
					Expect(r).NotTo(BeNil())
				})
			})
			var _ = Describe("Uninstall", func() {
				It("should succeed", func() {
					resp, err := ac.Uninstall(obj.GetName())
					Expect(err).To(BeNil())
					Expect(resp).NotTo(BeNil())
				})
			})
			var _ = Describe("Reconcile", func() {
				It("should succeed", func() {
					err := ac.Reconcile(installedRelease)
					Expect(err).To(BeNil())
				})
			})
		})
	})
})
