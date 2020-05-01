package manager_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/joelanford/helm-operator/pkg/manager"
)

var _ = Describe("NewDelegatingClientFunc", func() {
	It("should return a function that returns a delegating client", func() {
		// TODO(joelanford): Update this to use testenv.
		//   There isn't an easy way to test that the function returned
		//   by NewDelegatingClientFunc returns a delegating client
		//   without using testenv.
		Expect(NewDelegatingClientFunc()).NotTo(BeNil())
	})
})
