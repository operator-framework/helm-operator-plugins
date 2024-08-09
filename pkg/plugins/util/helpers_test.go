package util

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPlugin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Plugins Util Suite")
}

var _ = Describe("hasDifferentAPIVersion", func() {
	DescribeTable("should return false",
		func(versions []string) { Expect(hasDifferentAPIVersion(versions, "v1")).To(BeFalse()) },
		Entry("for an empty list of versions", []string{}),
		Entry("for a list of only that version", []string{"v1"}),
	)

	DescribeTable("should return true",
		func(versions []string) { Expect(hasDifferentAPIVersion(versions, "v1")).To(BeTrue()) },
		Entry("for a list of only a different version", []string{"v2"}),
		Entry("for a list of several different versions", []string{"v2", "v3"}),
		Entry("for a list of several versions containing that version", []string{"v1", "v2"}),
	)
})
