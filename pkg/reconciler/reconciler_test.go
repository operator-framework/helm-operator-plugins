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

package reconciler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/releaseutil"
	"helm.sh/helm/v3/pkg/storage/driver"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	"github.com/operator-framework/helm-operator-plugins/internal/sdk/controllerutil"
	"github.com/operator-framework/helm-operator-plugins/pkg/annotation"
	helmclient "github.com/operator-framework/helm-operator-plugins/pkg/client"
	"github.com/operator-framework/helm-operator-plugins/pkg/extension"
	"github.com/operator-framework/helm-operator-plugins/pkg/hook"
	"github.com/operator-framework/helm-operator-plugins/pkg/internal/status"
	"github.com/operator-framework/helm-operator-plugins/pkg/internal/testutil"
	"github.com/operator-framework/helm-operator-plugins/pkg/reconciler/internal/conditions"
	helmfake "github.com/operator-framework/helm-operator-plugins/pkg/reconciler/internal/fake"
	"github.com/operator-framework/helm-operator-plugins/pkg/values"
)

var _ = Describe("Reconciler", func() {
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

	var _ = Describe("Option", func() {
		var r *Reconciler
		BeforeEach(func() {
			r = &Reconciler{}
		})
		var _ = Describe("WithClient", func() {
			It("should set the reconciler client", func() {
				client := fake.NewClientBuilder().Build()
				Expect(WithClient(client)(r)).To(Succeed())
				Expect(r.client).To(Equal(client))
			})
		})
		var _ = Describe("WithActionClientGetter", func() {
			It("should set the reconciler action client getter", func() {
				cfgGetter := helmclient.NewActionConfigGetter(nil, nil, logr.Discard())
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
				log := logr.Discard()
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
		var _ = Describe("WithMaxReleaseHistory", func() {
			It("should set the max history size", func() {
				Expect(WithMaxReleaseHistory(10)(r)).To(Succeed())
				Expect(r.maxHistory).To(Equal(10))
			})
			It("should allow setting the history to unlimited", func() {
				Expect(WithMaxReleaseHistory(0)(r)).To(Succeed())
				Expect(r.maxHistory).To(Equal(0))
			})
			It("should fail if value is less than 0", func() {
				Expect(WithMaxReleaseHistory(-1)(r)).NotTo(Succeed())
			})
		})
		var _ = Describe("WithInstallAnnotations", func() {
			It("should set multiple reconciler install annotations", func() {
				a1 := annotation.InstallDisableHooks{CustomName: "my.domain/custom-name1"}
				a2 := annotation.InstallDisableHooks{CustomName: "my.domain/custom-name2"}
				Expect(WithInstallAnnotations(a1, a2)(r)).To(Succeed())
				Expect(r.annotations).To(Equal(map[string]struct{}{
					"my.domain/custom-name1": {},
					"my.domain/custom-name2": {},
				}))
				Expect(r.installAnnotations).To(Equal(map[string]annotation.Install{
					"my.domain/custom-name1": a1,
					"my.domain/custom-name2": a2,
				}))
			})
			It("should error with duplicate install annotation", func() {
				a1 := annotation.InstallDisableHooks{CustomName: "my.domain/custom-name1"}
				a2 := annotation.InstallDisableHooks{CustomName: "my.domain/custom-name1"}
				Expect(WithInstallAnnotations(a1, a2)(r)).NotTo(Succeed())
				Expect(r.annotations).To(Equal(map[string]struct{}{
					"my.domain/custom-name1": {},
				}))
				Expect(r.installAnnotations).To(Equal(map[string]annotation.Install{
					"my.domain/custom-name1": a1,
				}))
			})
			It("should error with duplicate upgrade annotation", func() {
				a1 := annotation.InstallDisableHooks{CustomName: "my.domain/custom-name1"}
				a2 := annotation.UpgradeDisableHooks{CustomName: "my.domain/custom-name1"}
				Expect(WithInstallAnnotations(a1)(r)).To(Succeed())
				Expect(WithUpgradeAnnotations(a2)(r)).To(HaveOccurred())
				Expect(r.annotations).To(Equal(map[string]struct{}{
					"my.domain/custom-name1": {},
				}))
				Expect(r.installAnnotations).To(Equal(map[string]annotation.Install{
					"my.domain/custom-name1": a1,
				}))
			})
			It("should error with duplicate uninstall annotation", func() {
				a1 := annotation.InstallDisableHooks{CustomName: "my.domain/custom-name1"}
				a2 := annotation.UninstallDisableHooks{CustomName: "my.domain/custom-name1"}
				Expect(WithInstallAnnotations(a1)(r)).To(Succeed())
				Expect(WithUninstallAnnotations(a2)(r)).To(HaveOccurred())
				Expect(r.annotations).To(Equal(map[string]struct{}{
					"my.domain/custom-name1": {},
				}))
				Expect(r.installAnnotations).To(Equal(map[string]annotation.Install{
					"my.domain/custom-name1": a1,
				}))
			})
		})
		var _ = Describe("WithUpgradeAnnotations", func() {
			It("should set multiple reconciler upgrade annotations", func() {
				a1 := annotation.UpgradeDisableHooks{CustomName: "my.domain/custom-name1"}
				a2 := annotation.UpgradeDisableHooks{CustomName: "my.domain/custom-name2"}
				Expect(WithUpgradeAnnotations(a1, a2)(r)).To(Succeed())
				Expect(r.annotations).To(Equal(map[string]struct{}{
					"my.domain/custom-name1": {},
					"my.domain/custom-name2": {},
				}))
				Expect(r.upgradeAnnotations).To(Equal(map[string]annotation.Upgrade{
					"my.domain/custom-name1": a1,
					"my.domain/custom-name2": a2,
				}))
			})
			It("should error with duplicate install annotation", func() {
				a1 := annotation.UpgradeDisableHooks{CustomName: "my.domain/custom-name1"}
				a2 := annotation.InstallDisableHooks{CustomName: "my.domain/custom-name1"}
				Expect(WithUpgradeAnnotations(a1)(r)).To(Succeed())
				Expect(WithInstallAnnotations(a2)(r)).To(HaveOccurred())
				Expect(r.annotations).To(Equal(map[string]struct{}{
					"my.domain/custom-name1": {},
				}))
				Expect(r.upgradeAnnotations).To(Equal(map[string]annotation.Upgrade{
					"my.domain/custom-name1": a1,
				}))
			})
			It("should error with duplicate upgrade annotation", func() {
				a1 := annotation.UpgradeDisableHooks{CustomName: "my.domain/custom-name1"}
				a2 := annotation.UpgradeDisableHooks{CustomName: "my.domain/custom-name1"}
				Expect(WithUpgradeAnnotations(a1, a2)(r)).NotTo(Succeed())
				Expect(r.annotations).To(Equal(map[string]struct{}{
					"my.domain/custom-name1": {},
				}))
				Expect(r.upgradeAnnotations).To(Equal(map[string]annotation.Upgrade{
					"my.domain/custom-name1": a1,
				}))
			})
			It("should error with duplicate uninstall annotation", func() {
				a1 := annotation.UpgradeDisableHooks{CustomName: "my.domain/custom-name1"}
				a2 := annotation.UninstallDisableHooks{CustomName: "my.domain/custom-name1"}
				Expect(WithUpgradeAnnotations(a1)(r)).To(Succeed())
				Expect(WithUninstallAnnotations(a2)(r)).To(HaveOccurred())
				Expect(r.annotations).To(Equal(map[string]struct{}{
					"my.domain/custom-name1": {},
				}))
				Expect(r.upgradeAnnotations).To(Equal(map[string]annotation.Upgrade{
					"my.domain/custom-name1": a1,
				}))
			})
		})
		var _ = Describe("WithUninstallAnnotations", func() {
			It("should set multiple reconciler uninstall annotations", func() {
				a1 := annotation.UninstallDisableHooks{CustomName: "my.domain/custom-name1"}
				a2 := annotation.UninstallDisableHooks{CustomName: "my.domain/custom-name2"}
				Expect(WithUninstallAnnotations(a1, a2)(r)).To(Succeed())
				Expect(r.annotations).To(Equal(map[string]struct{}{
					"my.domain/custom-name1": {},
					"my.domain/custom-name2": {},
				}))
				Expect(r.uninstallAnnotations).To(Equal(map[string]annotation.Uninstall{
					"my.domain/custom-name1": a1,
					"my.domain/custom-name2": a2,
				}))
			})
			It("should error with duplicate install annotation", func() {
				a1 := annotation.UninstallDisableHooks{CustomName: "my.domain/custom-name1"}
				a2 := annotation.InstallDisableHooks{CustomName: "my.domain/custom-name1"}
				Expect(WithUninstallAnnotations(a1)(r)).To(Succeed())
				Expect(WithInstallAnnotations(a2)(r)).To(HaveOccurred())
				Expect(r.annotations).To(Equal(map[string]struct{}{
					"my.domain/custom-name1": {},
				}))
				Expect(r.uninstallAnnotations).To(Equal(map[string]annotation.Uninstall{
					"my.domain/custom-name1": a1,
				}))
			})
			It("should error with duplicate uninstall annotation", func() {
				a1 := annotation.UninstallDisableHooks{CustomName: "my.domain/custom-name1"}
				a2 := annotation.UpgradeDisableHooks{CustomName: "my.domain/custom-name1"}
				Expect(WithUninstallAnnotations(a1)(r)).To(Succeed())
				Expect(WithUpgradeAnnotations(a2)(r)).To(HaveOccurred())
				Expect(r.annotations).To(Equal(map[string]struct{}{
					"my.domain/custom-name1": {},
				}))
				Expect(r.uninstallAnnotations).To(Equal(map[string]annotation.Uninstall{
					"my.domain/custom-name1": a1,
				}))
			})
			It("should error with duplicate uninstall annotation", func() {
				a1 := annotation.UninstallDisableHooks{CustomName: "my.domain/custom-name1"}
				a2 := annotation.UninstallDisableHooks{CustomName: "my.domain/custom-name1"}
				Expect(WithUninstallAnnotations(a1, a2)(r)).NotTo(Succeed())
				Expect(r.annotations).To(Equal(map[string]struct{}{
					"my.domain/custom-name1": {},
				}))
				Expect(r.uninstallAnnotations).To(Equal(map[string]annotation.Uninstall{
					"my.domain/custom-name1": a1,
				}))
			})
		})
		var _ = Describe("WithPreHook", func() {
			It("should set a reconciler prehook", func() {
				called := false
				preHook := hook.PreHookFunc(func(context.Context, *unstructured.Unstructured, logr.Logger) error {
					called = true
					return nil
				})
				nExtensions := len(r.extensions)
				Expect(WithPreHook(preHook)(r)).To(Succeed())
				Expect(len(r.extensions)).To(Equal(nExtensions + 1))
				hook := r.extensions[nExtensions].(extension.BeginReconciliationExtension)
				Expect(hook.BeginReconcile(context.TODO(), nil, nil)).To(Succeed())
				Expect(called).To(BeTrue())
			})
		})
		var _ = Describe("WithPostHook", func() {
			It("should set a reconciler posthook", func() {
				called := false
				postHook := hook.PostHookFunc(func(context.Context, *unstructured.Unstructured, release.Release, chartutil.Values, logr.Logger) error {
					called = true
					return nil
				})
				nExtensions := len(r.extensions)
				Expect(WithPostHook(postHook)(r)).To(Succeed())
				Expect(len(r.extensions)).To(Equal(nExtensions + 1))
				hook := r.extensions[nExtensions].(extension.EndReconciliationExtension)
				Expect(hook.EndReconcile(context.TODO(), nil, nil)).To(Succeed())
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
		var _ = Describe("WithValueTranslator", func() {
			It("should set the reconciler value translator", func() {
				translator := values.TranslatorFunc(func(ctx context.Context, u *unstructured.Unstructured) (chartutil.Values, error) {
					return chartutil.Values{"translated": true}, nil
				})
				Expect(WithValueTranslator(translator)(r)).To(Succeed())
				Expect(r.valueTranslator).NotTo(BeNil())
				Expect(r.valueTranslator.Translate(context.Background(), &unstructured.Unstructured{})).To(Equal(chartutil.Values{"translated": true}))
			})
		})
		var _ = Describe("WithSelector", func() {
			It("should set the reconciler selector", func() {
				objUnlabeled := &unstructured.Unstructured{}

				objLabeled := &unstructured.Unstructured{}
				objLabeled.SetLabels(map[string]string{"foo": "bar"})

				selector := metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}}
				Expect(WithSelector(selector)(r)).To(Succeed())
				Expect(r.selectorPredicate).NotTo(BeNil())

				Expect(r.selectorPredicate.Create(event.CreateEvent{Object: objLabeled})).To(BeTrue())
				Expect(r.selectorPredicate.Update(event.UpdateEvent{ObjectOld: objUnlabeled, ObjectNew: objLabeled})).To(BeTrue())
				Expect(r.selectorPredicate.Delete(event.DeleteEvent{Object: objLabeled})).To(BeTrue())
				Expect(r.selectorPredicate.Generic(event.GenericEvent{Object: objLabeled})).To(BeTrue())

				Expect(r.selectorPredicate.Create(event.CreateEvent{Object: objUnlabeled})).To(BeFalse())
				Expect(r.selectorPredicate.Update(event.UpdateEvent{ObjectOld: objLabeled, ObjectNew: objUnlabeled})).To(BeFalse())
				Expect(r.selectorPredicate.Update(event.UpdateEvent{ObjectOld: objUnlabeled, ObjectNew: objUnlabeled})).To(BeFalse())
				Expect(r.selectorPredicate.Delete(event.DeleteEvent{Object: objUnlabeled})).To(BeFalse())
				Expect(r.selectorPredicate.Generic(event.GenericEvent{Object: objUnlabeled})).To(BeFalse())
			})
		})
	})

	var _ = Describe("Reconcile", func() {
		var (
			obj    *unstructured.Unstructured
			objKey types.NamespacedName
			req    reconcile.Request

			mgr    manager.Manager
			ctx    context.Context
			cancel context.CancelFunc

			r  *Reconciler
			ac helmclient.ActionInterface
		)

		BeforeEach(func() {
			mgr = getManagerOrFail()
			ctx, cancel = context.WithCancel(context.Background())
			go func() { Expect(mgr.GetCache().Start(ctx)) }()
			Expect(mgr.GetCache().WaitForCacheSync(ctx)).To(BeTrue())

			obj = testutil.BuildTestCR(gvk)
			objKey = types.NamespacedName{Namespace: obj.GetNamespace(), Name: obj.GetName()}
			req = reconcile.Request{NamespacedName: objKey}

			var err error
			r, err = New(
				WithGroupVersionKind(gvk),
				WithChart(chrt),
				WithInstallAnnotations(annotation.InstallDescription{}),
				WithUpgradeAnnotations(annotation.UpgradeDescription{}),
				WithUninstallAnnotations(annotation.UninstallDescription{}),
				WithOverrideValues(map[string]string{
					"image.repository": "custom-nginx",
				}),
			)
			Expect(err).To(BeNil())
			Expect(r.SetupWithManager(mgr)).To(Succeed())

			ac, err = r.actionClientGetter.ActionClientFor(obj)
			Expect(err).To(BeNil())
		})

		AfterEach(func() {
			By("ensuring the release is uninstalled", func() {
				if _, err := ac.Get(obj.GetName()); err == driver.ErrReleaseNotFound {
					return
				}
				_, err := ac.Uninstall(obj.GetName())
				if err != nil {
					panic(err)
				}
			})

			By("ensuring the CR is deleted", func() {
				err := mgr.GetAPIReader().Get(ctx, objKey, obj)
				if apierrors.IsNotFound(err) {
					return
				}
				Expect(err).To(BeNil())
				obj.SetFinalizers([]string{})
				Expect(mgr.GetClient().Update(ctx, obj)).To(Succeed())
				err = mgr.GetClient().Delete(ctx, obj)
				if apierrors.IsNotFound(err) {
					return
				}
				Expect(err).To(BeNil())
			})
			cancel()
		})

		When("an extension fails", func() {
			It("subsequent extensions are not executed", func() {
				var (
					failingPreReconciliationExtCalled    bool
					succeedingPreReconciliationExtCalled bool
				)

				failingPreReconciliationExt := &testBeginReconcileExtension{f: func() error {
					failingPreReconciliationExtCalled = true
					return errors.New("error!")
				}}

				succeedingPreReconciliationExt := &testBeginReconcileExtension{f: func() error {
					succeedingPreReconciliationExtCalled = true
					return nil
				}}

				r.extensions = append(r.extensions, failingPreReconciliationExt)
				r.extensions = append(r.extensions, succeedingPreReconciliationExt)

				err := r.extBeginReconcile(ctx, nil, &unstructured.Unstructured{})
				Expect(err).To(HaveOccurred())
				Expect(failingPreReconciliationExtCalled).To(BeTrue())
				Expect(succeedingPreReconciliationExtCalled).To(BeFalse())
			})
		})

		When("requested CR is not found", func() {
			It("returns successfully with no action", func() {
				res, err := r.Reconcile(ctx, req)
				Expect(res).To(Equal(reconcile.Result{}))
				Expect(err).To(BeNil())

				rel, err := ac.Get(obj.GetName())
				Expect(err).To(Equal(driver.ErrReleaseNotFound))
				Expect(rel).To(BeNil())

				err = mgr.GetAPIReader().Get(ctx, objKey, obj)
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
			})
		})

		When("requested CR is found", func() {
			BeforeEach(func() {
				Expect(mgr.GetClient().Create(ctx, obj)).To(Succeed())
			})

			When("requested CR release is not present", func() {
				When("action client getter is not working", func() {
					It("returns an error getting the action client", func() {
						acgErr := errors.New("broken action client getter: error getting action client")

						By("creating a reconciler with a broken action client getter", func() {
							r.actionClientGetter = helmclient.ActionClientGetterFunc(func(client.Object) (helmclient.ActionInterface, error) {
								return nil, acgErr
							})
						})

						By("reconciling unsuccessfully", func() {
							res, err := r.Reconcile(ctx, req)
							Expect(res).To(Equal(reconcile.Result{}))
							Expect(err).To(MatchError(acgErr))
						})

						By("getting the CR", func() {
							Expect(mgr.GetAPIReader().Get(ctx, objKey, obj)).To(Succeed())
						})

						By("verifying the CR status", func() {
							objStat := &objStatus{}
							Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, objStat)).To(Succeed())
							Expect(objStat.Status.Conditions.IsUnknownFor(conditions.TypeInitialized)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeIrreconcilable)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsUnknownFor(conditions.TypeDeployed)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsUnknownFor(conditions.TypeReleaseFailed)).To(BeTrue())
							Expect(objStat.Status.DeployedRelease).To(BeNil())

							c := objStat.Status.Conditions.GetCondition(conditions.TypeIrreconcilable)
							Expect(c).NotTo(BeNil())
							Expect(c.Reason).To(Equal(conditions.ReasonErrorGettingClient))
							Expect(c.Message).To(Equal(acgErr.Error()))
						})

						By("verifying the uninstall finalizer is not present on the CR", func() {
							Expect(controllerutil.ContainsFinalizer(obj, uninstallFinalizer)).To(BeFalse())
						})
					})
					It("returns an error getting the release", func() {
						By("creating a reconciler with a broken action client getter", func() {
							r.actionClientGetter = helmclient.ActionClientGetterFunc(func(client.Object) (helmclient.ActionInterface, error) {
								cl := helmfake.NewActionClient()
								return &cl, nil
							})
						})

						By("reconciling unsuccessfully", func() {
							res, err := r.Reconcile(ctx, req)
							Expect(res).To(Equal(reconcile.Result{}))
							Expect(err).To(MatchError("get not implemented"))
						})

						By("getting the CR", func() {
							Expect(mgr.GetAPIReader().Get(ctx, objKey, obj)).To(Succeed())
						})

						By("verifying the CR status", func() {
							objStat := &objStatus{}
							Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, objStat)).To(Succeed())
							Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeInitialized)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeIrreconcilable)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsUnknownFor(conditions.TypeDeployed)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsUnknownFor(conditions.TypeReleaseFailed)).To(BeTrue())
							Expect(objStat.Status.DeployedRelease).To(BeNil())

							c := objStat.Status.Conditions.GetCondition(conditions.TypeIrreconcilable)
							Expect(c).NotTo(BeNil())
							Expect(c.Reason).To(Equal(conditions.ReasonErrorGettingReleaseState))
							Expect(c.Message).To(Equal("get not implemented"))
						})

						By("verifying the uninstall finalizer is not present on the CR", func() {
							Expect(controllerutil.ContainsFinalizer(obj, uninstallFinalizer)).To(BeFalse())
						})
					})
				})
				When("override values are invalid", func() {
					BeforeEach(func() {
						r.overrideValues = map[string]string{"r[": "foobar"}
					})
					It("returns an error", func() {
						By("reconciling unsuccessfully", func() {
							res, err := r.Reconcile(ctx, req)
							Expect(res).To(Equal(reconcile.Result{}))
							Expect(err).ToNot(BeNil())
							Expect(err.Error()).To(ContainSubstring("error parsing index"))
						})

						By("getting the CR", func() {
							Expect(mgr.GetAPIReader().Get(ctx, objKey, obj)).To(Succeed())
						})

						By("verifying the CR status", func() {
							objStat := &objStatus{}
							Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, objStat)).To(Succeed())
							Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeInitialized)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeIrreconcilable)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsFalseFor(conditions.TypeDeployed)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsUnknownFor(conditions.TypeReleaseFailed)).To(BeTrue())
							Expect(objStat.Status.DeployedRelease).To(BeNil())

							c := objStat.Status.Conditions.GetCondition(conditions.TypeIrreconcilable)
							Expect(c).NotTo(BeNil())
							Expect(c.Reason).To(Equal(conditions.ReasonErrorGettingValues))
							Expect(c.Message).To(ContainSubstring("error parsing index"))
						})

						By("verifying the uninstall finalizer is not present on the CR", func() {
							Expect(controllerutil.ContainsFinalizer(obj, uninstallFinalizer)).To(BeFalse())
						})
					})
				})
				When("CR is deleted, release is not present, but uninstall finalizer exists", func() {
					It("removes the finalizer", func() {
						By("adding the uninstall finalizer and deleting the CR", func() {
							obj.SetFinalizers([]string{uninstallFinalizer})
							Expect(mgr.GetClient().Update(ctx, obj)).To(Succeed())
							Expect(mgr.GetClient().Delete(ctx, obj)).To(Succeed())
						})

						By("successfully reconciling a request", func() {
							res, err := r.Reconcile(ctx, req)
							Expect(res).To(Equal(reconcile.Result{}))
							Expect(err).To(BeNil())
						})

						By("ensuring the finalizer is removed and the CR is deleted", func() {
							err := mgr.GetAPIReader().Get(ctx, objKey, obj)
							Expect(apierrors.IsNotFound(err)).To(BeTrue())
						})
					})
				})
				When("all install preconditions met", func() {
					When("installation fails", func() {
						BeforeEach(func() {
							ac := helmfake.NewActionClient()
							ac.HandleGet = func() (*release.Release, error) {
								return nil, driver.ErrReleaseNotFound
							}
							ac.HandleInstall = func() (*release.Release, error) {
								return nil, errors.New("install failed: foobar")
							}
							r.actionClientGetter = helmfake.NewActionClientGetter(&ac, nil)
						})
						It("handles the installation error", func() {
							By("returning an error", func() {
								res, err := r.Reconcile(ctx, req)
								Expect(res).To(Equal(reconcile.Result{}))
								Expect(err).To(HaveOccurred())
							})

							By("getting the CR", func() {
								Expect(mgr.GetAPIReader().Get(ctx, objKey, obj)).To(Succeed())
							})

							By("ensuring the correct conditions are set on the CR", func() {
								objStat := &objStatus{}
								Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, objStat)).To(Succeed())
								Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeInitialized)).To(BeTrue())
								Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeIrreconcilable)).To(BeTrue())
								Expect(objStat.Status.Conditions.IsFalseFor(conditions.TypeDeployed)).To(BeTrue())
								Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeReleaseFailed)).To(BeTrue())

								c := objStat.Status.Conditions.GetCondition(conditions.TypeReleaseFailed)
								Expect(c).NotTo(BeNil())
								Expect(c.Reason).To(Equal(conditions.ReasonInstallError))
								Expect(c.Message).To(ContainSubstring("install failed: foobar"))

								c = objStat.Status.Conditions.GetCondition(conditions.TypeIrreconcilable)
								Expect(c).NotTo(BeNil())
								Expect(c.Reason).To(Equal(conditions.ReasonReconcileError))
								Expect(c.Message).To(ContainSubstring("install failed: foobar"))
							})

							By("ensuring the uninstall finalizer is not present on the CR", func() {
								Expect(controllerutil.ContainsFinalizer(obj, uninstallFinalizer)).To(BeFalse())
							})
						})
					})
					When("installation succeeds", func() {
						It("installs the release", func() {
							var (
								rel *release.Release
								err error
							)
							By("successfully reconciling a request", func() {
								res, err := r.Reconcile(ctx, req)
								Expect(err).To(BeNil())
								Expect(res).To(Equal(reconcile.Result{}))
							})

							By("getting the release and CR", func() {
								rel, err = ac.Get(obj.GetName())
								Expect(err).To(BeNil())
								Expect(rel).NotTo(BeNil())
								Expect(mgr.GetAPIReader().Get(ctx, objKey, obj)).To(Succeed())
							})

							By("verifying the release", func() {
								Expect(rel.Version).To(Equal(1))
								verifyRelease(ctx, mgr.GetClient(), obj.GetNamespace(), rel)
							})

							By("verifying override event", func() {
								verifyEvent(ctx, mgr.GetAPIReader(), obj,
									"Warning",
									"ValueOverridden",
									`Chart value "image.repository" overridden to "custom-nginx" by operator`)
							})

							By("ensuring the uninstall finalizer is present", func() {
								Expect(obj.GetFinalizers()).To(ContainElement(uninstallFinalizer))
							})

							By("verifying the CR status", func() {
								objStat := &objStatus{}
								Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, objStat)).To(Succeed())
								Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeInitialized)).To(BeTrue())
								Expect(objStat.Status.Conditions.IsFalseFor(conditions.TypeIrreconcilable)).To(BeTrue())
								Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeDeployed)).To(BeTrue())
								Expect(objStat.Status.Conditions.IsFalseFor(conditions.TypeReleaseFailed)).To(BeTrue())
								Expect(objStat.Status.DeployedRelease.Name).To(Equal(obj.GetName()))
								Expect(objStat.Status.DeployedRelease.Manifest).To(Equal(rel.Manifest))
							})
						})
						It("calls pre and post hooks", func() {
							verifyHooksCalled(ctx, r, req)
						})
					})
				})
			})
			When("requested CR release is present", func() {
				var (
					currentRelease *release.Release
				)
				BeforeEach(func() {
					// Reconcile once to get the release installed and finalizers added
					var err error
					res, err := r.Reconcile(ctx, req)
					Expect(res).To(Equal(reconcile.Result{}))
					Expect(err).To(BeNil())

					currentRelease, err = ac.Get(obj.GetName())
					Expect(err).To(BeNil())
				})
				When("action client getter is not working", func() {
					It("returns an error getting the action client", func() {
						acgErr := errors.New("broken action client getter: error getting action client")

						By("creating a reconciler with a broken action client getter", func() {
							r.actionClientGetter = helmclient.ActionClientGetterFunc(func(client.Object) (helmclient.ActionInterface, error) {
								return nil, acgErr
							})
						})

						By("reconciling unsuccessfully", func() {
							res, err := r.Reconcile(ctx, req)
							Expect(res).To(Equal(reconcile.Result{}))
							Expect(err).To(MatchError(acgErr))
						})

						By("getting the CR", func() {
							Expect(mgr.GetAPIReader().Get(ctx, objKey, obj)).To(Succeed())
						})

						By("verifying the CR status", func() {
							objStat := &objStatus{}
							Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, objStat)).To(Succeed())
							Expect(objStat.Status.Conditions.IsUnknownFor(conditions.TypeInitialized)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeIrreconcilable)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsUnknownFor(conditions.TypeDeployed)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsUnknownFor(conditions.TypeReleaseFailed)).To(BeTrue())
							Expect(objStat.Status.DeployedRelease).To(BeNil())

							c := objStat.Status.Conditions.GetCondition(conditions.TypeIrreconcilable)
							Expect(c).NotTo(BeNil())
							Expect(c.Reason).To(Equal(conditions.ReasonErrorGettingClient))
							Expect(c.Message).To(Equal(acgErr.Error()))
						})

						By("verifying the uninstall finalizer is present on the CR", func() {
							Expect(controllerutil.ContainsFinalizer(obj, uninstallFinalizer)).To(BeTrue())
						})
					})
					It("returns an error getting the release", func() {
						By("creating a reconciler with a broken action client getter", func() {
							r.actionClientGetter = helmclient.ActionClientGetterFunc(func(client.Object) (helmclient.ActionInterface, error) {
								cl := helmfake.NewActionClient()
								return &cl, nil
							})
						})

						By("reconciling unsuccessfully", func() {
							res, err := r.Reconcile(ctx, req)
							Expect(res).To(Equal(reconcile.Result{}))
							Expect(err).To(MatchError("get not implemented"))
						})

						By("getting the CR", func() {
							Expect(mgr.GetAPIReader().Get(ctx, objKey, obj)).To(Succeed())
						})

						By("verifying the CR status", func() {
							objStat := &objStatus{}
							Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, objStat)).To(Succeed())
							Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeInitialized)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeIrreconcilable)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsUnknownFor(conditions.TypeDeployed)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsUnknownFor(conditions.TypeReleaseFailed)).To(BeTrue())
							Expect(objStat.Status.DeployedRelease).To(BeNil())

							c := objStat.Status.Conditions.GetCondition(conditions.TypeIrreconcilable)
							Expect(c).NotTo(BeNil())
							Expect(c.Reason).To(Equal(conditions.ReasonErrorGettingReleaseState))
							Expect(c.Message).To(Equal("get not implemented"))
						})

						By("verifying the uninstall finalizer is present on the CR", func() {
							Expect(controllerutil.ContainsFinalizer(obj, uninstallFinalizer)).To(BeTrue())
						})
					})
				})
				When("override values are invalid", func() {
					BeforeEach(func() {
						r.overrideValues = map[string]string{"r[": "foobar"}
					})
					It("returns an error", func() {
						By("reconciling unsuccessfully", func() {
							res, err := r.Reconcile(ctx, req)
							Expect(res).To(Equal(reconcile.Result{}))
							Expect(err).ToNot(BeNil())
							Expect(err.Error()).To(ContainSubstring("error parsing index"))
						})

						By("getting the CR", func() {
							Expect(mgr.GetAPIReader().Get(ctx, objKey, obj)).To(Succeed())
						})

						By("verifying the CR status", func() {
							objStat := &objStatus{}
							Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, objStat)).To(Succeed())
							Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeInitialized)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeIrreconcilable)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeDeployed)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsUnknownFor(conditions.TypeReleaseFailed)).To(BeTrue())

							c := objStat.Status.Conditions.GetCondition(conditions.TypeIrreconcilable)
							Expect(c).NotTo(BeNil())
							Expect(c.Reason).To(Equal(conditions.ReasonErrorGettingValues))
							Expect(c.Message).To(ContainSubstring("error parsing index"))

							Expect(objStat.Status.DeployedRelease.Name).To(Equal(currentRelease.Name))
							Expect(objStat.Status.DeployedRelease.Manifest).To(Equal(currentRelease.Manifest))
						})

						By("verifying the uninstall finalizer is not present on the CR", func() {
							Expect(controllerutil.ContainsFinalizer(obj, uninstallFinalizer)).To(BeTrue())
						})
					})
				})
				When("value translator fails", func() {
					BeforeEach(func() {
						r.valueTranslator = values.TranslatorFunc(func(ctx context.Context, u *unstructured.Unstructured) (chartutil.Values, error) {
							return nil, errors.New("translation failure")
						})
					})
					It("returns an error", func() {
						By("reconciling unsuccessfully", func() {
							res, err := r.Reconcile(ctx, req)
							Expect(res).To(Equal(reconcile.Result{}))
							Expect(err.Error()).To(ContainSubstring("translation failure"))
						})

						By("getting the CR", func() {
							Expect(mgr.GetAPIReader().Get(ctx, objKey, obj)).To(Succeed())
						})

						By("verifying the CR status", func() {
							objStat := &objStatus{}
							Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, objStat)).To(Succeed())
							Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeInitialized)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeIrreconcilable)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeDeployed)).To(BeTrue())
							Expect(objStat.Status.Conditions.IsUnknownFor(conditions.TypeReleaseFailed)).To(BeTrue())

							c := objStat.Status.Conditions.GetCondition(conditions.TypeIrreconcilable)
							Expect(c).NotTo(BeNil())
							Expect(c.Reason).To(Equal(conditions.ReasonErrorGettingValues))
							Expect(c.Message).To(ContainSubstring("translation failure"))

							Expect(objStat.Status.DeployedRelease.Name).To(Equal(currentRelease.Name))
							Expect(objStat.Status.DeployedRelease.Manifest).To(Equal(currentRelease.Manifest))
						})

						By("verifying the uninstall finalizer is not present on the CR", func() {
							Expect(controllerutil.ContainsFinalizer(obj, uninstallFinalizer)).To(BeTrue())
						})
					})
				})
				When("requested CR release is not deployed", func() {
					var actionConf *action.Configuration
					BeforeEach(func() {
						By("getting the current release and config", func() {
							var err error
							acg := helmclient.NewActionConfigGetter(mgr.GetConfig(), mgr.GetRESTMapper(), logr.Discard())
							actionConf, err = acg.ActionConfigFor(obj)
							Expect(err).To(BeNil())
						})
					})
					When("state is Failed", func() {
						BeforeEach(func() {
							currentRelease.Info.Status = release.StatusFailed
							Expect(actionConf.Releases.Update(currentRelease)).To(Succeed())
						})
						It("upgrades the release", func() {
							By("successfully reconciling a request", func() {
								res, err := r.Reconcile(ctx, req)
								Expect(res).To(Equal(reconcile.Result{}))
								Expect(err).To(BeNil())
							})
							By("verifying the release", func() {
								rel, err := ac.Get(obj.GetName())
								Expect(err).To(BeNil())
								Expect(rel).NotTo(BeNil())
								Expect(rel.Version).To(Equal(2))
								verifyRelease(ctx, mgr.GetAPIReader(), obj.GetNamespace(), rel)
							})
						})
					})
					When("state is Superseded", func() {
						BeforeEach(func() {
							currentRelease.Info.Status = release.StatusSuperseded
							Expect(actionConf.Releases.Update(currentRelease)).To(Succeed())
						})
						It("upgrades the release", func() {
							By("successfully reconciling a request", func() {
								res, err := r.Reconcile(ctx, req)
								Expect(res).To(Equal(reconcile.Result{}))
								Expect(err).To(BeNil())
							})
							By("verifying the release", func() {
								rel, err := ac.Get(obj.GetName())
								Expect(err).To(BeNil())
								Expect(rel).NotTo(BeNil())
								Expect(rel.Version).To(Equal(2))
								verifyRelease(ctx, mgr.GetAPIReader(), obj.GetNamespace(), rel)
							})
						})
					})
				})
				When("state is Deployed", func() {
					When("upgrade fails", func() {
						BeforeEach(func() {
							ac := helmfake.NewActionClient()
							ac.HandleGet = func() (*release.Release, error) {
								return &release.Release{Name: "test", Version: 1, Manifest: "manifest: 1"}, nil
							}
							firstRun := true
							ac.HandleUpgrade = func() (*release.Release, error) {
								if firstRun {
									firstRun = false
									return &release.Release{Name: "test", Version: 1, Manifest: "manifest: 2"}, nil
								}
								return nil, errors.New("upgrade failed: foobar")
							}
							r.actionClientGetter = helmfake.NewActionClientGetter(&ac, nil)
						})
						It("handles the upgrade error", func() {
							By("returning an error", func() {
								res, err := r.Reconcile(ctx, req)
								Expect(res).To(Equal(reconcile.Result{}))
								Expect(err).To(HaveOccurred())
							})

							By("getting the CR", func() {
								Expect(mgr.GetAPIReader().Get(ctx, objKey, obj)).To(Succeed())
							})

							By("ensuring the correct conditions are set on the CR", func() {
								objStat := &objStatus{}
								Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, objStat)).To(Succeed())
								Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeInitialized)).To(BeTrue())
								Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeIrreconcilable)).To(BeTrue())
								Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeDeployed)).To(BeTrue())
								Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeReleaseFailed)).To(BeTrue())
								Expect(objStat.Status.DeployedRelease.Name).To(Equal("test"))
								Expect(objStat.Status.DeployedRelease.Manifest).To(Equal("manifest: 1"))

								c := objStat.Status.Conditions.GetCondition(conditions.TypeReleaseFailed)
								Expect(c).NotTo(BeNil())
								Expect(c.Reason).To(Equal(conditions.ReasonUpgradeError))
								Expect(c.Message).To(ContainSubstring("upgrade failed: foobar"))

								c = objStat.Status.Conditions.GetCondition(conditions.TypeIrreconcilable)
								Expect(c).NotTo(BeNil())
								Expect(c.Reason).To(Equal(conditions.ReasonReconcileError))
								Expect(c.Message).To(ContainSubstring("upgrade failed: foobar"))
							})

							By("ensuring the uninstall finalizer is present on the CR", func() {
								Expect(controllerutil.ContainsFinalizer(obj, uninstallFinalizer)).To(BeTrue())
							})
						})
					})
					When("upgrade succeeds", func() {
						It("upgrades the release", func() {
							var (
								rel *release.Release
								err error
							)
							By("changing the CR", func() {
								Expect(mgr.GetClient().Get(ctx, objKey, obj)).To(Succeed())
								obj.Object["spec"] = map[string]interface{}{"replicaCount": "2"}
								Expect(mgr.GetClient().Update(ctx, obj)).To(Succeed())
							})

							By("successfully reconciling a request", func() {
								res, err := r.Reconcile(ctx, req)
								Expect(res).To(Equal(reconcile.Result{}))
								Expect(err).To(BeNil())
							})

							By("getting the release and CR", func() {
								rel, err = ac.Get(obj.GetName())
								Expect(err).To(BeNil())
								Expect(rel).NotTo(BeNil())
								Expect(mgr.GetAPIReader().Get(ctx, objKey, obj)).To(Succeed())
							})

							By("verifying the release", func() {
								Expect(rel.Version).To(Equal(2))
								verifyRelease(ctx, mgr.GetAPIReader(), obj.GetNamespace(), rel)
							})

							By("verifying override event", func() {
								verifyEvent(ctx, mgr.GetAPIReader(), obj,
									"Warning",
									"ValueOverridden",
									`Chart value "image.repository" overridden to "custom-nginx" by operator`)
							})

							By("ensuring the uninstall finalizer is present", func() {
								Expect(obj.GetFinalizers()).To(ContainElement(uninstallFinalizer))
							})

							By("verifying the CR status", func() {
								objStat := &objStatus{}
								Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, objStat)).To(Succeed())
								Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeInitialized)).To(BeTrue())
								Expect(objStat.Status.Conditions.IsFalseFor(conditions.TypeIrreconcilable)).To(BeTrue())
								Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeDeployed)).To(BeTrue())
								Expect(objStat.Status.Conditions.IsFalseFor(conditions.TypeReleaseFailed)).To(BeTrue())
								Expect(objStat.Status.DeployedRelease.Name).To(Equal(rel.Name))
								Expect(objStat.Status.DeployedRelease.Manifest).To(Equal(rel.Manifest))
							})
						})
					})
					When("reconciliation fails", func() {
						BeforeEach(func() {
							ac := helmfake.NewActionClient()
							ac.HandleGet = func() (*release.Release, error) {
								return &release.Release{Name: "test", Version: 1, Manifest: "manifest: 1", Info: &release.Info{Status: release.StatusDeployed}}, nil
							}
							ac.HandleUpgrade = func() (*release.Release, error) {
								return &release.Release{Name: "test", Version: 2, Manifest: "manifest: 1", Info: &release.Info{Status: release.StatusDeployed}}, nil
							}
							ac.HandleReconcile = func() error {
								return errors.New("reconciliation failed: foobar")
							}
							r.actionClientGetter = helmfake.NewActionClientGetter(&ac, nil)
						})
						It("handles the reconciliation error", func() {
							By("returning an error", func() {
								res, err := r.Reconcile(ctx, req)
								Expect(res).To(Equal(reconcile.Result{}))
								Expect(err).To(HaveOccurred())
							})

							By("getting the CR", func() {
								Expect(mgr.GetAPIReader().Get(ctx, objKey, obj)).To(Succeed())
							})

							By("ensuring the correct conditions are set on the CR", func() {
								objStat := &objStatus{}
								Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, objStat)).To(Succeed())
								Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeInitialized)).To(BeTrue())
								Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeIrreconcilable)).To(BeTrue())
								Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeDeployed)).To(BeTrue())
								Expect(objStat.Status.Conditions.IsFalseFor(conditions.TypeReleaseFailed)).To(BeTrue())
								Expect(objStat.Status.DeployedRelease.Name).To(Equal("test"))
								Expect(objStat.Status.DeployedRelease.Manifest).To(Equal("manifest: 1"))

								c := objStat.Status.Conditions.GetCondition(conditions.TypeIrreconcilable)
								Expect(c).NotTo(BeNil())
								Expect(c.Reason).To(Equal(conditions.ReasonReconcileError))
								Expect(c.Message).To(ContainSubstring("reconciliation failed: foobar"))
							})

							By("ensuring the uninstall finalizer is present on the CR", func() {
								Expect(controllerutil.ContainsFinalizer(obj, uninstallFinalizer)).To(BeTrue())
							})
						})
					})
					When("reconciliation succeeds", func() {
						It("reconciles the release", func() {
							var (
								rel *release.Release
								err error
							)
							By("changing the release resources", func() {
								for _, resource := range manifestToObjects(currentRelease.Manifest) {
									key := client.ObjectKeyFromObject(resource)

									u := &unstructured.Unstructured{}
									u.SetGroupVersionKind(resource.GetObjectKind().GroupVersionKind())
									err = mgr.GetAPIReader().Get(ctx, key, u)
									Expect(err).To(BeNil())

									labels := u.GetLabels()
									labels["app.kubernetes.io/managed-by"] = "Unmanaged"
									u.SetLabels(labels)

									err = mgr.GetClient().Update(ctx, u)
									Expect(err).To(BeNil())
								}
							})

							By("successfully reconciling a request", func() {
								res, err := r.Reconcile(ctx, req)
								Expect(res).To(Equal(reconcile.Result{}))
								Expect(err).To(BeNil())
							})

							By("getting the release and CR", func() {
								rel, err = ac.Get(obj.GetName())
								Expect(err).To(BeNil())
								Expect(rel).NotTo(BeNil())
								Expect(mgr.GetAPIReader().Get(ctx, objKey, obj)).To(Succeed())
							})

							By("verifying the release", func() {
								Expect(rel.Version).To(Equal(1))
								verifyRelease(ctx, mgr.GetAPIReader(), obj.GetNamespace(), rel)
							})

							By("verifying override event", func() {
								verifyEvent(ctx, mgr.GetAPIReader(), obj,
									"Warning",
									"ValueOverridden",
									`Chart value "image.repository" overridden to "custom-nginx" by operator`)
							})

							By("ensuring the uninstall finalizer is present", func() {
								Expect(obj.GetFinalizers()).To(ContainElement(uninstallFinalizer))
							})

							By("verifying the CR status", func() {
								objStat := &objStatus{}
								Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, objStat)).To(Succeed())
								Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeInitialized)).To(BeTrue())
								Expect(objStat.Status.Conditions.IsFalseFor(conditions.TypeIrreconcilable)).To(BeTrue())
								Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeDeployed)).To(BeTrue())
								Expect(objStat.Status.Conditions.IsFalseFor(conditions.TypeReleaseFailed)).To(BeTrue())
								Expect(objStat.Status.DeployedRelease.Name).To(Equal(rel.Name))
								Expect(objStat.Status.DeployedRelease.Manifest).To(Equal(rel.Manifest))
							})
						})
					})
					When("uninstall fails", func() {
						BeforeEach(func() {
							ac := helmfake.NewActionClient()
							ac.HandleGet = func() (*release.Release, error) {
								return &release.Release{Name: "test", Version: 1, Manifest: "manifest: 1"}, nil
							}
							ac.HandleUninstall = func() (*release.UninstallReleaseResponse, error) {
								return nil, errors.New("uninstall failed: foobar")
							}
							r.actionClientGetter = helmfake.NewActionClientGetter(&ac, nil)
						})
						It("handles the uninstall error", func() {
							By("deleting the CR", func() {
								Expect(mgr.GetClient().Delete(ctx, obj)).To(Succeed())
							})

							By("returning an error", func() {
								res, err := r.Reconcile(ctx, req)
								Expect(res).To(Equal(reconcile.Result{}))
								Expect(err).To(HaveOccurred())
							})

							By("getting the CR", func() {
								Expect(mgr.GetAPIReader().Get(ctx, objKey, obj)).To(Succeed())
							})

							By("ensuring the correct conditions are set on the CR", func() {
								objStat := &objStatus{}
								Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, objStat)).To(Succeed())
								Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeInitialized)).To(BeTrue())
								Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeIrreconcilable)).To(BeTrue())
								Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeDeployed)).To(BeTrue())
								Expect(objStat.Status.Conditions.IsTrueFor(conditions.TypeReleaseFailed)).To(BeTrue())
								Expect(objStat.Status.DeployedRelease.Name).To(Equal("test"))
								Expect(objStat.Status.DeployedRelease.Manifest).To(Equal("manifest: 1"))

								c := objStat.Status.Conditions.GetCondition(conditions.TypeReleaseFailed)
								Expect(c).NotTo(BeNil())
								Expect(c.Reason).To(Equal(conditions.ReasonUninstallError))
								Expect(c.Message).To(ContainSubstring("uninstall failed: foobar"))

								c = objStat.Status.Conditions.GetCondition(conditions.TypeIrreconcilable)
								Expect(c).NotTo(BeNil())
								Expect(c.Reason).To(Equal(conditions.ReasonReconcileError))
								Expect(c.Message).To(ContainSubstring("uninstall failed: foobar"))
							})

							By("ensuring the uninstall finalizer is present on the CR", func() {
								Expect(controllerutil.ContainsFinalizer(obj, uninstallFinalizer)).To(BeTrue())
							})
						})
					})
					When("uninstall succeeds", func() {
						It("uninstalls the release and removes the finalizer", func() {
							By("deleting the CR", func() {
								Expect(mgr.GetClient().Delete(ctx, obj)).To(Succeed())
							})

							By("successfully reconciling a request", func() {
								res, err := r.Reconcile(ctx, req)
								Expect(res).To(Equal(reconcile.Result{}))
								Expect(err).To(BeNil())
							})

							By("verifying the release is uninstalled", func() {
								verifyNoRelease(ctx, mgr.GetClient(), obj.GetNamespace(), obj.GetName(), currentRelease)
							})

							By("ensuring the finalizer is removed and the CR is deleted", func() {
								err := mgr.GetAPIReader().Get(ctx, objKey, obj)
								Expect(apierrors.IsNotFound(err)).To(BeTrue())
							})
						})
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
	Expect(err).To(BeNil())
	return mgr
}

type objStatus struct {
	Status struct {
		Conditions      status.Conditions `json:"conditions"`
		DeployedRelease *struct {
			Name     string `json:"name"`
			Manifest string `json:"manifest"`
		} `json:"deployedRelease"`
	} `json:"status"`
}

func manifestToObjects(manifest string) []client.Object {
	objs := []client.Object{}
	for _, m := range releaseutil.SplitManifests(manifest) {
		u := &unstructured.Unstructured{}
		err := yaml.Unmarshal([]byte(m), u)
		Expect(err).To(BeNil())
		objs = append(objs, u)
	}
	return objs
}

func verifyRelease(ctx context.Context, cl client.Reader, ns string, rel *release.Release) {
	By("verifying release secret exists at release version", func() {
		releaseSecrets := &v1.SecretList{}
		err := cl.List(ctx, releaseSecrets, client.InNamespace(ns), client.MatchingLabels{"owner": "helm", "name": rel.Name})
		Expect(err).To(BeNil())
		Expect(releaseSecrets.Items).To(HaveLen(rel.Version))
		Expect(releaseSecrets.Items[rel.Version-1].Type).To(Equal(v1.SecretType("helm.sh/release.v1")))
		Expect(releaseSecrets.Items[rel.Version-1].Labels["version"]).To(Equal(strconv.Itoa(rel.Version)))
		Expect(releaseSecrets.Items[rel.Version-1].Data["release"]).NotTo(BeNil())
	})

	By("verifying description annotation was honored", func() {
		if rel.Version == 1 {
			Expect(rel.Info.Description).To(Equal("test install description"))
		} else {
			Expect(rel.Info.Description).To(Equal("test upgrade description"))
		}
	})

	var objs []client.Object

	By("verifying the release resources exist", func() {
		objs = manifestToObjects(rel.Manifest)
		for _, obj := range objs {
			key := client.ObjectKeyFromObject(obj)
			err := cl.Get(ctx, key, obj)
			Expect(err).To(BeNil())
		}
	})

	By("verifying that deployment image was overridden", func() {
		for _, obj := range objs {
			if obj.GetName() == "test-test-chart" && obj.GetObjectKind().GroupVersionKind().Kind == "Deployment" {
				expectDeploymentImagePrefix(obj, "custom-nginx:")
				return
			}
		}
		Fail("expected deployment not found")
	})
}

func expectDeploymentImagePrefix(obj client.Object, prefix string) {
	u := obj.(*unstructured.Unstructured)
	containers, ok, err := unstructured.NestedSlice(u.Object, "spec", "template", "spec", "containers")
	Expect(ok).To(BeTrue())
	Expect(err).To(BeNil())
	container := containers[0].(map[string]interface{})
	val, ok, err := unstructured.NestedString(container, "image")
	Expect(ok).To(BeTrue())
	Expect(err).To(BeNil())
	Expect(val).To(HavePrefix(prefix))
}

func verifyNoRelease(ctx context.Context, cl client.Client, ns string, name string, rel *release.Release) {
	By("verifying all release secrets are removed", func() {
		releaseSecrets := &v1.SecretList{}
		err := cl.List(ctx, releaseSecrets, client.InNamespace(ns), client.MatchingLabels{"owner": "helm", "name": name})
		Expect(err).To(BeNil())
		Expect(releaseSecrets.Items).To(HaveLen(0))
	})
	By("verifying all release resources are removed", func() {
		if rel != nil {
			for _, r := range releaseutil.SplitManifests(rel.Manifest) {
				u := &unstructured.Unstructured{}
				err := yaml.Unmarshal([]byte(r), u)
				Expect(err).To(BeNil())

				key := client.ObjectKeyFromObject(u)
				err = cl.Get(ctx, key, u)
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}
		}
	})
}

func verifyHooksCalled(ctx context.Context, r *Reconciler, req reconcile.Request) {
	buf := &bytes.Buffer{}
	By("setting up a pre and post hook", func() {
		preHook := func(context.Context, *unstructured.Unstructured, logr.Logger) error {
			return errors.New("pre hook foobar")
		}
		postHook := func(context.Context, *unstructured.Unstructured, release.Release, chartutil.Values, logr.Logger) error {
			return errors.New("post hook foobar")
		}
		r.log = zap.New(zap.WriteTo(buf))
		r.extensions = append(r.extensions, hook.PreHookFunc(hook.WrapPreHookFunc(preHook)))
		r.extensions = append(r.extensions, hook.PostHookFunc(hook.WrapPostHookFunc(postHook)))
	})
	By("successfully reconciling a request", func() {
		res, err := r.Reconcile(ctx, req)
		Expect(err).To(BeNil())
		Expect(res).To(Equal(reconcile.Result{}))
	})
	By("verifying pre and post hooks were called and errors logged", func() {
		Expect(buf.String()).To(ContainSubstring("pre-release hook failed"))
		Expect(buf.String()).To(ContainSubstring("pre hook foobar"))
		Expect(buf.String()).To(ContainSubstring("post-release hook failed"))
		Expect(buf.String()).To(ContainSubstring("post hook foobar"))
	})
}

func verifyEvent(ctx context.Context, cl client.Reader, obj metav1.Object, eventType, reason, message string) {
	events := &v1.EventList{}
	Expect(cl.List(ctx, events, client.InNamespace(obj.GetNamespace()))).To(Succeed())
	for _, e := range events.Items {
		if e.Type == eventType && e.Reason == reason && e.Message == message {
			return
		}
	}
	Fail(fmt.Sprintf(`expected event with
	Type: %q
	Reason: %q
	Message: %q`, eventType, reason, message))
}

type testBeginReconcileExtension struct {
	f func() error
	extension.NoOpReconcilerExtension
}

func (e *testBeginReconcileExtension) Name() string {
	return "test-extension"
}

func (e *testBeginReconcileExtension) BeginReconcile(ctx context.Context, reconciliationContext *extension.Context, obj *unstructured.Unstructured) error {
	return e.f()
}

var _ extension.ReconcilerExtension = (*testBeginReconcileExtension)(nil)
