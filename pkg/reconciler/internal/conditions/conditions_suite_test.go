package conditions_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestConditions(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Conditions Suite")
}
