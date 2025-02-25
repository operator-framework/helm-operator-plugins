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

package updater

import (
	"context"

	"helm.sh/helm/v3/pkg/release"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/operator-framework/helm-operator-plugins/internal/sdk/controllerutil"
	"github.com/operator-framework/helm-operator-plugins/pkg/internal/status"
)

func New(client client.Client) Updater {
	return Updater{
		client: client,
	}
}

type Updater struct {
	isCanceled        bool
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

func (u *Updater) CancelUpdates() {
	u.isCanceled = true
}

func isRetryableUpdateError(err error) bool {
	return !errors.IsConflict(err) && !errors.IsNotFound(err)
}

// retryOnRetryableUpdateError retries the given function until it succeeds,
// until the given backoff is exhausted, or until the error is not retryable.
//
// In case of a Conflict error, the update cannot be retried because the underlying
// resource has been modified in the meantime, and the reconciliation loop needs
// to be restarted anew.
//
// A NotFound error means that the object has been deleted, and the reconciliation loop
// needs to be restarted anew as well.
func retryOnRetryableUpdateError(backoff wait.Backoff, f func() error) error {
	return retry.OnError(backoff, isRetryableUpdateError, f)
}

func (u *Updater) Apply(ctx context.Context, obj *unstructured.Unstructured) error {
	if u.isCanceled {
		return nil
	}

	backoff := retry.DefaultRetry

	st := statusFor(obj)
	needsStatusUpdate := false
	for _, f := range u.updateStatusFuncs {
		needsStatusUpdate = f(st) || needsStatusUpdate
	}

	// Always update the status first. During uninstall, if
	// we remove the finalizer, updating the status will fail
	// because the object and its status will be garbage-collected.
	if needsStatusUpdate {
		uSt, err := runtime.DefaultUnstructuredConverter.ToUnstructured(st)
		if err != nil {
			return err
		}
		obj.Object["status"] = uSt

		if err := retryOnRetryableUpdateError(backoff, func() error {
			return u.client.Status().Update(ctx, obj)
		}); err != nil {
			return err
		}
	}

	needsUpdate := false
	for _, f := range u.updateFuncs {
		needsUpdate = f(obj) || needsUpdate
	}
	if needsUpdate {
		if err := retryOnRetryableUpdateError(backoff, func() error {
			return u.client.Update(ctx, obj)
		}); err != nil {
			return err
		}
	}
	return nil
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

func EnsureConditionUnknown(t status.ConditionType) UpdateStatusFunc {
	return func(s *helmAppStatus) bool {
		return s.Conditions.SetCondition(status.Condition{
			Type:   t,
			Status: corev1.ConditionUnknown,
		})
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
