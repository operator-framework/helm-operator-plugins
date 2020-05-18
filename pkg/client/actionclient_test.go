package client

import (
	"context"
	"errors"
	"strconv"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/releaseutil"
	"helm.sh/helm/v3/pkg/storage/driver"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/yaml"

	"github.com/joelanford/helm-operator/pkg/internal/testutil"
)

var _ = Describe("ActionClient", func() {
	var (
		rm meta.RESTMapper
	)
	BeforeEach(func() {
		var err error
		rm, err = apiutil.NewDynamicRESTMapper(cfg)
		Expect(err).To(BeNil())
	})
	var _ = Describe("NewActionClientGetter", func() {
		It("should return a valid ActionConfigGetter", func() {
			actionConfigGetter := NewActionConfigGetter(cfg, rm, nil)
			Expect(NewActionClientGetter(actionConfigGetter)).NotTo(BeNil())
		})
	})

	var _ = Describe("ActionClientFor", func() {
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
			vals = chartutil.Values{"service": map[string]interface{}{"type": "NodePort"}}
		)
		BeforeEach(func() {
			obj = testutil.BuildTestCR(gvk)

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
				_, err := ac.Uninstall(obj.GetName())
				if err != nil {
					panic(err)
				}
			})
			var _ = Describe("Install", func() {
				It("should succeed", func() {
					var (
						rel *release.Release
						err error
					)
					By("installing the release", func() {
						opt := func(i *action.Install) error { i.Description = "Test Description"; return nil }
						rel, err = ac.Install(obj.GetName(), obj.GetNamespace(), &chrt, vals, opt)
						Expect(err).To(BeNil())
						Expect(rel).NotTo(BeNil())
					})
					verifyRelease(cl, obj.GetNamespace(), rel)
				})
				It("should uninstall a failed install", func() {
					By("failing to install the release", func() {
						chrt := testutil.MustLoadChart("../../testdata/test-chart-0.1.0.tgz")
						chrt.Templates[2].Data = append(chrt.Templates[2].Data, []byte("\ngibberish")...)
						r, err := ac.Install(obj.GetName(), obj.GetNamespace(), &chrt, vals)
						Expect(err).NotTo(BeNil())
						Expect(r).To(BeNil())
					})
					verifyNoRelease(cl, obj.GetNamespace(), obj.GetName(), nil)
				})
				When("using an option function that returns an error", func() {
					It("should fail", func() {
						opt := func(*action.Install) error { return errors.New("expect this error") }
						r, err := ac.Install(obj.GetName(), obj.GetNamespace(), &chrt, vals, opt)
						Expect(err).To(MatchError("expect this error"))
						Expect(r).To(BeNil())
					})
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
				opt := func(i *action.Install) error { i.Description = "Test Description"; return nil }
				installedRelease, err = ac.Install(obj.GetName(), obj.GetNamespace(), &chrt, vals, opt)
				Expect(err).To(BeNil())
				Expect(installedRelease).NotTo(BeNil())
			})
			AfterEach(func() {
				if _, err := ac.Get(obj.GetName()); err == driver.ErrReleaseNotFound {
					return
				}
				_, err := ac.Uninstall(obj.GetName())
				if err != nil {
					panic(err)
				}
			})
			var _ = Describe("Get", func() {
				var (
					rel *release.Release
					err error
				)
				It("should succeed", func() {
					By("getting the release", func() {
						rel, err = ac.Get(obj.GetName())
						Expect(err).To(BeNil())
						Expect(rel).NotTo(BeNil())
					})
					verifyRelease(cl, obj.GetNamespace(), rel)
				})
				When("using an option function that returns an error", func() {
					It("should fail", func() {
						opt := func(*action.Get) error { return errors.New("expect this error") }
						rel, err = ac.Get(obj.GetName(), opt)
						Expect(err).To(MatchError("expect this error"))
						Expect(rel).To(BeNil())
					})
				})
				When("setting the version option", func() {
					It("should succeed with an existing version", func() {
						opt := func(g *action.Get) error { g.Version = 1; return nil }
						rel, err = ac.Get(obj.GetName(), opt)
						Expect(err).To(BeNil())
						Expect(rel).NotTo(BeNil())
					})
					It("should fail with a non-existent version", func() {
						opt := func(g *action.Get) error { g.Version = 10; return nil }
						rel, err = ac.Get(obj.GetName(), opt)
						Expect(err).NotTo(BeNil())
						Expect(rel).To(BeNil())
					})
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
					var (
						rel *release.Release
						err error
					)
					By("upgrading the release", func() {
						opt := func(u *action.Upgrade) error { u.Description = "Test Description"; return nil }
						rel, err = ac.Upgrade(obj.GetName(), obj.GetNamespace(), &chrt, vals, opt)
						Expect(err).To(BeNil())
						Expect(rel).NotTo(BeNil())
					})
					verifyRelease(cl, obj.GetNamespace(), rel)
				})
				It("should rollback a failed upgrade", func() {
					By("failing to install the release", func() {
						vals = chartutil.Values{"service": map[string]interface{}{"type": "ClusterIP"}}
						r, err := ac.Upgrade(obj.GetName(), obj.GetNamespace(), &chrt, vals)
						Expect(err).NotTo(BeNil())
						Expect(r).To(BeNil())
					})
					tmp := *installedRelease
					rollbackRelease := &tmp
					rollbackRelease.Version = installedRelease.Version + 2
					verifyRelease(cl, obj.GetNamespace(), rollbackRelease)
				})
				When("using an option function that returns an error", func() {
					It("should fail", func() {
						opt := func(*action.Upgrade) error { return errors.New("expect this error") }
						r, err := ac.Upgrade(obj.GetName(), obj.GetNamespace(), &chrt, vals, opt)
						Expect(err).To(MatchError("expect this error"))
						Expect(r).To(BeNil())
					})
				})
			})
			var _ = Describe("Uninstall", func() {
				It("should succeed", func() {
					var (
						resp *release.UninstallReleaseResponse
						err  error
					)
					By("uninstalling the release", func() {
						opt := func(i *action.Uninstall) error { i.Description = "Test Description"; return nil }
						resp, err = ac.Uninstall(obj.GetName(), opt)
						Expect(err).To(BeNil())
						Expect(resp).NotTo(BeNil())
					})
					verifyNoRelease(cl, obj.GetNamespace(), obj.GetName(), resp.Release)
				})
				When("using an option function that returns an error", func() {
					It("should fail", func() {
						opt := func(*action.Uninstall) error { return errors.New("expect this error") }
						r, err := ac.Uninstall(obj.GetName(), opt)
						Expect(err).To(MatchError("expect this error"))
						Expect(r).To(BeNil())
					})
				})
			})
			var _ = Describe("Reconcile", func() {
				It("should succeed", func() {
					By("reconciling the release", func() {
						err := ac.Reconcile(installedRelease)
						Expect(err).To(BeNil())
					})
					verifyRelease(cl, obj.GetNamespace(), installedRelease)
				})
				It("should re-create deleted resources", func() {
					By("deleting the manifest resources", func() {
						objs := manifestToObjects(installedRelease.Manifest)
						for _, obj := range objs {
							err := cl.Delete(context.TODO(), obj)
							Expect(err).To(BeNil())
						}
					})
					By("reconciling the release", func() {
						err := ac.Reconcile(installedRelease)
						Expect(err).To(BeNil())
					})
					verifyRelease(cl, obj.GetNamespace(), installedRelease)
				})
				It("should patch changed resources", func() {
					By("changing manifest resources", func() {
						objs := manifestToObjects(installedRelease.Manifest)
						for _, obj := range objs {
							key, err := client.ObjectKeyFromObject(obj)
							Expect(err).To(BeNil())

							u := &unstructured.Unstructured{}
							u.SetGroupVersionKind(obj.GetObjectKind().GroupVersionKind())
							err = cl.Get(context.TODO(), key, u)
							Expect(err).To(BeNil())

							labels := u.GetLabels()
							labels["app.kubernetes.io/managed-by"] = "Unmanaged"
							u.SetLabels(labels)

							err = cl.Update(context.TODO(), u)
							Expect(err).To(BeNil())
						}
					})
					By("reconciling the release", func() {
						err := ac.Reconcile(installedRelease)
						Expect(err).To(BeNil())
					})
					verifyRelease(cl, obj.GetNamespace(), installedRelease)
				})
			})
		})
	})
})

func manifestToObjects(manifest string) []runtime.Object {
	objs := []runtime.Object{}
	for _, m := range releaseutil.SplitManifests(manifest) {
		u := &unstructured.Unstructured{}
		err := yaml.Unmarshal([]byte(m), u)
		Expect(err).To(BeNil())
		objs = append(objs, u)
	}
	return objs
}

func verifyRelease(cl client.Client, ns string, rel *release.Release) {
	By("verifying release secret exists at release version", func() {
		releaseSecrets := &v1.SecretList{}
		err := cl.List(context.TODO(), releaseSecrets, client.InNamespace(ns), client.MatchingLabels{"owner": "helm", "name": rel.Name})
		Expect(err).To(BeNil())
		Expect(releaseSecrets.Items).To(HaveLen(rel.Version))
		Expect(releaseSecrets.Items[rel.Version-1].Type).To(Equal(v1.SecretType("helm.sh/release.v1")))
		Expect(releaseSecrets.Items[rel.Version-1].Labels["version"]).To(Equal(strconv.Itoa(rel.Version)))
		Expect(releaseSecrets.Items[rel.Version-1].Data["release"]).NotTo(BeNil())
	})

	By("verifying release status description option was honored", func() {
		Expect(rel.Info.Description).To(Equal("Test Description"))
	})

	By("verifying the release resources exist", func() {
		objs := manifestToObjects(rel.Manifest)
		for _, obj := range objs {
			key, err := client.ObjectKeyFromObject(obj)
			Expect(err).To(BeNil())

			err = cl.Get(context.TODO(), key, obj)
			Expect(err).To(BeNil())
		}
	})
}

func verifyNoRelease(cl client.Client, ns string, name string, rel *release.Release) {
	By("verifying all release secrets are removed", func() {
		releaseSecrets := &v1.SecretList{}
		err := cl.List(context.TODO(), releaseSecrets, client.InNamespace(ns), client.MatchingLabels{"owner": "helm", "name": name})
		Expect(err).To(BeNil())
		Expect(releaseSecrets.Items).To(HaveLen(0))
	})
	By("verifying the uninstall description option was honored", func() {
		if rel != nil {
			Expect(rel.Info.Description).To(Equal("Test Description"))
		}
	})
	By("verifying all release resources are removed", func() {
		if rel != nil {
			for _, r := range releaseutil.SplitManifests(rel.Manifest) {
				u := &unstructured.Unstructured{}
				err := yaml.Unmarshal([]byte(r), u)
				Expect(err).To(BeNil())

				key, err := client.ObjectKeyFromObject(u)
				Expect(err).To(BeNil())

				err = cl.Get(context.TODO(), key, u)
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}
		}
	})
}
