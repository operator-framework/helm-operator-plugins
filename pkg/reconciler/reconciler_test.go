package reconciler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/joelanford/helm-operator/pkg/annotation"
	helmclient "github.com/joelanford/helm-operator/pkg/client"
	"github.com/joelanford/helm-operator/pkg/hook"
	"github.com/joelanford/helm-operator/pkg/internal/sdk/controllerutil"
	"github.com/joelanford/helm-operator/pkg/internal/sdk/status"
	"github.com/joelanford/helm-operator/pkg/internal/testutil"
	"github.com/joelanford/helm-operator/pkg/reconciler/internal/conditions"
	helmfake "github.com/joelanford/helm-operator/pkg/reconciler/internal/fake"
	"github.com/joelanford/helm-operator/pkg/reconciler/internal/values"
)

var _ = Describe("Reconciler", func() {
	var (
		r *Reconciler
	)

	BeforeEach(func() {
		r = &Reconciler{}
	})
	var _ = Describe("New", func() {
		It("should fail without a GVK", func() {
			r, err := New(WithChart(chart.Chart{}))
			Expect(r).To(BeNil())
			Expect(err).NotTo(BeNil())
		})
		It("should fail without a chart", func() {
			r, err := New(WithGroupVersionKind(schema.GroupVersionKind{}))
			Expect(r).To(BeNil())
			Expect(err).NotTo(BeNil())
		})
		It("should succeed with just a GVK and chart", func() {
			r, err := New(WithChart(chart.Chart{}), WithGroupVersionKind(schema.GroupVersionKind{}))
			Expect(r).NotTo(BeNil())
			Expect(err).To(BeNil())
		})
		It("should return an error if an option func fails", func() {
			r, err := New(func(r *Reconciler) error { return errors.New("expect this error") })
			Expect(r).To(BeNil())
			Expect(err).To(MatchError("expect this error"))
		})
	})

	var _ = PDescribe("SetupWithManager", func() {

	})

	var _ = Describe("Option", func() {
		var _ = Describe("WithClient", func() {
			It("should set the reconciler client", func() {
				client := fake.NewFakeClientWithScheme(scheme.Scheme)
				Expect(WithClient(client)(r)).To(Succeed())
				Expect(r.client).To(Equal(client))
			})
		})
		var _ = Describe("WithActionClientGetter", func() {
			It("should set the reconciler action client getter", func() {
				cfgGetter := helmclient.NewActionConfigGetter(nil, nil, nil)
				acg := helmclient.NewActionClientGetter(cfgGetter)
				Expect(WithActionClientGetter(acg)(r)).To(Succeed())
				Expect(r.actionClientGetter).To(Equal(acg))
			})
		})
		var _ = Describe("WithEventRecorder", func() {
			It("should set the reconciler event recorder", func() {
				rec := record.NewFakeRecorder(0)
				Expect(WithEventRecorder(rec)(r)).To(Succeed())
				Expect(r.eventRecorder).To(Equal(rec))
			})
		})
		var _ = Describe("WithLog", func() {
			It("should set the reconciler log", func() {
				log := testing.TestLogger{}
				Expect(WithLog(log)(r)).To(Succeed())
				Expect(r.log).To(Equal(log))
			})
		})
		var _ = Describe("WithGroupVersionKind", func() {
			It("should set the reconciler GVK", func() {
				gvk := schema.GroupVersionKind{Group: "mygroup", Version: "v1", Kind: "MyApp"}
				Expect(WithGroupVersionKind(gvk)(r)).To(Succeed())
				Expect(r.gvk).To(Equal(&gvk))
			})
		})
		var _ = Describe("WithChart", func() {
			It("should set the reconciler chart", func() {
				chrt := chart.Chart{Metadata: &chart.Metadata{Name: "my-chart"}}
				Expect(WithChart(chrt)(r)).To(Succeed())
				Expect(r.chrt).To(Equal(&chrt))
			})
		})
		var _ = Describe("WithOverrideValues", func() {
			It("should succeed with valid overrides", func() {
				overrides := map[string]string{"foo": "bar"}
				Expect(WithOverrideValues(overrides)(r)).To(Succeed())
				Expect(r.overrideValues).To(Equal(overrides))
			})

			It("should fail with invalid overrides", func() {
				overrides := map[string]string{"foo[": "bar"}
				Expect(WithOverrideValues(overrides)(r)).NotTo(Succeed())
			})
		})
		var _ = Describe("SkipDependentWatches", func() {
			It("should set to false", func() {
				Expect(SkipDependentWatches(false)(r)).To(Succeed())
				Expect(r.skipDependentWatches).To(Equal(false))
			})
			It("should set to true", func() {
				Expect(SkipDependentWatches(true)(r)).To(Succeed())
				Expect(r.skipDependentWatches).To(Equal(true))
			})
		})
		var _ = Describe("WithMaxConcurrentReconciles", func() {
			It("should set the reconciler max concurrent reconciled", func() {
				Expect(WithMaxConcurrentReconciles(1)(r)).To(Succeed())
				Expect(r.maxConcurrentReconciles).To(Equal(1))
			})
			It("should fail if value is less than 1", func() {
				Expect(WithMaxConcurrentReconciles(0)(r)).NotTo(Succeed())
				Expect(WithMaxConcurrentReconciles(-1)(r)).NotTo(Succeed())
			})
		})
		var _ = Describe("WithReconcilePeriod", func() {
			It("should set the reconciler reconcile period", func() {
				Expect(WithReconcilePeriod(0)(r)).To(Succeed())
				Expect(r.reconcilePeriod).To(Equal(time.Duration(0)))
			})
			It("should fail if value is less than 0", func() {
				Expect(WithReconcilePeriod(-time.Nanosecond)(r)).NotTo(Succeed())
			})
		})
		var _ = Describe("WithInstallAnnotation", func() {
			It("should set multiple reconciler install annotations", func() {
				a1 := annotation.InstallDisableHooks{CustomName: "my.domain/custom-name1"}
				a2 := annotation.InstallDisableHooks{CustomName: "my.domain/custom-name2"}
				Expect(WithInstallAnnotation(a1)(r)).To(Succeed())
				Expect(WithInstallAnnotation(a2)(r)).To(Succeed())
				Expect(r.annotations).To(Equal(map[string]struct{}{
					"my.domain/custom-name1": struct{}{},
					"my.domain/custom-name2": struct{}{},
				}))
				Expect(r.installAnnotations).To(Equal(map[string]annotation.Install{
					"my.domain/custom-name1": a1,
					"my.domain/custom-name2": a2,
				}))
			})
			It("should error with duplicate install annotation", func() {
				a1 := annotation.InstallDisableHooks{CustomName: "my.domain/custom-name1"}
				a2 := annotation.InstallDisableHooks{CustomName: "my.domain/custom-name1"}
				Expect(WithInstallAnnotation(a1)(r)).To(Succeed())
				Expect(WithInstallAnnotation(a2)(r)).To(HaveOccurred())
				Expect(r.annotations).To(Equal(map[string]struct{}{
					"my.domain/custom-name1": struct{}{},
				}))
				Expect(r.installAnnotations).To(Equal(map[string]annotation.Install{
					"my.domain/custom-name1": a1,
				}))
			})
			It("should error with duplicate upgrade annotation", func() {
				a1 := annotation.InstallDisableHooks{CustomName: "my.domain/custom-name1"}
				a2 := annotation.UpgradeDisableHooks{CustomName: "my.domain/custom-name1"}
				Expect(WithInstallAnnotation(a1)(r)).To(Succeed())
				Expect(WithUpgradeAnnotation(a2)(r)).To(HaveOccurred())
				Expect(r.annotations).To(Equal(map[string]struct{}{
					"my.domain/custom-name1": struct{}{},
				}))
				Expect(r.installAnnotations).To(Equal(map[string]annotation.Install{
					"my.domain/custom-name1": a1,
				}))
			})
			It("should error with duplicate uninstall annotation", func() {
				a1 := annotation.InstallDisableHooks{CustomName: "my.domain/custom-name1"}
				a2 := annotation.UninstallDisableHooks{CustomName: "my.domain/custom-name1"}
				Expect(WithInstallAnnotation(a1)(r)).To(Succeed())
				Expect(WithUninstallAnnotation(a2)(r)).To(HaveOccurred())
				Expect(r.annotations).To(Equal(map[string]struct{}{
					"my.domain/custom-name1": struct{}{},
				}))
				Expect(r.installAnnotations).To(Equal(map[string]annotation.Install{
					"my.domain/custom-name1": a1,
				}))
			})
		})
		var _ = Describe("WithUpgradeAnnotation", func() {
			It("should set multiple reconciler upgrade annotations", func() {
				a1 := annotation.UpgradeDisableHooks{CustomName: "my.domain/custom-name1"}
				a2 := annotation.UpgradeDisableHooks{CustomName: "my.domain/custom-name2"}
				Expect(WithUpgradeAnnotation(a1)(r)).To(Succeed())
				Expect(WithUpgradeAnnotation(a2)(r)).To(Succeed())
				Expect(r.annotations).To(Equal(map[string]struct{}{
					"my.domain/custom-name1": struct{}{},
					"my.domain/custom-name2": struct{}{},
				}))
				Expect(r.upgradeAnnotations).To(Equal(map[string]annotation.Upgrade{
					"my.domain/custom-name1": a1,
					"my.domain/custom-name2": a2,
				}))
			})
			It("should error with duplicate install annotation", func() {
				a1 := annotation.UpgradeDisableHooks{CustomName: "my.domain/custom-name1"}
				a2 := annotation.InstallDisableHooks{CustomName: "my.domain/custom-name1"}
				Expect(WithUpgradeAnnotation(a1)(r)).To(Succeed())
				Expect(WithInstallAnnotation(a2)(r)).To(HaveOccurred())
				Expect(r.annotations).To(Equal(map[string]struct{}{
					"my.domain/custom-name1": struct{}{},
				}))
				Expect(r.upgradeAnnotations).To(Equal(map[string]annotation.Upgrade{
					"my.domain/custom-name1": a1,
				}))
			})
			It("should error with duplicate upgrade annotation", func() {
				a1 := annotation.UpgradeDisableHooks{CustomName: "my.domain/custom-name1"}
				a2 := annotation.UpgradeDisableHooks{CustomName: "my.domain/custom-name1"}
				Expect(WithUpgradeAnnotation(a1)(r)).To(Succeed())
				Expect(WithUpgradeAnnotation(a2)(r)).To(HaveOccurred())
				Expect(r.annotations).To(Equal(map[string]struct{}{
					"my.domain/custom-name1": struct{}{},
				}))
				Expect(r.upgradeAnnotations).To(Equal(map[string]annotation.Upgrade{
					"my.domain/custom-name1": a1,
				}))
			})
			It("should error with duplicate uninstall annotation", func() {
				a1 := annotation.UpgradeDisableHooks{CustomName: "my.domain/custom-name1"}
				a2 := annotation.UninstallDisableHooks{CustomName: "my.domain/custom-name1"}
				Expect(WithUpgradeAnnotation(a1)(r)).To(Succeed())
				Expect(WithUninstallAnnotation(a2)(r)).To(HaveOccurred())
				Expect(r.annotations).To(Equal(map[string]struct{}{
					"my.domain/custom-name1": struct{}{},
				}))
				Expect(r.upgradeAnnotations).To(Equal(map[string]annotation.Upgrade{
					"my.domain/custom-name1": a1,
				}))
			})
		})
		var _ = Describe("WithUninstallAnnotation", func() {
			It("should set multiple reconciler uninstall annotations", func() {
				a1 := annotation.UninstallDisableHooks{CustomName: "my.domain/custom-name1"}
				a2 := annotation.UninstallDisableHooks{CustomName: "my.domain/custom-name2"}
				Expect(WithUninstallAnnotation(a1)(r)).To(Succeed())
				Expect(WithUninstallAnnotation(a2)(r)).To(Succeed())
				Expect(r.annotations).To(Equal(map[string]struct{}{
					"my.domain/custom-name1": struct{}{},
					"my.domain/custom-name2": struct{}{},
				}))
				Expect(r.uninstallAnnotations).To(Equal(map[string]annotation.Uninstall{
					"my.domain/custom-name1": a1,
					"my.domain/custom-name2": a2,
				}))
			})
			It("should error with duplicate install annotation", func() {
				a1 := annotation.UninstallDisableHooks{CustomName: "my.domain/custom-name1"}
				a2 := annotation.InstallDisableHooks{CustomName: "my.domain/custom-name1"}
				Expect(WithUninstallAnnotation(a1)(r)).To(Succeed())
				Expect(WithInstallAnnotation(a2)(r)).To(HaveOccurred())
				Expect(r.annotations).To(Equal(map[string]struct{}{
					"my.domain/custom-name1": struct{}{},
				}))
				Expect(r.uninstallAnnotations).To(Equal(map[string]annotation.Uninstall{
					"my.domain/custom-name1": a1,
				}))
			})
			It("should error with duplicate uninstall annotation", func() {
				a1 := annotation.UninstallDisableHooks{CustomName: "my.domain/custom-name1"}
				a2 := annotation.UpgradeDisableHooks{CustomName: "my.domain/custom-name1"}
				Expect(WithUninstallAnnotation(a1)(r)).To(Succeed())
				Expect(WithUpgradeAnnotation(a2)(r)).To(HaveOccurred())
				Expect(r.annotations).To(Equal(map[string]struct{}{
					"my.domain/custom-name1": struct{}{},
				}))
				Expect(r.uninstallAnnotations).To(Equal(map[string]annotation.Uninstall{
					"my.domain/custom-name1": a1,
				}))
			})
			It("should error with duplicate uninstall annotation", func() {
				a1 := annotation.UninstallDisableHooks{CustomName: "my.domain/custom-name1"}
				a2 := annotation.UninstallDisableHooks{CustomName: "my.domain/custom-name1"}
				Expect(WithUninstallAnnotation(a1)(r)).To(Succeed())
				Expect(WithUninstallAnnotation(a2)(r)).To(HaveOccurred())
				Expect(r.annotations).To(Equal(map[string]struct{}{
					"my.domain/custom-name1": struct{}{},
				}))
				Expect(r.uninstallAnnotations).To(Equal(map[string]annotation.Uninstall{
					"my.domain/custom-name1": a1,
				}))
			})
		})
		var _ = Describe("WithPreHook", func() {
			It("should set a reconciler prehook", func() {
				called := false
				preHook := hook.PreHookFunc(func(*unstructured.Unstructured, *chartutil.Values, logr.Logger) error {
					called = true
					return nil
				})
				Expect(WithPreHook(preHook)(r)).To(Succeed())
				Expect(r.preHooks).To(HaveLen(1))
				Expect(r.preHooks[0].Exec(nil, nil, nil)).To(Succeed())
				Expect(called).To(BeTrue())
			})
		})
		var _ = Describe("WithPostHook", func() {
			It("should set a reconciler posthook", func() {
				called := false
				postHook := hook.PostHookFunc(func(*unstructured.Unstructured, *release.Release, logr.Logger) error {
					called = true
					return nil
				})
				Expect(WithPostHook(postHook)(r)).To(Succeed())
				Expect(r.postHooks).To(HaveLen(1))
				Expect(r.postHooks[0].Exec(nil, nil, nil)).To(Succeed())
				Expect(called).To(BeTrue())
			})
		})
		var _ = Describe("WithValueMapper", func() {
			It("should set the reconciler value mapper", func() {
				mapper := values.MapperFunc(func(chartutil.Values) chartutil.Values {
					return chartutil.Values{"mapped": true}
				})
				Expect(WithValueMapper(mapper)(r)).To(Succeed())
				Expect(r.valueMapper).NotTo(BeNil())
				Expect(r.valueMapper.Map(chartutil.Values{})).To(Equal(chartutil.Values{"mapped": true}))
			})
		})
	})

	var _ = Describe("Reconcile", func() {
		var (
			obj             *unstructured.Unstructured
			objKey          types.NamespacedName
			req             reconcile.Request
			mgr             manager.Manager
			actionClient    helmfake.ActionClient
			reconcilePeriod time.Duration
			preHookCalled   bool
			postHookCalled  bool
			done            chan struct{}
		)

		BeforeEach(func() {
			mgr = getManagerOrFail()
			valueMapper := values.MapperFunc(func(vals chartutil.Values) chartutil.Values {
				if v, ok := vals["replicas"]; ok {
					vals["replicaCount"] = v
					delete(vals, "replicas")
				}
				return vals
			})

			actionClient = helmfake.NewActionClient()
			preHookCalled = false
			postHookCalled = false

			var err error
			r, err = New(
				WithGroupVersionKind(gvk),
				WithChart(chrt),
				WithValueMapper(valueMapper),
				WithActionClientGetter(helmfake.NewActionClientGetter(&actionClient, nil)),
				WithReconcilePeriod(reconcilePeriod),
				WithPreHook(hook.PreHookFunc(func(*unstructured.Unstructured, *chartutil.Values, logr.Logger) error {
					preHookCalled = true
					return nil
				})),
				WithPostHook(hook.PostHookFunc(func(*unstructured.Unstructured, *release.Release, logr.Logger) error {
					postHookCalled = true
					return nil
				})),
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(r.SetupWithManager(mgr)).NotTo(HaveOccurred())

			obj = testutil.BuildTestCR(gvk)
			objKey = types.NamespacedName{Namespace: obj.GetNamespace(), Name: obj.GetName()}
			req = reconcile.Request{NamespacedName: objKey}

		})

		JustBeforeEach(func() {
			done = make(chan struct{})
			go func() { Expect(mgr.GetCache().Start(done)) }()
			Expect(mgr.GetCache().WaitForCacheSync(done)).To(BeTrue())
		})

		AfterEach(func() {
			close(done)
		})

		When("requested CR is not found", func() {
			It("should return with no action", func() {
				res, err := r.Reconcile(req)
				Expect(res).To(Equal(reconcile.Result{}))
				Expect(err).NotTo(HaveOccurred())
				Expect(actionClient.Gets).To(HaveLen(0))
			})
		})

		When("requested CR is found", func() {
			BeforeEach(func() {
				err := mgr.GetClient().Create(context.TODO(), obj)
				Expect(err).NotTo(HaveOccurred())
			})
			AfterEach(func() {
				err := mgr.GetClient().Get(context.TODO(), objKey, obj)
				if apierrors.IsNotFound(err) {
					return
				}
				Expect(err).NotTo(HaveOccurred())
				controllerutil.RemoveFinalizer(obj, "uninstall-helm-release")
				err = mgr.GetClient().Update(context.TODO(), obj)
				Expect(err).NotTo(HaveOccurred())
				err = mgr.GetClient().Delete(context.TODO(), obj)
				Expect(err).NotTo(HaveOccurred())
				err = controllerutil.WaitForDeletion(context.TODO(), mgr.GetClient(), obj)
				Expect(err).NotTo(HaveOccurred())
			})

			When("actionClientGetter returns an error", func() {
				var acgErr = errors.New("failed to get action client")
				BeforeEach(func() {
					r.actionClientGetter = helmfake.NewActionClientGetter(nil, acgErr)
				})
				It("should fail reconciliation", func() {
					By("returning an error", func() {
						res, err := r.Reconcile(req)
						Expect(res).To(Equal(reconcile.Result{}))
						Expect(err).To(HaveOccurred())
					})

					Expect(mgr.GetClient().Get(context.TODO(), objKey, obj)).To(Succeed())

					By("ensuring the correct conditions are set on the CR", func() {
						objStat := &objStatus{}
						Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, objStat)).To(Succeed())
						Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeIrreconcilable)).To(BeTrue())
						Expect(objStat.Status.Conditions.IsUnknownFor(conditions.TypeInitialized)).To(BeTrue())
						Expect(objStat.Status.Conditions.IsUnknownFor(conditions.TypeDeployed)).To(BeTrue())
						Expect(objStat.Status.Conditions.IsUnknownFor(conditions.TypeReleaseFailed)).To(BeTrue())
					})

					By("ensuring the uninstall finalizer is not preset on the CR", func() {
						Expect(controllerutil.ContainsFinalizer(obj, "uninstall-helm-release")).To(BeFalse())
					})
				})
			})

			When("there is an error getting values", func() {
				BeforeEach(func() {
					r.overrideValues = map[string]string{"r[": "foobar"}
				})
				It("should fail reconciliation", func() {
					By("returning an error", func() {
						res, err := r.Reconcile(req)
						Expect(res).To(Equal(reconcile.Result{}))
						Expect(err).To(HaveOccurred())
					})

					Expect(mgr.GetClient().Get(context.TODO(), objKey, obj)).To(Succeed())

					By("ensuring the correct conditions are set on the CR", func() {
						objStat := &objStatus{}
						Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, objStat)).To(Succeed())
						Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeIrreconcilable)).To(BeTrue())
						Expect(objStat.Status.Conditions.IsUnknownFor(conditions.TypeInitialized)).To(BeTrue())
						Expect(objStat.Status.Conditions.IsUnknownFor(conditions.TypeDeployed)).To(BeTrue())
						Expect(objStat.Status.Conditions.IsUnknownFor(conditions.TypeReleaseFailed)).To(BeTrue())
					})

					By("ensuring the uninstall finalizer is not preset on the CR", func() {
						Expect(controllerutil.ContainsFinalizer(obj, "uninstall-helm-release")).To(BeFalse())
					})
				})
			})

			When("there is an error getting release state", func() {
				BeforeEach(func() {
					actionClient.HandleGet = func() (*release.Release, error) {
						return nil, errors.New("unexpected error")
					}
				})
				It("should fail reconciliation", func() {
					By("returning an error", func() {
						res, err := r.Reconcile(req)
						Expect(res).To(Equal(reconcile.Result{}))
						Expect(err).To(HaveOccurred())
					})

					Expect(mgr.GetClient().Get(context.TODO(), objKey, obj)).To(Succeed())

					By("ensuring the correct conditions are set on the CR", func() {
						objStat := &objStatus{}
						Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, objStat)).To(Succeed())
						Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeIrreconcilable)).To(BeTrue())
						Expect(objStat.Status.Conditions.IsUnknownFor(conditions.TypeInitialized)).To(BeTrue())
						Expect(objStat.Status.Conditions.IsUnknownFor(conditions.TypeDeployed)).To(BeTrue())
						Expect(objStat.Status.Conditions.IsUnknownFor(conditions.TypeReleaseFailed)).To(BeTrue())
					})

					By("ensuring the uninstall finalizer is not preset on the CR", func() {
						Expect(controllerutil.ContainsFinalizer(obj, "uninstall-helm-release")).To(BeFalse())
					})
				})
			})

			When("requested CR release is not installed", func() {
				BeforeEach(func() {
					actionClient.HandleGet = func() (*release.Release, error) {
						return nil, driver.ErrReleaseNotFound
					}
				})
				When("release installation fails", func() {
					BeforeEach(func() {
						actionClient.HandleInstall = func() (*release.Release, error) {
							return nil, errors.New("install failed")
						}
					})
					It("should fail reconciliation", func() {
						By("returning an error", func() {
							res, err := r.Reconcile(req)
							Expect(res).To(Equal(reconcile.Result{}))
							Expect(err).To(HaveOccurred())
						})

						Expect(mgr.GetClient().Get(context.TODO(), objKey, obj)).To(Succeed())

						By("ensuring the correct conditions are set on the CR", func() {
							objStat := &objStatus{}
							Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, objStat)).To(Succeed())
							Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeIrreconcilable)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeInitialized)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsUnknownFor(conditions.TypeDeployed)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeReleaseFailed)).To(BeTrue())
						})

						By("ensuring the uninstall finalizer is not preset on the CR", func() {
							Expect(controllerutil.ContainsFinalizer(obj, "uninstall-helm-release")).To(BeFalse())
						})
					})
				})
				When("release installation succeeds", func() {
					BeforeEach(func() {
						actionClient.HandleInstall = func() (*release.Release, error) {
							return getRelease(obj.GetName(), 1), nil
						}
					})
					It("should install the release", func() {
						By("successfully reconciling a request", func() {
							res, err := r.Reconcile(req)
							Expect(res).To(Equal(reconcile.Result{}))
							Expect(err).NotTo(HaveOccurred())
							Expect(actionClient.Installs).To(HaveLen(1))
						})

						By("running pre-hooks", func() {
							Expect(preHookCalled).To(BeTrue())
						})

						By("doing an installation", func() {
							Expect(actionClient.Installs[0].Name).To(Equal(obj.GetName()))
							Expect(actionClient.Installs[0].Namespace).To(Equal(obj.GetNamespace()))
							Expect(actionClient.Installs[0].Chart).To(Equal(&chrt))
							Expect(actionClient.Installs[0].Values).To(HaveKey("replicaCount"))
							Expect(actionClient.Installs[0].Opts).To(HaveLen(0))
						})

						By("running post-hooks", func() {
							Expect(postHookCalled).To(BeTrue())
						})

						Expect(mgr.GetClient().Get(context.TODO(), objKey, obj)).To(Succeed())

						By("ensuring the uninstall finalizer is present", func() {
							Expect(obj.GetFinalizers()).To(ContainElement("uninstall-helm-release"))
						})

						By("ensuring the correct conditions are set on the CR", func() {
							objStat := &objStatus{}
							Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, objStat)).To(Succeed())
							Expect(objStat.Status.Conditions.IsFalseFor(conditions.TypeIrreconcilable)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeInitialized)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeDeployed)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsFalseFor(conditions.TypeReleaseFailed)).To(BeTrue())
						})
					})
				})

			})

			When("requested CR release is not upgraded", func() {
				BeforeEach(func() {
					actionClient.HandleGet = func() (*release.Release, error) {
						return getRelease(obj.GetName(), 1), nil
					}
				})
				When("release upgrade fails", func() {
					BeforeEach(func() {
						fail := false
						actionClient.HandleUpgrade = func() (*release.Release, error) {
							if fail {
								return nil, errors.New("upgrade failed")
							}
							fail = true
							return getRelease(obj.GetName(), 2), nil
						}
					})
					It("should fail reconciliation", func() {
						By("returning an error", func() {
							res, err := r.Reconcile(req)
							Expect(res).To(Equal(reconcile.Result{}))
							Expect(err).To(HaveOccurred())
						})

						Expect(mgr.GetClient().Get(context.TODO(), objKey, obj)).To(Succeed())

						By("ensuring the correct conditions are set on the CR", func() {
							objStat := &objStatus{}
							Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, objStat)).To(Succeed())
							Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeIrreconcilable)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeInitialized)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeDeployed)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeReleaseFailed)).To(BeTrue())
						})

						By("ensuring the uninstall finalizer is not preset on the CR", func() {
							Expect(controllerutil.ContainsFinalizer(obj, "uninstall-helm-release")).To(BeFalse())
						})
					})
				})

				When("release upgrade succeeds", func() {
					BeforeEach(func() {
						actionClient.HandleUpgrade = func() (*release.Release, error) {
							return getRelease(obj.GetName(), 2), nil
						}
					})
					It("should upgrade the release", func() {
						By("successfully reconciling a request", func() {
							res, err := r.Reconcile(req)
							Expect(res).To(Equal(reconcile.Result{}))
							Expect(err).NotTo(HaveOccurred())
							Expect(actionClient.Upgrades).To(HaveLen(2))
						})

						By("doing a dry run upgrade", func() {
							Expect(actionClient.Upgrades[0].Name).To(Equal(obj.GetName()))
							Expect(actionClient.Upgrades[0].Namespace).To(Equal(obj.GetNamespace()))
							Expect(actionClient.Upgrades[0].Chart).To(Equal(&chrt))
							Expect(actionClient.Upgrades[0].Values).To(HaveKey("replicaCount"))
							Expect(actionClient.Upgrades[0].Opts).To(HaveLen(1))

							u := action.Upgrade{}
							Expect(actionClient.Upgrades[0].Opts[0](&u)).To(Succeed())
							Expect(u.DryRun).To(BeTrue())
						})

						By("doing an actual upgrade", func() {
							Expect(actionClient.Upgrades[1].Name).To(Equal(obj.GetName()))
							Expect(actionClient.Upgrades[1].Namespace).To(Equal(obj.GetNamespace()))
							Expect(actionClient.Upgrades[1].Chart).To(Equal(&chrt))
							Expect(actionClient.Upgrades[1].Values).To(HaveKey("replicaCount"))
							Expect(actionClient.Upgrades[1].Opts).To(HaveLen(0))
						})

						Expect(mgr.GetClient().Get(context.TODO(), objKey, obj)).To(Succeed())

						By("ensuring the uninstall finalizer is present", func() {
							Expect(obj.GetFinalizers()).To(ContainElement("uninstall-helm-release"))
						})

						By("ensuring the correct conditions are set on the CR", func() {
							objStat := &objStatus{}
							Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, objStat)).To(Succeed())
							Expect(objStat.Status.Conditions.IsFalseFor(conditions.TypeIrreconcilable)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeInitialized)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeDeployed)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsFalseFor(conditions.TypeReleaseFailed)).To(BeTrue())
						})
					})
				})
			})

			When("requested CR release is not reconciled", func() {
				var currentRelease *release.Release

				BeforeEach(func() {
					currentRelease = getRelease(obj.GetName(), 1)
					actionClient.HandleGet = func() (*release.Release, error) {
						return currentRelease, nil
					}
					actionClient.HandleUpgrade = func() (*release.Release, error) {
						return currentRelease, nil
					}
				})
				When("reconciling the release fails", func() {
					BeforeEach(func() {
						actionClient.HandleReconcile = func() error {
							return errors.New("reconcile failed")
						}
					})
					It("should fail reconciliation", func() {
						By("returning an error", func() {
							res, err := r.Reconcile(req)
							Expect(res).To(Equal(reconcile.Result{}))
							Expect(err).To(HaveOccurred())
						})

						Expect(mgr.GetClient().Get(context.TODO(), objKey, obj)).To(Succeed())

						By("ensuring the correct conditions are set on the CR", func() {
							objStat := &objStatus{}
							Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, objStat)).To(Succeed())
							Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeIrreconcilable)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeInitialized)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeDeployed)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsFalseFor(conditions.TypeReleaseFailed)).To(BeTrue())
						})

						By("ensuring the uninstall finalizer is not preset on the CR", func() {
							Expect(controllerutil.ContainsFinalizer(obj, "uninstall-helm-release")).To(BeFalse())
						})
					})
				})
				When("reconciling the release succeeds", func() {
					BeforeEach(func() {
						actionClient.HandleReconcile = func() error {
							return nil
						}
					})
					It("should reconcile the release", func() {

						By("successfully reconciling a request", func() {
							res, err := r.Reconcile(req)
							Expect(res).To(Equal(reconcile.Result{}))
							Expect(err).NotTo(HaveOccurred())
						})

						By("doing a dry run upgrade", func() {
							Expect(actionClient.Upgrades).To(HaveLen(1))
							Expect(actionClient.Upgrades[0].Name).To(Equal(obj.GetName()))
							Expect(actionClient.Upgrades[0].Namespace).To(Equal(obj.GetNamespace()))
							Expect(actionClient.Upgrades[0].Chart).To(Equal(&chrt))
							Expect(actionClient.Upgrades[0].Values).To(HaveKey("replicaCount"))
							Expect(actionClient.Upgrades[0].Opts).To(HaveLen(1))

							u := action.Upgrade{}
							Expect(actionClient.Upgrades[0].Opts[0](&u)).To(Succeed())
							Expect(u.DryRun).To(BeTrue())
						})

						By("doing a reconciliation", func() {
							Expect(actionClient.Reconciles).To(HaveLen(1))
							Expect(actionClient.Reconciles[0].Release).To(Equal(currentRelease))
						})

						Expect(mgr.GetClient().Get(context.TODO(), objKey, obj)).To(Succeed())

						By("ensuring the uninstall finalizer is present", func() {
							Expect(obj.GetFinalizers()).To(ContainElement("uninstall-helm-release"))
						})

						By("ensuring the correct conditions are set on the CR", func() {
							objStat := &objStatus{}
							Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, objStat)).To(Succeed())
							Expect(objStat.Status.Conditions.IsFalseFor(conditions.TypeIrreconcilable)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeInitialized)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeDeployed)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsFalseFor(conditions.TypeReleaseFailed)).To(BeTrue())
						})
					})
				})
			})

			When("requested CR is deleted", func() {
				var currentRelease *release.Release

				BeforeEach(func() {
					controllerutil.AddFinalizer(obj, "uninstall-helm-release")
					Expect(mgr.GetClient().Update(context.TODO(), obj)).To(Succeed())
					Expect(mgr.GetClient().Delete(context.TODO(), obj)).To(Succeed())
					Expect(wait.PollImmediate(time.Millisecond*100, time.Second*10, func() (bool, error) {
						if err := mgr.GetAPIReader().Get(context.TODO(), objKey, obj); err != nil {
							return false, err
						}
						return obj.GetDeletionTimestamp() != nil, nil
					})).To(Succeed())
					currentRelease = getRelease(obj.GetName(), 1)
					actionClient.HandleGet = func() (*release.Release, error) {
						return currentRelease, nil
					}
					actionClient.HandleUninstall = func() (*release.UninstallReleaseResponse, error) {
						return &release.UninstallReleaseResponse{Release: currentRelease}, nil
					}
				})

				It("should uninstall the release and remove the finalizer", func() {
					By("successfully reconciling a request", func() {
						res, err := r.Reconcile(req)
						Expect(res).To(Equal(reconcile.Result{}))
						Expect(err).NotTo(HaveOccurred())
					})

					By("doing an uninstall", func() {
						Expect(actionClient.Uninstalls).To(HaveLen(1))
						Expect(actionClient.Uninstalls[0].Name).To(Equal(obj.GetName()))
					})

					By("ensuring the finalizer is removed and the CR is deleted", func() {
						err := mgr.GetClient().Get(context.TODO(), objKey, obj)
						Expect(apierrors.IsNotFound(err)).To(BeTrue())
					})
				})
			})
		})
	})
})

func getManagerOrFail() manager.Manager {
	mgr, err := manager.New(cfg, manager.Options{
		MetricsBindAddress: "0",
	})
	Expect(err).NotTo(HaveOccurred())
	return mgr
}

func getRelease(name string, version int) *release.Release {
	return &release.Release{
		Name:     name,
		Version:  version,
		Manifest: fmt.Sprintf("v%d manifest", version),
		Info: &release.Info{
			Notes: fmt.Sprintf("v%d notes", version),
		},
	}
}

type objStatus struct {
	Status struct {
		Conditions status.Conditions `json:"conditions"`
	} `json:"status"`
}
