package conditions

import (
	"github.com/operator-framework/helm-operator/internal/status"
	corev1 "k8s.io/api/core/v1"
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
