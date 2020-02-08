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

package helm

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"

	"github.com/go-logr/logr"
	"github.com/operator-framework/helm-operator/internal/conditions"
	helmclient "github.com/operator-framework/helm-operator/pkg/client"
	"github.com/operator-framework/helm-operator/pkg/hooks"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/operator-framework/helm-operator/internal/controllerutil"
	"github.com/operator-framework/helm-operator/internal/predicate"
	"github.com/operator-framework/helm-operator/internal/updater"
	"github.com/operator-framework/helm-operator/internal/values"
)

// reconciler reconciles a Helm object
type reconciler struct {
	client             client.Client
	scheme             *runtime.Scheme
	actionClientGetter helmclient.ActionClientGetter
	eventRecorder      record.EventRecorder
	hooks              []hooks.ReleaseHook

	log             logr.Logger
	gvk             *schema.GroupVersionKind
	chrt            *chart.Chart
	overrideValues  map[string]string
	addWatchesFor   func(*release.Release) error
	watchDependents *bool
}

func New(opts ...ReconcilerOption) (*reconciler, error) {
	r := &reconciler{}
	for _, o := range opts {
		if err := o(r); err != nil {
			return nil, err
		}
	}

	if err := r.validate(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *reconciler) SetupWithManager(mgr ctrl.Manager) error {
	controllerName := fmt.Sprintf("%v-controller", strings.ToLower(r.gvk.Kind))

	if err := r.addDefaults(mgr, controllerName); err != nil {
		return err
	}

	r.setupScheme(mgr)

	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r, MaxConcurrentReconciles: 4})
	if err != nil {
		return err
	}

	if err := r.setupWatches(mgr, c); err != nil {
		return err
	}

	r.log.Info("Watching resource",
		"group", r.gvk.Group,
		"version", r.gvk.Version,
		"kind", r.gvk.Kind,
	)

	return nil
}

type ReconcilerOption func(r *reconciler) error

func WithClient(cl client.Client) ReconcilerOption {
	return func(r *reconciler) error {
		r.client = cl
		return nil
	}
}

func WithScheme(scheme *runtime.Scheme) ReconcilerOption {
	return func(r *reconciler) error {
		r.scheme = scheme
		return nil
	}
}

func WithActionClientGetter(actionClientGetter helmclient.ActionClientGetter) ReconcilerOption {
	return func(r *reconciler) error {
		r.actionClientGetter = actionClientGetter
		return nil
	}
}

func WithEventRecorder(er record.EventRecorder) ReconcilerOption {
	return func(r *reconciler) error {
		r.eventRecorder = er
		return nil
	}
}

func WithLog(log logr.Logger) ReconcilerOption {
	return func(r *reconciler) error {
		r.log = log
		return nil
	}
}

func WithGroupVersionKind(gvk schema.GroupVersionKind) ReconcilerOption {
	return func(r *reconciler) error {
		r.gvk = &gvk
		return nil
	}
}

func WithChart(chrt *chart.Chart) ReconcilerOption {
	return func(r *reconciler) error {
		r.chrt = chrt
		return nil
	}
}

func WithOverrideValues(overrides map[string]string) ReconcilerOption {
	return func(r *reconciler) error {
		// Validate that overrides can be parsed and applied
		// so that we fail fast during operator setup rather
		// than during the first reconciliation.
		m := values.New(map[string]interface{}{})
		if err := m.ApplyOverrides(overrides); err != nil {
			return err
		}

		r.overrideValues = overrides
		return nil
	}
}

func WithDependentWatchesEnabled(enable bool) ReconcilerOption {
	return func(r *reconciler) error {
		r.watchDependents = &enable
		return nil
	}
}

func (r *reconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.log.WithValues(strings.ToLower(r.gvk.Kind), req.NamespacedName, "id", rand.Int())

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(*r.gvk)
	err := r.client.Get(ctx, req.NamespacedName, obj)
	if apierrors.IsNotFound(err) {
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	helmClient, err := r.actionClientGetter.ActionClientFor(obj)
	if err != nil {
		return ctrl.Result{}, err
	}

	crVals, err := values.FromUnstructured(obj)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err := crVals.ApplyOverrides(r.overrideValues); err != nil {
		return ctrl.Result{}, err
	}
	vals, err := chartutil.CoalesceValues(r.chrt, crVals.Map())
	if err != nil {
		return ctrl.Result{}, err
	}

	u := updater.New(r.client, obj)

	rel, state, err := r.getReleaseState(helmClient, obj, vals.AsMap())
	if err != nil {
		u.UpdateStatus(updater.EnsureCondition(conditions.Irreconcilable(err)))
		return ctrl.Result{}, err
	}

	_ = helmClient
	_ = vals
	_ = u
	_ = rel
	_ = state

	u.UpdateStatus(updater.RemoveCondition(conditions.TypeIrreconcilable))

	if state == stateAlreadyUninstalled {
		log.Info("Resource is terminated, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	if state == stateNeedsUninstall {
		if err := func() (err error) {
			defer func() {
				updateErr := u.Apply(ctx, obj)
				if err == nil {
					err = updateErr
				}
			}()
			resp, err := helmClient.Uninstall(obj.GetName())
			if errors.Is(err, driver.ErrReleaseNotFound) {
				log.Info("Release not found, removing finalizer")
			} else if err != nil {
				log.Error(err, "Failed to uninstall release")
				u.UpdateStatus(updater.EnsureCondition(conditions.ReleaseFailed(conditions.ReasonUninstallError, err)))
				return err
			} else {
				log.Info("Release uninstalled", "name", resp.Release.Name, "version", resp.Release.Version)
			}
			u.Update(updater.RemoveFinalizer(uninstallFinalizer))
			u.UpdateStatus(
				updater.RemoveCondition(conditions.TypeReleaseFailed),
				updater.EnsureCondition(conditions.Deployed(corev1.ConditionFalse, conditions.ReasonUninstallSuccessful, "")),
				updater.RemoveDeployedRelease(),
			)
			return nil
		}(); err != nil {
			return ctrl.Result{}, err
		}

		// Since the client is hitting a cache, waiting for the
		// deletion here will guarantee that the next reconciliation
		// will see that the CR has been deleted and that there's
		// nothing left to do.
		if err := controllerutil.WaitForDeletion(ctx, r.client, obj); err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	defer func() {
		updateErr := u.Apply(ctx, obj)
		if err == nil {
			err = updateErr
		}
	}()
	u.Update(updater.EnsureFinalizer(uninstallFinalizer))
	u.UpdateStatus(
		updater.EnsureCondition(conditions.Initialized()),
		updater.EnsureDeployedRelease(rel),
	)

	switch state {
	case stateNeedsInstall:
		rel, err = helmClient.Install(obj.GetName(), obj.GetNamespace(), r.chrt, vals.AsMap())
		if err != nil {
			u.UpdateStatus(updater.EnsureCondition(conditions.ReleaseFailed(conditions.ReasonInstallError, err)))
			return ctrl.Result{}, err
		}
		for k, v := range r.overrideValues {
			r.eventRecorder.Eventf(obj, "Warning", "ValueOverridden",
				"Chart value %q overridden to %q by operator", k, v)
		}
		u.UpdateStatus(updater.EnsureCondition(conditions.Deployed(corev1.ConditionTrue, conditions.ReasonInstallSuccessful, rel.Info.Notes)))
		log.Info("Release installed", "name", rel.Name, "version", rel.Version)

	case stateNeedsUpgrade:
		rel, err = helmClient.Upgrade(obj.GetName(), obj.GetNamespace(), r.chrt, vals.AsMap())
		if err != nil {
			u.UpdateStatus(updater.EnsureCondition(conditions.ReleaseFailed(conditions.ReasonUpgradeError, err)))
			return ctrl.Result{}, err
		}
		for k, v := range r.overrideValues {
			r.eventRecorder.Eventf(obj, "Warning", "ValueOverridden",
				"Chart value %q overridden to %q by operator", k, v)
		}
		u.UpdateStatus(updater.EnsureCondition(conditions.Deployed(corev1.ConditionTrue, conditions.ReasonUpgradeSuccessful, rel.Info.Notes)))
		log.Info("Release upgraded", "name", rel.Name, "version", rel.Version)

	case stateUnchanged:
		// If a change is made to the CR spec that causes a release failure, a
		// ConditionReleaseFailed is added to the status conditions. If that change
		// is then reverted to its previous state, the operator will stop
		// attempting the release and will resume reconciling. In this case, we
		// need to remove the ConditionReleaseFailed because the failing release is
		// no longer being attempted.
		u.UpdateStatus(updater.RemoveCondition(conditions.TypeReleaseFailed))

		if err := helmClient.Reconcile(rel); err != nil {
			u.UpdateStatus(updater.EnsureCondition(conditions.Irreconcilable(err)))
			return ctrl.Result{}, err
		}
		u.UpdateStatus(updater.RemoveCondition(conditions.TypeIrreconcilable))
		log.Info("Release reconciled", "name", rel.Name, "version", rel.Version)

	default:
		return ctrl.Result{}, fmt.Errorf("unexpected release state: %s", state)
	}

	for _, hook := range r.hooks {
		if err := hook.Exec(rel); err != nil {
			log.Error(err, "failed to execute release hook", "name", rel.Name, "version", rel.Version)
		}
	}

	u.UpdateStatus(
		updater.RemoveCondition(conditions.TypeReleaseFailed),
		updater.EnsureDeployedRelease(rel),
	)

	return ctrl.Result{}, nil
}

type helmReleaseState string

const (
	stateNeedsInstall       helmReleaseState = "needs install"
	stateNeedsUpgrade       helmReleaseState = "needs upgrade"
	stateNeedsUninstall     helmReleaseState = "needs uninstall"
	stateUnchanged          helmReleaseState = "unchanged"
	stateAlreadyUninstalled helmReleaseState = "already uninstalled"
	stateError              helmReleaseState = "error"
)

func (r *reconciler) getReleaseState(client helmclient.ActionInterface, obj *unstructured.Unstructured, vals map[string]interface{}) (*release.Release, helmReleaseState, error) {
	deployedRelease, err := client.Get(obj.GetName())
	if err != nil && !errors.Is(err, driver.ErrReleaseNotFound) {
		return nil, stateError, err
	}

	if obj.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(obj, uninstallFinalizer) {
			return deployedRelease, stateNeedsUninstall, nil
		}
		return deployedRelease, stateAlreadyUninstalled, nil
	}
	if errors.Is(err, driver.ErrReleaseNotFound) {
		return nil, stateNeedsInstall, nil
	}

	specRelease, err := client.Upgrade(obj.GetName(), obj.GetNamespace(), r.chrt, vals, func(u *action.Upgrade) error {
		u.DryRun = true
		return nil
	})
	if err != nil {
		return deployedRelease, stateError, err
	}
	if specRelease.Manifest != deployedRelease.Manifest {
		return deployedRelease, stateNeedsUpgrade, nil
	}
	return deployedRelease, stateUnchanged, nil
}

const uninstallFinalizer = "uninstall-reconciler-release"

func (r *reconciler) validate() error {
	if r.log == nil {
		return errors.New("log must not be nil")
	}
	if r.gvk == nil {
		return errors.New("gvk must not be nil")
	}
	if r.chrt == nil {
		return errors.New("chart must not be nil")
	}
	return nil
}

func (r *reconciler) addDefaults(mgr ctrl.Manager, controllerName string) error {
	trueVal := true
	if r.watchDependents == nil {
		r.watchDependents = &trueVal
	}

	if r.client == nil {
		r.client = mgr.GetClient()
	}
	if r.actionClientGetter == nil {
		actionConfigGetter, err := helmclient.NewActionConfigGetter(mgr.GetConfig(), mgr.GetRESTMapper(), r.log)
		if err != nil {
			return err
		}
		r.actionClientGetter = helmclient.NewActionClientGetter(actionConfigGetter)
	}
	if r.scheme == nil {
		r.scheme = mgr.GetScheme()
	}
	if r.eventRecorder == nil {
		r.eventRecorder = mgr.GetEventRecorderFor(controllerName)
	}
	return nil
}

func (r *reconciler) setupScheme(mgr ctrl.Manager) {
	mgr.GetScheme().AddKnownTypeWithName(*r.gvk, &unstructured.Unstructured{})
	metav1.AddToGroupVersion(mgr.GetScheme(), r.gvk.GroupVersion())
}

func (r *reconciler) setupWatches(mgr ctrl.Manager, c controller.Controller) error {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(*r.gvk)
	if err := c.Watch(
		&source.Kind{Type: obj},
		&handler.EnqueueRequestForObject{},
		predicate.GenerationChangedPredicate{},
	); err != nil {
		return err
	}

	secret := &corev1.Secret{}
	secret.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Secret",
	})

	if err := c.Watch(
		&source.Kind{Type: secret},
		&handler.EnqueueRequestForOwner{
			OwnerType:    obj,
			IsController: true,
		},
	); err != nil {
		return err
	}

	if *r.watchDependents {
		r.hooks = append([]hooks.ReleaseHook{hooks.NewDependentResourceWatcher(c, mgr.GetRESTMapper(), obj, r.log)}, r.hooks...)
	}
	return nil
}
