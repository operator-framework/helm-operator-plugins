package hook_test

import (
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	. "github.com/joelanford/helm-operator/pkg/hook"
)

var _ = Describe("Hook", func() {
	var _ = Describe("PreHookFunc", func() {
		It("should implement the PreHook interface", func() {
			called := false
			var h PreHook = PreHookFunc(func(*unstructured.Unstructured, *chartutil.Values, logr.Logger) error {
				called = true
				return nil
			})
			Expect(h.Exec(nil, nil, nil)).To(Succeed())
			Expect(called).To(BeTrue())
		})
	})
	var _ = Describe("PostHookFunc", func() {
		It("should implement the PostHook interface", func() {
			called := false
			var h PostHook = PostHookFunc(func(*unstructured.Unstructured, *release.Release, logr.Logger) error {
				called = true
				return nil
			})
			Expect(h.Exec(nil, nil, nil)).To(Succeed())
			Expect(called).To(BeTrue())
		})
	})
})
