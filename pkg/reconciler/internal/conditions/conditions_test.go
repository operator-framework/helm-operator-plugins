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

package conditions_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	"github.com/operator-framework/helm-operator-plugins/pkg/internal/status"
	. "github.com/operator-framework/helm-operator-plugins/pkg/reconciler/internal/conditions"
)

var _ = Describe("Conditions", func() {
	var _ = Describe("Initialized", func() {
		It("should return an Initialized condition with the correct status, reason, and message", func() {
			e := status.Condition{
				Type:    TypeInitialized,
				Status:  corev1.ConditionTrue,
				Reason:  "reason",
				Message: "message",
			}
			Expect(Initialized(e.Status, e.Reason, e.Message)).To(Equal(e))
		})
	})

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
