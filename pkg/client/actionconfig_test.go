package client

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/joelanford/helm-operator/pkg/internal/testutil"
)

var _ = Describe("ActionConfig", func() {
	var _ = Describe("NewActionConfigGetter", func() {
		It("should return a valid ActionConfigGetter", func() {
			Expect(NewActionConfigGetter(nil, nil, nil)).NotTo(BeNil())
		})
	})

	var _ = Describe("GetActionConfig", func() {
		var obj Object
		BeforeEach(func() {
			obj = testutil.BuildTestCR(gvk)
		})
		It("should return a valid action.Configuration", func() {
			rm, err := apiutil.NewDiscoveryRESTMapper(cfg)
			Expect(err).To(BeNil())

			acg := NewActionConfigGetter(cfg, rm, nil)
			ac, err := acg.ActionConfigFor(obj)
			Expect(err).To(BeNil())
			Expect(ac).NotTo(BeNil())
		})
	})
})
