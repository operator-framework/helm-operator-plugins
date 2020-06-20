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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/joelanford/helm-operator/pkg/internal/sdk/controllerutil"
	"github.com/joelanford/helm-operator/pkg/internal/sdk/status"
)

// New returns new Updater for the client
func New(client client.Client) Updater {
	return Updater{
		client: client,
	}
}

// Updater represents the update release needs
type Updater struct {
	client            client.Client
	updateFuncs       []UpdateFunc
	updateStatusFuncs []UpdateStatusFunc
}

// UpdateFunc implements the UpdateFunc for an release/CR
type UpdateFunc func(*unstructured.Unstructured) bool
// UpdateStatusFunc implements the UpdateStatusFunc for an helmAppStatus
type UpdateStatusFunc func(*helmAppStatus) bool

// Update add UpdateFuncs to Updater
func (u *Updater) Update(fs ...UpdateFunc) {
	u.updateFuncs = append(u.updateFuncs, fs...)
}

// UpdateStatus add updateStatusFuncs to UpdateStatusFunc
func (u *Updater) UpdateStatus(fs ...UpdateStatusFunc) {
	u.updateStatusFuncs = append(u.updateStatusFuncs, fs...)
}

// Apply update functions to the release/CR
func (u *Updater) Apply(ctx context.Context, obj *unstructured.Unstructured) error {
	backoff := retry.DefaultRetry

	// Always update the status first. During uninstall, if
	// we remove the finalizer, updating the status will fail
	// because the object and its status will be garbage-collected
	if err := retry.RetryOnConflict(backoff, func() error {
		st := statusFor(obj)
		needsStatusUpdate := false
		for _, f := range u.updateStatusFuncs {
			needsStatusUpdate = f(st) || needsStatusUpdate
		}
		if needsStatusUpdate {
			uSt, err := runtime.DefaultUnstructuredConverter.ToUnstructured(st)
			if err != nil {
				return err
			}
			obj.Object["status"] = uSt
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

// EnsureFinalizer ensure that the release/CR will have the finalizer
func EnsureFinalizer(finalizer string) UpdateFunc {
	return func(obj *unstructured.Unstructured) bool {
		if controllerutil.ContainsFinalizer(obj, finalizer) {
			return false
		}
		controllerutil.AddFinalizer(obj, finalizer)
		return true
	}
}

// RemoveFinalizer removes the finalizer informed from the release/CR
func RemoveFinalizer(finalizer string) UpdateFunc {
	return func(obj *unstructured.Unstructured) bool {
		if !controllerutil.ContainsFinalizer(obj, finalizer) {
			return false
		}
		controllerutil.RemoveFinalizer(obj, finalizer)
		return true
	}
}

// EnsureCondition ensure status conditions updates for the release/CR
func EnsureCondition(condition status.Condition) UpdateStatusFunc {
	return func(status *helmAppStatus) bool {
		return status.Conditions.SetCondition(condition)
	}
}

// EnsureConditionUnknown ensure that Unknown status will be set in the release/CR
func EnsureConditionUnknown(t status.ConditionType) UpdateStatusFunc {
	return func(s *helmAppStatus) bool {
		return s.Conditions.SetCondition(status.Condition{
			Type:   t,
			Status: corev1.ConditionUnknown,
		})
	}
}

// EnsureDeployedRelease ensure that deploy status will be set in the release/CR
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

// RemoveDeployedRelease will call UpdateStatusFunc without a release
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
