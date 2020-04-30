package hook_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestHook(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Hook Suite")
}
