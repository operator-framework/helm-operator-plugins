// Copyright 2020 The Operator-SDK Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package conditions

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/operator-framework/helm-operator/internal/status"
)

const (
	TypeInitialized    = "Initialized"
	TypeDeployed       = "Deployed"
	TypeReleaseFailed  = "ReleaseFailed"
	TypeIrreconcilable = "Irreconcilable"

	ReasonInstallSuccessful   = status.ConditionReason("InstallSuccessful")
	ReasonUpgradeSuccessful   = status.ConditionReason("UpgradeSuccessful")
	ReasonUninstallSuccessful = status.ConditionReason("UninstallSuccessful")

	ReasonInstallError   = status.ConditionReason("InstallError")
	ReasonUpgradeError   = status.ConditionReason("UpgradeError")
	ReasonReconcileError = status.ConditionReason("ReconcileError")
	ReasonUninstallError = status.ConditionReason("UninstallError")
)

func Initialized() status.Condition {
	return status.Condition{
		Type:   TypeInitialized,
		Status: corev1.ConditionTrue,
	}
}

func Deployed(stat corev1.ConditionStatus, reason status.ConditionReason, message string) status.Condition {
	return status.Condition{
		Type:    TypeDeployed,
		Status:  stat,
		Reason:  reason,
		Message: message,
	}
}

func ReleaseFailed(reason status.ConditionReason, err error) status.Condition {
	return status.Condition{
		Type:    TypeReleaseFailed,
		Status:  corev1.ConditionTrue,
		Reason:  reason,
		Message: err.Error(),
	}
}

func Irreconcilable(err error) status.Condition {
	return status.Condition{
		Type:    TypeIrreconcilable,
		Status:  corev1.ConditionTrue,
		Reason:  ReasonReconcileError,
		Message: err.Error(),
	}
}
