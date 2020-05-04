package reconciler

import (
	"time"

	"github.com/go-logr/logr/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/chart"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/joelanford/helm-operator/pkg/client"
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
				cfgGetter, err := client.NewActionConfigGetter(nil, nil, nil)
				Expect(err).To(BeNil())

				acg := client.NewActionClientGetter(cfgGetter)
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
				Expect(WithMaxConcurrentReconciles(5)(r)).To(Succeed())
				Expect(r.maxConcurrentReconciles).To(Equal(5))
			})
		})
		var _ = Describe("WithReconcilePeriod", func() {
			It("should set the reconciler reconcile period", func() {
				Expect(WithReconcilePeriod(time.Nanosecond)(r)).To(Succeed())
				Expect(r.reconcilePeriod).To(Equal(time.Nanosecond))
			})
		})
		var _ = Describe("WithInstallAnnotation", func() {
			PIt("should set the reconciler install annotation", func() {

			})
		})
		var _ = Describe("WithUpgradeAnnotation", func() {
			PIt("should set the reconciler upgrade annotation", func() {

			})
		})
		var _ = Describe("WithUninstallAnnotation", func() {
			PIt("should set the reconciler uninstall annotation", func() {

			})
		})
		var _ = Describe("WithPreHook", func() {
			PIt("should set a reconciler prehook", func() {

			})
		})
		var _ = Describe("WithPostHook", func() {
			PIt("should set a reconciler posthook", func() {

			})
		})
		var _ = Describe("WithValueMapper", func() {
			PIt("should set the reconciler value mapper", func() {

			})
		})
	})
})
