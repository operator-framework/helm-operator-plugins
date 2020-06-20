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

package conditions

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/joelanford/helm-operator/pkg/internal/sdk/status"
)

const (
	// TypeInitialized //TODO
	TypeInitialized = "Initialized"
	// TypeDeployed //TODO
	TypeDeployed = "Deployed"
	// TypeReleaseFailed //TODO
	TypeReleaseFailed = "ReleaseFailed"
	// TypeIrreconcilable //TODO
	TypeIrreconcilable = "Irreconcilable"

	// ReasonInstallSuccessful //TODO
	ReasonInstallSuccessful = status.ConditionReason("InstallSuccessful")
	// ReasonUpgradeSuccessful //TODO
	ReasonUpgradeSuccessful = status.ConditionReason("UpgradeSuccessful")
	// ReasonUninstallSuccessful //TODO
	ReasonUninstallSuccessful = status.ConditionReason("UninstallSuccessful")

	// ReasonErrorGettingClient //TODO
	ReasonErrorGettingClient = status.ConditionReason("ErrorGettingClient")
	// ReasonErrorGettingValues //TODO
	ReasonErrorGettingValues = status.ConditionReason("ErrorGettingValues")
	// ReasonErrorGettingReleaseState //TODO
	ReasonErrorGettingReleaseState = status.ConditionReason("ErrorGettingReleaseState")
	// ReasonInstallError //TODO
	ReasonInstallError = status.ConditionReason("InstallError")
	// ReasonUpgradeError //TODO
	ReasonUpgradeError = status.ConditionReason("UpgradeError")
	// ReasonReconcileError //TODO
	ReasonReconcileError = status.ConditionReason("ReconcileError")
	// ReasonUninstallError //TODO
	ReasonUninstallError = status.ConditionReason("UninstallError")
)

// Initialized //TODO
func Initialized(stat corev1.ConditionStatus, reason status.ConditionReason, message interface{}) status.Condition {
	return newCondition(TypeInitialized, stat, reason, message)
}

// Deployed //TODO
func Deployed(stat corev1.ConditionStatus, reason status.ConditionReason, message interface{}) status.Condition {
	return newCondition(TypeDeployed, stat, reason, message)
}

// ReleaseFailed //TODO
func ReleaseFailed(stat corev1.ConditionStatus, reason status.ConditionReason, message interface{}) status.Condition {
	return newCondition(TypeReleaseFailed, stat, reason, message)
}

//Irreconcilable //TODO
func Irreconcilable(stat corev1.ConditionStatus, reason status.ConditionReason, message interface{}) status.Condition {
	return newCondition(TypeIrreconcilable, stat, reason, message)
}

func newCondition(t status.ConditionType, s corev1.ConditionStatus, r status.ConditionReason, m interface{}) status.Condition {
	message := fmt.Sprintf("%s", m)
	return status.Condition{
		Type:    t,
		Status:  s,
		Reason:  r,
		Message: message,
	}
}
