package reconciler_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/chart"
	"k8s.io/apimachinery/pkg/runtime/schema"

	. "github.com/joelanford/helm-operator/pkg/reconciler"
)

var _ = Describe("Reconciler", func() {
	var _ = Describe("New", func() {
		It("should fail with no options", func() {
			r, err := New()
			Expect(r).To(BeNil())
			Expect(err).NotTo(BeNil())
		})
		It("should fail without a GVK", func() {
			r, err := New(WithChart(&chart.Chart{}))
			Expect(r).To(BeNil())
			Expect(err).NotTo(BeNil())
		})
		It("should fail without a chart", func() {
			r, err := New(WithGroupVersionKind(schema.GroupVersionKind{}))
			Expect(r).To(BeNil())
			Expect(err).NotTo(BeNil())
		})
		It("should succeed with just a GVK and chart", func() {
			r, err := New(WithChart(&chart.Chart{}), WithGroupVersionKind(schema.GroupVersionKind{}))
			Expect(r).NotTo(BeNil())
			Expect(err).To(BeNil())
		})
	})
})
