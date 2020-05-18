package conditions_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	"github.com/joelanford/helm-operator/pkg/internal/sdk/status"
	. "github.com/joelanford/helm-operator/pkg/reconciler/internal/conditions"
)

var _ = Describe("Conditions", func() {
	var _ = Describe("Deployed", func() {
		It("should return a Deployed condition with the correct status, reason, and message", func() {
			e := status.Condition{
				Type:    TypeDeployed,
				Status:  corev1.ConditionTrue,
				Reason:  "reason",
				Message: "message",
			}
			Expect(Deployed(e.Status, e.Reason, e.Message)).To(Equal(e))
		})
	})

	var _ = Describe("ReleaseFailed", func() {
		It("should return a ReleaseFailed condition with the correct reason and message", func() {
			err := errors.New("error message")
			e := status.Condition{
				Type:    TypeReleaseFailed,
				Status:  corev1.ConditionTrue,
				Reason:  "reason",
				Message: err.Error(),
			}
			Expect(ReleaseFailed(e.Status, e.Reason, err)).To(Equal(e))
		})
	})

	var _ = Describe("Irreconcilable", func() {
		It("should return an Irreconcilable condition with the correct message", func() {
			err := errors.New("error message")
			e := status.Condition{
				Type:    TypeIrreconcilable,
				Status:  corev1.ConditionTrue,
				Reason:  ReasonReconcileError,
				Message: err.Error(),
			}
			Expect(Irreconcilable(e.Status, e.Reason, err)).To(Equal(e))
		})
	})
})
