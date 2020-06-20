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
	// TypeInitialized define the type of the Status Conditional when the controller is successfully initialized
	// Use to create conditional status to describe that this step was successfully performed when an error be faced
	// in the next steps for example to Deploy or to Reconcile an CR.
	TypeInitialized    = "Initialized"
	// TypeDeployed define the type of the Status Conditional when the controller is successfully deployed.
	// Use to create conditional status to describe that this step was successfully performed when an error be faced
	// in the next steps for example to Reconcile.
	TypeDeployed       = "Deployed"
	// TypeReleaseFailed define the type of the Status Conditional to failures related to release (CR) such as
	// when is not possible to ensure the correct conditions are set on the CR, to verify the CR status.
	TypeReleaseFailed  = "ReleaseFailed"
	// TypeIrreconcilable define the type of the Status Conditional when the controller reconcile failed in a conditional
	// that has no reason to still trying perform the reconciliation, as for example, when the CR/Release is not
	// found in the cluster.
	TypeIrreconcilable = "Irreconcilable"

	// ReasonInstallSuccessful used to create the status when the release was successfully uninstalled. When a released
	// is deployed and its version < 1 than it means that it was successfully installed.
	ReasonInstallSuccessful   = status.ConditionReason("InstallSuccessful")
	// ReasonUpgradeSuccessful used to create the status when the release was successfully upgraded. When a released
	// is deployed if its version is > 1 that it means that it was an upgrade and not an installation
	ReasonUpgradeSuccessful   = status.ConditionReason("UpgradeSuccessful")
	// ReasonUninstallSuccessful used to create the status when the deployed fails because the release is uninstalled
	ReasonUninstallSuccessful = status.ConditionReason("UninstallSuccessful")

	// ReasonErrorGettingClient is used when is not possible reconcile because the release/CR was not found
	ReasonErrorGettingClient       = status.ConditionReason("ErrorGettingClient")
	// ReasonErrorGettingValues is used when is not possible reconcile because was not possible parse the chart values
	// E.g example/helm-charts/nginx/values.yaml
	ReasonErrorGettingValues       = status.ConditionReason("ErrorGettingValues")
	// ReasonErrorGettingReleaseState is used when is not possible reconcile because was not possible obtain the
	// release/CR status
	ReasonErrorGettingReleaseState = status.ConditionReason("ErrorGettingReleaseState")
	// ReasonInstallError is used when an error is faced to install the release as for example when is not possible to
	// ensure the correct conditions are set on the CR
	ReasonInstallError             = status.ConditionReason("InstallError")
	// ReasonUpgradeError is used when an error is faced to upgrade the release as for example when is not possible to
	// ensure the correct conditions are set on the CR
	ReasonUpgradeError             = status.ConditionReason("UpgradeError")
	// ReasonReconcileError is used when an error is faced to reconcile the release/CR
	ReasonReconcileError           = status.ConditionReason("ReconcileError")
	// ReasonUninstallError is used when an error is faced to uninstall the release/CR
	ReasonUninstallError           = status.ConditionReason("UninstallError")
)

// Initialized returns a status.Condition of the TypeInitialized with the reason and message informed
func Initialized(stat corev1.ConditionStatus, reason status.ConditionReason, message interface{}) status.Condition {
	return newCondition(TypeInitialized, stat, reason, message)
}

// Deployed returns a status.Condition of the TypeDeployed with the reason and message informed
func Deployed(stat corev1.ConditionStatus, reason status.ConditionReason, message interface{}) status.Condition {
	return newCondition(TypeDeployed, stat, reason, message)
}

// ReleaseFailed returns a status.Condition of the TypeReleaseFailed with the reason and message informed
func ReleaseFailed(stat corev1.ConditionStatus, reason status.ConditionReason, message interface{}) status.Condition {
	return newCondition(TypeReleaseFailed, stat, reason, message)
}

// Irreconcilable returns a status.Condition of the TypeIrreconcilable with the reason and message informed
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
