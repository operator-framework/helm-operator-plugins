// +k8s:deepcopy-gen=package
package updater

import (
	"context"

	"helm.sh/helm/v3/pkg/release"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/operator-framework/helm-operator/internal/controllerutil"
	"github.com/operator-framework/helm-operator/internal/status"
)

func New(client client.Client, obj *unstructured.Unstructured) *Updater {
	return &Updater{
		client: client,
	}
}

type Updater struct {
	client            client.Client
	updateFuncs       []UpdateFunc
	updateStatusFuncs []UpdateStatusFunc
}

type UpdateFunc func(*unstructured.Unstructured) bool
type UpdateStatusFunc func(*helmAppStatus) bool

func (u *Updater) Update(fs ...UpdateFunc) {
	u.updateFuncs = append(u.updateFuncs, fs...)
}

func (u *Updater) UpdateStatus(fs ...UpdateStatusFunc) {
	u.updateStatusFuncs = append(u.updateStatusFuncs, fs...)
}

func (u *Updater) Apply(ctx context.Context, obj *unstructured.Unstructured) error {
	backoff := retry.DefaultRetry

	// Always update the status first. During uninstall, if
	// we remove the finalizer, updating the status will fail
	// because the object and its status will be garbage-collected
	if err := retry.RetryOnConflict(backoff, func() error {
		status := statusFor(obj)
		needsStatusUpdate := false
		for _, f := range u.updateStatusFuncs {
			needsStatusUpdate = f(status) || needsStatusUpdate
		}
		if needsStatusUpdate {
			obj.Object["status"] = status
			return u.client.Status().Update(ctx, obj)
		}
		return nil
	}); err != nil {
		return err
	}

	if err := retry.RetryOnConflict(backoff, func() error {
		needsUpdate := false
		for _, f := range u.updateFuncs {
			needsUpdate = f(obj) || needsUpdate
		}
		if needsUpdate {
			return u.client.Update(ctx, obj)
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}

func EnsureFinalizer(finalizer string) UpdateFunc {
	return func(obj *unstructured.Unstructured) bool {
		if controllerutil.ContainsFinalizer(obj, finalizer) {
			return false
		}
		controllerutil.AddFinalizer(obj, finalizer)
		return true
	}
}

func RemoveFinalizer(finalizer string) UpdateFunc {
	return func(obj *unstructured.Unstructured) bool {
		if !controllerutil.ContainsFinalizer(obj, finalizer) {
			return false
		}
		controllerutil.RemoveFinalizer(obj, finalizer)
		return true
	}
}

func EnsureCondition(condition status.Condition) UpdateStatusFunc {
	return func(status *helmAppStatus) bool {
		return status.Conditions.SetCondition(condition)
	}
}

func RemoveCondition(conditionType status.ConditionType) UpdateStatusFunc {
	return func(status *helmAppStatus) bool {
		return status.Conditions.RemoveCondition(conditionType)
	}
}

func EnsureDeployedRelease(rel *release.Release) UpdateStatusFunc {
	return func(status *helmAppStatus) bool {
		newRel := helmAppReleaseFor(rel)
		if status.DeployedRelease == nil && newRel == nil {
			return false
		}
		if status.DeployedRelease != nil && newRel != nil &&
			*status.DeployedRelease == *newRel {
			return false
		}
		status.DeployedRelease = newRel
		return true
	}
}

func RemoveDeployedRelease() UpdateStatusFunc {
	return EnsureDeployedRelease(nil)
}

type helmAppStatus struct {
	Conditions      status.Conditions `json:"conditions"`
	DeployedRelease *helmAppRelease   `json:"deployedRelease,omitempty"`
}

type helmAppRelease struct {
	Name     string `json:"name,omitempty"`
	Manifest string `json:"manifest,omitempty"`
}

func statusFor(obj *unstructured.Unstructured) *helmAppStatus {
	if obj == nil || obj.Object == nil {
		return nil
	}
	status, ok := obj.Object["status"]
	if !ok {
		return &helmAppStatus{}
	}

	switch s := status.(type) {
	case *helmAppStatus:
		return s
	case helmAppStatus:
		return &s
	case map[string]interface{}:
		out := &helmAppStatus{}
		_ = runtime.DefaultUnstructuredConverter.FromUnstructured(s, out)
		return out
	default:
		return &helmAppStatus{}
	}
}

func helmAppReleaseFor(rel *release.Release) *helmAppRelease {
	if rel == nil {
		return nil
	}
	return &helmAppRelease{
		Name:     rel.Name,
		Manifest: rel.Manifest,
	}
}
