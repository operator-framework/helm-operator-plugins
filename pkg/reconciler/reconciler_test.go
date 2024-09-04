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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/releaseutil"
	"helm.sh/helm/v3/pkg/storage/driver"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sigs.k8s.io/yaml"

	sdkhandler "github.com/operator-framework/operator-lib/handler"

	"github.com/operator-framework/helm-operator-plugins/internal/sdk/controllerutil"
	"github.com/operator-framework/helm-operator-plugins/pkg/annotation"
	helmclient "github.com/operator-framework/helm-operator-plugins/pkg/client"
	"github.com/operator-framework/helm-operator-plugins/pkg/hook"
	"github.com/operator-framework/helm-operator-plugins/pkg/internal/status"
	"github.com/operator-framework/helm-operator-plugins/pkg/internal/testutil"
	"github.com/operator-framework/helm-operator-plugins/pkg/reconciler/internal/conditions"
	helmfake "github.com/operator-framework/helm-operator-plugins/pkg/reconciler/internal/fake"
	"github.com/operator-framework/helm-operator-plugins/pkg/values"
)

// custom is used within the reconciler test suite as underlying type for the GVK scheme.
type custom struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// [...]
}

// Usually auto-generated, required for making custom a type which can be registered within a GVK scheme.
func (in *custom) DeepCopyInto(out *custom) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
}

// Usually auto-generated, required for making custom a type which can be registered within a GVK scheme.
func (in *custom) DeepCopy() *custom {
	if in == nil {
		return nil
	}
	out := new(custom)
	in.DeepCopyInto(out)
	return out
}

// Usually auto-generated, required for making custom a type which can be registered within a GVK scheme.
func (in *custom) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// reconcilerTestSuiteOpts can be used for modifying for parameterizing the reconciler test suite.
type reconcilerTestSuiteOpts struct {
	customGVKSchemeSetup bool
}

var _ = Describe("Reconciler", func() {
	_ = Describe("New", func() {
		It("should fail without a GVK", func() {
			r, err := New(WithChart(chart.Chart{}))
			Expect(r).To(BeNil())
			Expect(err).To(HaveOccurred())
		})
		It("should fail without a chart", func() {
			r, err := New(WithGroupVersionKind(schema.GroupVersionKind{}))
			Expect(r).To(BeNil())
			Expect(err).To(HaveOccurred())
		})
		It("should succeed with just a GVK and chart", func() {
			r, err := New(WithChart(chart.Chart{}), WithGroupVersionKind(schema.GroupVersionKind{}))
			Expect(r).NotTo(BeNil())
			Expect(err).ToNot(HaveOccurred())
		})
		It("should return an error if an option func fails", func() {
			r, err := New(func(r *Reconciler) error { return errors.New("expect this error") })
			Expect(r).To(BeNil())
			Expect(err).To(MatchError("expect this error"))
		})
	})

	_ = Describe("Option", func() {
		var r *Reconciler
		BeforeEach(func() {
			r = &Reconciler{}
		})
		_ = Describe("WithClient", func() {
			It("should set the reconciler client", func() {
				client := fake.NewClientBuilder().Build()
				Expect(WithClient(client)(r)).To(Succeed())
				Expect(r.client).To(Equal(client))
			})
		})
		_ = Describe("WithActionClientGetter", func() {
			It("should set the reconciler action client getter", func() {
				fakeActionClientGetter := helmfake.NewActionClientGetter(nil, nil)
				Expect(WithActionClientGetter(fakeActionClientGetter)(r)).To(Succeed())
				Expect(r.actionClientGetter).To(Equal(fakeActionClientGetter))
			})
		})
		_ = Describe("WithEventRecorder", func() {
			It("should set the reconciler event recorder", func() {
				rec := record.NewFakeRecorder(0)
				Expect(WithEventRecorder(rec)(r)).To(Succeed())
				Expect(r.eventRecorder).To(Equal(rec))
			})
		})
		_ = Describe("WithLog", func() {
			It("should set the reconciler log", func() {
				log := logr.Discard()
				Expect(WithLog(log)(r)).To(Succeed())
				Expect(r.log).To(Equal(log))
			})
		})
		_ = Describe("WithGroupVersionKind", func() {
			It("should set the reconciler GVK", func() {
				gvk := schema.GroupVersionKind{Group: "mygroup", Version: "v1", Kind: "MyApp"}
				Expect(WithGroupVersionKind(gvk)(r)).To(Succeed())
				Expect(r.gvk).To(Equal(&gvk))
			})
		})
		_ = Describe("WithChart", func() {
			It("should set the reconciler chart", func() {
				chrt := chart.Chart{Metadata: &chart.Metadata{Name: "my-chart"}}
				Expect(WithChart(chrt)(r)).To(Succeed())
				Expect(r.chrt).To(Equal(&chrt))
			})
		})
		_ = Describe("WithOverrideValues", func() {
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
		_ = Describe("SkipDependentWatches", func() {
			It("should set to false", func() {
				Expect(SkipDependentWatches(false)(r)).To(Succeed())
				Expect(r.skipDependentWatches).To(BeFalse())
			})
			It("should set to true", func() {
				Expect(SkipDependentWatches(true)(r)).To(Succeed())
				Expect(r.skipDependentWatches).To(BeTrue())
			})
		})
		_ = Describe("WithMaxConcurrentReconciles", func() {
			It("should set the reconciler max concurrent reconciled", func() {
				Expect(WithMaxConcurrentReconciles(1)(r)).To(Succeed())
				Expect(r.maxConcurrentReconciles).To(Equal(1))
			})
			It("should fail if value is less than 1", func() {
				Expect(WithMaxConcurrentReconciles(0)(r)).NotTo(Succeed())
				Expect(WithMaxConcurrentReconciles(-1)(r)).NotTo(Succeed())
			})
		})
		_ = Describe("WithWaitForDeletionTimeout", func() {
			It("should set the reconciler wait for deletion timeout", func() {
				Expect(WithWaitForDeletionTimeout(time.Second)(r)).To(Succeed())
				Expect(r.waitForDeletionTimeout).To(Equal(time.Second))
			})
			It("should fail if value is zero", func() {
				Expect(WithWaitForDeletionTimeout(0)(r)).NotTo(Succeed())
			})
			It("should fail if value is negative", func() {
				Expect(WithWaitForDeletionTimeout(-time.Second)(r)).NotTo(Succeed())
			})
		})
		_ = Describe("WithReconcilePeriod", func() {
			It("should set the reconciler reconcile period", func() {
				Expect(WithReconcilePeriod(0)(r)).To(Succeed())
				Expect(r.reconcilePeriod).To(Equal(time.Duration(0)))
			})
			It("should fail if value is less than 0", func() {
				Expect(WithReconcilePeriod(-time.Nanosecond)(r)).NotTo(Succeed())
			})
		})
		_ = Describe("WithMaxReleaseHistory", func() {
			It("should set the max history size", func() {
				Expect(WithMaxReleaseHistory(10)(r)).To(Succeed())
				Expect(r.maxReleaseHistory).To(PointTo(Equal(10)))
			})
			It("should allow setting the history to unlimited", func() {
				Expect(WithMaxReleaseHistory(0)(r)).To(Succeed())
				Expect(r.maxReleaseHistory).To(PointTo(Equal(0)))
			})
			It("should fail if value is less than 0", func() {
				Expect(WithMaxReleaseHistory(-1)(r)).NotTo(Succeed())
			})
		})
		_ = Describe("WithInstallAnnotations", func() {
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
		_ = Describe("WithUpgradeAnnotations", func() {
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
		_ = Describe("WithUninstallAnnotations", func() {
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
		_ = Describe("WithPreHook", func() {
			It("should set a reconciler prehook", func() {
				called := false
				preHook := hook.PreHookFunc(func(*unstructured.Unstructured, chartutil.Values, logr.Logger) error {
					called = true
					return nil
				})
				Expect(WithPreHook(preHook)(r)).To(Succeed())
				Expect(r.preHooks).To(HaveLen(1))
				Expect(r.preHooks[0].Exec(nil, nil, logr.Discard())).To(Succeed())
				Expect(called).To(BeTrue())
			})
		})
		_ = Describe("WithPostHook", func() {
			It("should set a reconciler posthook", func() {
				called := false
				postHook := hook.PostHookFunc(func(*unstructured.Unstructured, release.Release, logr.Logger) error {
					called = true
					return nil
				})
				Expect(WithPostHook(postHook)(r)).To(Succeed())
				Expect(r.postHooks).To(HaveLen(1))
				Expect(r.postHooks[0].Exec(nil, release.Release{}, logr.Discard())).To(Succeed())
				Expect(called).To(BeTrue())
			})
		})
		_ = Describe("WithValueMapper", func() {
			It("should set the reconciler value mapper", func() {
				mapper := values.MapperFunc(func(chartutil.Values) chartutil.Values {
					return chartutil.Values{"mapped": true}
				})
				Expect(WithValueMapper(mapper)(r)).To(Succeed())
				Expect(r.valueMapper).NotTo(BeNil())
				Expect(r.valueMapper.Map(chartutil.Values{})).To(Equal(chartutil.Values{"mapped": true}))
			})
		})
		_ = Describe("WithValueTranslator", func() {
			It("should set the reconciler value translator", func() {
				translator := values.TranslatorFunc(func(ctx context.Context, u *unstructured.Unstructured) (chartutil.Values, error) {
					return chartutil.Values{"translated": true}, nil
				})
				Expect(WithValueTranslator(translator)(r)).To(Succeed())
				Expect(r.valueTranslator).NotTo(BeNil())
				Expect(r.valueTranslator.Translate(context.Background(), &unstructured.Unstructured{})).To(Equal(chartutil.Values{"translated": true}))
			})
		})
		_ = Describe("WithSelector", func() {
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

	_ = Describe("Reconcile", func() {
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
			go func() { Expect(mgr.GetCache().Start(ctx)).To(Succeed()) }()
			Expect(mgr.GetCache().WaitForCacheSync(ctx)).To(BeTrue())

			obj = testutil.BuildTestCR(gvk)
			objKey = types.NamespacedName{Namespace: obj.GetNamespace(), Name: obj.GetName()}
			req = reconcile.Request{NamespacedName: objKey}
		})

		AfterEach(func() {
			By("ensuring the release is uninstalled", func() {
				if _, err := ac.Get(obj.GetName()); errors.Is(err, driver.ErrReleaseNotFound) {
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
				Expect(err).ToNot(HaveOccurred())
				obj.SetFinalizers([]string{})
				Expect(mgr.GetClient().Update(ctx, obj)).To(Succeed())
				err = mgr.GetClient().Delete(ctx, obj)
				if apierrors.IsNotFound(err) {
					return
				}
				Expect(err).ToNot(HaveOccurred())
			})
			cancel()
		})

		// After migration to Ginkgo v2 this can be rewritten using e.g. DescribeTable.
		parameterizedReconcilerTests := func(opts reconcilerTestSuiteOpts) {
			BeforeEach(func() {
				var err error
				if opts.customGVKSchemeSetup {
					// Register custom type in manager.
					mgr.GetScheme().AddKnownTypes(gv, &custom{})
					metav1.AddToGroupVersion(mgr.GetScheme(), gv)

					// Create new reconciler, disabling the built-in registration of
					// generic unstructured.Unstructured as underlying type for the GVK scheme.
					r, err = New(
						WithGroupVersionKind(gvk),
						WithChart(chrt),
						WithInstallAnnotations(annotation.InstallDescription{}),
						WithUpgradeAnnotations(annotation.UpgradeDescription{}),
						WithUninstallAnnotations(annotation.UninstallDescription{}),
						WithOverrideValues(map[string]string{
							"image.repository": "custom-nginx",
						}),
						SkipPrimaryGVKSchemeRegistration(true),
					)
				} else {
					// Default behaviour using generic GVK scheme based on unstructured.Unstructured
					// for the reconciler.
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
				}
				Expect(err).ToNot(HaveOccurred())
				Expect(r.SetupWithManager(mgr)).To(Succeed())

				ac, err = r.actionClientGetter.ActionClientFor(ctx, obj)
				Expect(err).ToNot(HaveOccurred())
			})

			It("GVK type is registered in scheme", func() {
				customGvk := gvk
				if opts.customGVKSchemeSetup {
					customGvk.Kind = "custom"
				}
				Expect(mgr.GetScheme().Recognizes(customGvk)).To(BeTrue())
			})

			When("requested CR is not found", func() {
				It("returns successfully with no action", func() {
					res, err := r.Reconcile(ctx, req)
					Expect(res).To(Equal(reconcile.Result{}))
					Expect(err).ToNot(HaveOccurred())

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
								r.actionClientGetter = helmclient.ActionClientGetterFunc(func(context.Context, client.Object) (helmclient.ActionInterface, error) {
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
								r.actionClientGetter = helmclient.ActionClientGetterFunc(func(context.Context, client.Object) (helmclient.ActionInterface, error) {
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
								Expect(err).To(HaveOccurred())
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

							By("verifying the uninstall finalizer is present on the CR", func() {
								Expect(controllerutil.ContainsFinalizer(obj, uninstallFinalizer)).To(BeTrue())
							})
						})
					})
					When("cache contains stale CR that has actually been deleted", func() {
						// This test simulates what we expect to happen when we time out waiting for a CR that we
						// deleted to be removed from the cache.
						It("ignores not found errors and returns successfully", func() {
							By("deleting the CR and then setting a finalizer on the stale CR", func() {
								// We _actually_ remove the CR from the API server, but we'll make a fake client
								// that returns the stale CR.
								Expect(mgr.GetClient().Delete(ctx, obj)).To(Succeed())
								Eventually(func() error {
									return mgr.GetAPIReader().Get(ctx, objKey, obj)
								}).Should(WithTransform(apierrors.IsNotFound, BeTrue()))

								// We set the finalizer on the stale CR to simulate the typical state of the CR from a
								// prior reconcile run that timed out waiting for the CR to be removed from the cache.
								obj.SetFinalizers([]string{uninstallFinalizer})
							})

							By("configuring a client that returns the stale CR", func() {
								// Make a client that returns the stale CR, but sends writes to the real client.
								cl := fake.NewClientBuilder().WithObjects(obj).WithInterceptorFuncs(interceptor.Funcs{
									Create: func(ctx context.Context, _ client.WithWatch, fakeObj client.Object, opts ...client.CreateOption) error {
										return mgr.GetClient().Create(ctx, fakeObj, opts...)
									},
									Delete: func(ctx context.Context, _ client.WithWatch, fakeObj client.Object, opts ...client.DeleteOption) error {
										return mgr.GetClient().Delete(ctx, fakeObj, opts...)
									},
									DeleteAllOf: func(ctx context.Context, _ client.WithWatch, fakeObj client.Object, opts ...client.DeleteAllOfOption) error {
										return mgr.GetClient().DeleteAllOf(ctx, fakeObj, opts...)
									},
									Update: func(ctx context.Context, _ client.WithWatch, fakeObj client.Object, opts ...client.UpdateOption) error {
										return mgr.GetClient().Update(ctx, fakeObj, opts...)
									},
									Patch: func(ctx context.Context, _ client.WithWatch, fakeObj client.Object, patch client.Patch, opts ...client.PatchOption) error {
										return mgr.GetClient().Patch(ctx, fakeObj, patch, opts...)
									},
									SubResource: func(_ client.WithWatch, subresource string) client.SubResourceClient {
										return mgr.GetClient().SubResource(subresource)
									},
								}).WithStatusSubresource(obj).Build()
								r.client = cl
							})

							By("successfully ignoring not found errors and returning a nil error", func() {
								res, err := r.Reconcile(ctx, req)
								Expect(res).To(Equal(reconcile.Result{}))
								Expect(err).ToNot(HaveOccurred())
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
								Expect(err).ToNot(HaveOccurred())
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

								By("verifying the uninstall finalizer is present on the CR", func() {
									Expect(controllerutil.ContainsFinalizer(obj, uninstallFinalizer)).To(BeTrue())
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
									Expect(err).ToNot(HaveOccurred())
									Expect(res).To(Equal(reconcile.Result{}))
								})

								By("getting the release and CR", func() {
									rel, err = ac.Get(obj.GetName())
									Expect(err).ToNot(HaveOccurred())
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
					var currentRelease *release.Release
					BeforeEach(func() {
						// Reconcile once to get the release installed and finalizers added
						var err error
						res, err := r.Reconcile(ctx, req)
						Expect(res).To(Equal(reconcile.Result{}))
						Expect(err).ToNot(HaveOccurred())

						currentRelease, err = ac.Get(obj.GetName())
						Expect(err).ToNot(HaveOccurred())
					})
					When("action client getter is not working", func() {
						It("returns an error getting the action client", func() {
							acgErr := errors.New("broken action client getter: error getting action client")

							By("creating a reconciler with a broken action client getter", func() {
								r.actionClientGetter = helmclient.ActionClientGetterFunc(func(context.Context, client.Object) (helmclient.ActionInterface, error) {
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
								r.actionClientGetter = helmclient.ActionClientGetterFunc(func(context.Context, client.Object) (helmclient.ActionInterface, error) {
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
								Expect(err).To(HaveOccurred())
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

							By("verifying the uninstall finalizer is present on the CR", func() {
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

							By("verifying the uninstall finalizer is present on the CR", func() {
								Expect(controllerutil.ContainsFinalizer(obj, uninstallFinalizer)).To(BeTrue())
							})
						})
					})
					When("requested CR release is not deployed", func() {
						var actionConf *action.Configuration
						BeforeEach(func() {
							By("getting the current release and config", func() {
								acg, err := helmclient.NewActionConfigGetter(mgr.GetConfig(), mgr.GetRESTMapper())
								Expect(err).ShouldNot(HaveOccurred())
								actionConf, err = acg.ActionConfigFor(ctx, obj)
								Expect(err).ToNot(HaveOccurred())
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
									Expect(err).ToNot(HaveOccurred())
								})
								By("verifying the release", func() {
									rel, err := ac.Get(obj.GetName())
									Expect(err).ToNot(HaveOccurred())
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
									Expect(err).ToNot(HaveOccurred())
								})
								By("verifying the release", func() {
									rel, err := ac.Get(obj.GetName())
									Expect(err).ToNot(HaveOccurred())
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
									Expect(err).ToNot(HaveOccurred())
								})

								By("getting the release and CR", func() {
									rel, err = ac.Get(obj.GetName())
									Expect(err).ToNot(HaveOccurred())
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
										Expect(err).ToNot(HaveOccurred())

										labels := u.GetLabels()
										labels["app.kubernetes.io/managed-by"] = "Unmanaged"
										u.SetLabels(labels)

										err = mgr.GetClient().Update(ctx, u)
										Expect(err).ToNot(HaveOccurred())
									}
								})

								By("successfully reconciling a request", func() {
									res, err := r.Reconcile(ctx, req)
									Expect(res).To(Equal(reconcile.Result{}))
									Expect(err).ToNot(HaveOccurred())
								})

								By("getting the release and CR", func() {
									rel, err = ac.Get(obj.GetName())
									Expect(err).ToNot(HaveOccurred())
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
									Expect(err).ToNot(HaveOccurred())
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
		}

		When("generic GVK scheme setup", func() {
			parameterizedReconcilerTests(reconcilerTestSuiteOpts{})
		})

		When("custom type GVK scheme setup ", func() {
			parameterizedReconcilerTests(reconcilerTestSuiteOpts{customGVKSchemeSetup: true})
		})
	})

	_ = Describe("Test custom controller setup", func() {
		var (
			mgr                   manager.Manager
			r                     *Reconciler
			err                   error
			controllerSetupCalled bool
		)
		additionalGVK := schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "SomeOtherKind"}
		setupController := func(c ControllerSetup) error {
			controllerSetupCalled = true
			u := &unstructured.Unstructured{}
			u.SetGroupVersionKind(additionalGVK)
			return c.Watch(
				source.Kind(
					mgr.GetCache(),
					u,
					&sdkhandler.InstrumentedEnqueueRequestForObject[*unstructured.Unstructured]{},
				),
			)
		}

		It("Registering builder setup function for reconciler works", func() {
			mgr = getManagerOrFail()
			r, err = New(
				WithGroupVersionKind(gvk),
				WithChart(chrt),
				WithInstallAnnotations(annotation.InstallDescription{}),
				WithUpgradeAnnotations(annotation.UpgradeDescription{}),
				WithUninstallAnnotations(annotation.UninstallDescription{}),
				WithOverrideValues(map[string]string{
					"image.repository": "custom-nginx",
				}),
				WithControllerSetupFunc(setupController),
			)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Setting up reconciler with manager causes custom builder setup to be executed", func() {
			Expect(r.SetupWithManager(mgr)).To(Succeed())
			Expect(controllerSetupCalled).To(BeTrue())
		})
	})
})

func getManagerOrFail() manager.Manager {
	// Since the dependent resource watcher accepts a scheme, everytime
	// a new manager is created it needs to have a new scheme in tests
	// to avoid race conditions.
	sch := runtime.NewScheme()
	Expect(clientgoscheme.AddToScheme(sch)).NotTo(HaveOccurred())
	mgr, err := manager.New(cfg, manager.Options{
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
		Scheme: sch,
	})
	Expect(err).ToNot(HaveOccurred())
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
		Expect(err).ToNot(HaveOccurred())
		objs = append(objs, u)
	}
	return objs
}

func verifyRelease(ctx context.Context, cl client.Reader, ns string, rel *release.Release) {
	By("verifying release secret exists at release version", func() {
		releaseSecrets := &corev1.SecretList{}
		err := cl.List(ctx, releaseSecrets, client.InNamespace(ns), client.MatchingLabels{"owner": "helm", "name": rel.Name})
		Expect(err).ToNot(HaveOccurred())
		Expect(releaseSecrets.Items).To(HaveLen(rel.Version))
		Expect(releaseSecrets.Items[rel.Version-1].Type).To(Equal(corev1.SecretType("helm.sh/release.v1")))
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
			Expect(err).ToNot(HaveOccurred())
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
	Expect(err).ToNot(HaveOccurred())
	container := containers[0].(map[string]interface{})
	val, ok, err := unstructured.NestedString(container, "image")
	Expect(ok).To(BeTrue())
	Expect(err).ToNot(HaveOccurred())
	Expect(val).To(HavePrefix(prefix))
}

func verifyNoRelease(ctx context.Context, cl client.Client, ns string, name string, rel *release.Release) {
	By("verifying all release secrets are removed", func() {
		releaseSecrets := &corev1.SecretList{}
		err := cl.List(ctx, releaseSecrets, client.InNamespace(ns), client.MatchingLabels{"owner": "helm", "name": name})
		Expect(err).ToNot(HaveOccurred())
		Expect(releaseSecrets.Items).To(BeEmpty())
	})
	By("verifying all release resources are removed", func() {
		if rel != nil {
			for _, r := range releaseutil.SplitManifests(rel.Manifest) {
				u := &unstructured.Unstructured{}
				err := yaml.Unmarshal([]byte(r), u)
				Expect(err).ToNot(HaveOccurred())

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
		preHook := hook.PreHookFunc(func(*unstructured.Unstructured, chartutil.Values, logr.Logger) error {
			return errors.New("pre hook foobar")
		})
		postHook := hook.PostHookFunc(func(*unstructured.Unstructured, release.Release, logr.Logger) error {
			return errors.New("post hook foobar")
		})
		r.log = zap.New(zap.WriteTo(buf))
		r.preHooks = append(r.preHooks, preHook)
		r.postHooks = append(r.postHooks, postHook)
	})
	By("successfully reconciling a request", func() {
		res, err := r.Reconcile(ctx, req)
		Expect(err).ToNot(HaveOccurred())
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
	events := &corev1.EventList{}
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
