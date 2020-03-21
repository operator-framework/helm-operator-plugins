// Copyright 2020 The Operator-SDK Authors.
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

package reconciler

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/go-logr/logr"
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

	"github.com/operator-framework/helm-operator/internal/conditions"
	"github.com/operator-framework/helm-operator/internal/controllerutil"
	internalhook "github.com/operator-framework/helm-operator/internal/hook"
	"github.com/operator-framework/helm-operator/internal/migrator"
	"github.com/operator-framework/helm-operator/internal/predicate"
	"github.com/operator-framework/helm-operator/internal/updater"
	"github.com/operator-framework/helm-operator/internal/values"
	helmclient "github.com/operator-framework/helm-operator/pkg/client"
	"github.com/operator-framework/helm-operator/pkg/hook"
)

const uninstallFinalizer = "uninstall-reconciler-release"

// reconciler reconciles a Helm object
type reconciler struct {
	client             client.Client
	scheme             *runtime.Scheme
	actionClientGetter helmclient.ActionClientGetter
	eventRecorder      record.EventRecorder
	hooks              []hook.Hook
	migratorGetter     migrator.MigratorGetter

	log                     logr.Logger
	gvk                     *schema.GroupVersionKind
	chrt                    *chart.Chart
	overrideValues          map[string]string
	watchDependents         *bool
	maxConcurrentReconciles int
	reconcilePeriod         time.Duration
}

// New creates a new Reconciler that reconciles custom resources that
// define a Helm release. New takes variadic ReconcilerOption arguments
// that are used to configure the Reconciler.
//
// Required options are:
//   - WithGroupVersionKind
//   - WithChart
//
// Other options are defaulted to sane defaults when SetupWithManager
// is called.
//
// If an error occurs configuring or validating the reconciler, it is
// returned.
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

// SetupWithManager configures a controller for the reconciler and registers
// watches. It also uses the passed Manager to initialize default values for the
// reconciler and sets up the manager's scheme with the reconciler's configured
// GroupVersionKind.
//
// If an error occurs setting up the reconciler with the manager, it is returned.
func (r *reconciler) SetupWithManager(mgr ctrl.Manager) error {
	controllerName := fmt.Sprintf("%v-controller", strings.ToLower(r.gvk.Kind))

	if err := r.addDefaults(mgr, controllerName); err != nil {
		return err
	}

	r.setupScheme(mgr)

	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r, MaxConcurrentReconciles: r.maxConcurrentReconciles})
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

// ReoncilerOption is a function that configures the
// helm reconciler.
type ReconcilerOption func(r *reconciler) error

// WithClient is a ReconcilerOption that configures
// a reconciler's client.
//
// By default, manager.GetClient() is used if this
// option is not configured.
func WithClient(cl client.Client) ReconcilerOption {
	return func(r *reconciler) error {
		r.client = cl
		return nil
	}
}

// WithScheme is a ReconcilerOption that configures
// a reconciler's scheme.
//
// By default, manager.GetScheme() is used if this
// option is not configured.
func WithScheme(scheme *runtime.Scheme) ReconcilerOption {
	return func(r *reconciler) error {
		r.scheme = scheme
		return nil
	}
}

// WithActionClientGetter is a ReconcilerOption that
// configures a reconciler's ActionClientGetter.
//
// A default ActionClientGetter is used if this
// option is not configured.
func WithActionClientGetter(actionClientGetter helmclient.ActionClientGetter) ReconcilerOption {
	return func(r *reconciler) error {
		r.actionClientGetter = actionClientGetter
		return nil
	}
}

// WithEventRecorder is a ReconcilerOption that configures a
// reconciler's EventRecorder.
//
// By default, manager.GetEventRecorderFor() is used if this
// option is not configured.
func WithEventRecorder(er record.EventRecorder) ReconcilerOption {
	return func(r *reconciler) error {
		r.eventRecorder = er
		return nil
	}
}

// WithLog is a ReconcilerOption that configures
// a reconciler's logger.
//
// A default logger is used if this option is
// not configured.
func WithLog(log logr.Logger) ReconcilerOption {
	return func(r *reconciler) error {
		r.log = log
		return nil
	}
}

// WithGroupVersionKind is a ReconcilerOption that
// configures a reconciler's GroupVersionKind.
//
// This option is required.
func WithGroupVersionKind(gvk schema.GroupVersionKind) ReconcilerOption {
	return func(r *reconciler) error {
		r.gvk = &gvk
		return nil
	}
}

// WithChart is a ReconcilerOption that configures
// a reconciler's helm chart.
//
// This option is required.
func WithChart(chrt *chart.Chart) ReconcilerOption {
	return func(r *reconciler) error {
		r.chrt = chrt
		return nil
	}
}

// WithOverrideValues is a ReconcilerOption that configures
// a reconciler's override values.
//
// Override values can be used to enforce that certain values
// provided by the chart's default values.yaml or by a CR spec
// are always overridden when rendering the chart. If a value
// in overrides is set by a CR, it is overridden by the override
// value. The override value can be static but can also refer to
// an environment variable.
//
// If an environment variable reference is listed in override
// values but is not present in the environment when this function
// runs, it will resolve to an empty string and override all other
// values. Therefore, when using environment variable expansion,
// ensure that the environment variable is set.
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

// WithDependentWatchesEnabled is a ReconcilerOption that configures
// whether the reconciler will register watches for dependent objects
// in releases and trigger reconciliations when they change.
//
// By default, dependent watches are enabled.
func WithDependentWatchesEnabled(enable bool) ReconcilerOption {
	return func(r *reconciler) error {
		r.watchDependents = &enable
		return nil
	}
}

func WithMaxConcurrentReconciles(max int) ReconcilerOption {
	return func(r *reconciler) error {
		if max < 1 {
			return errors.New("maxConcurrentReconciles must be at least 1")
		}
		r.maxConcurrentReconciles = max
		return nil
	}
}

func WithReconcilePeriod(rp time.Duration) ReconcilerOption {
	return func(r *reconciler) error {
		if rp < 0 {
			return errors.New("reconcile period must not be negative")
		}
		r.reconcilePeriod = rp
		return nil
	}
}

// Reconcile reconciles a CR that defines a Helm v3 release.
//
// If a v2 release exists for the CR, it is automatically migrated to the
// v3 storage format, and the old v2 release is deleted if the migration
// succeeds.
//
//   - If a release does not exist for this CR, a new release is installed.
//   - If a release exists and the CR spec has changed since the last,
//     reconciliation, the release is upgraded.
//   - If a release exists and the CR spec has not changed since the last
//     reconciliation, the release is reconciled. Any dependent resources that
//     have diverged from the release manifest are re-created or patched so
//     that they are re-aligned with the release.
//   - If the CR has been deleted, the release will be uninstalled. The
//     reconciler uses a finalizer to ensure the release uninstall succeeds
//     before CR deletion occurs.
//
// If an error occurs during release installation or upgrade, the change will
// be rolled back to restore the previous state.
//
// Reconcile also manages the status field of the custom resource. It includes
// the release name and manifest in `status.deployedRelease`, and it updates
// `status.conditions` based on reconciliation progress and success. Condition
// types include:
//
//   - Initialized - initial reconciler-managed fields added (e.g. the uninstall
//                   finalizer
//   - Deployed - a release for this CR is deployed (but not necessarily ready).
//   - ReleaseFailed - an installation or upgrade failed.
//   - Irreconcilable - an error occurred during reconciliation
func (r *reconciler) Reconcile(req ctrl.Request) (res ctrl.Result, err error) {
	ctx := context.Background()
	log := r.log.WithValues(strings.ToLower(r.gvk.Kind), req.NamespacedName, "id", rand.Int())

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(*r.gvk)
	err = r.client.Get(ctx, req.NamespacedName, obj)
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

	vals, err := r.getValues(obj)
	if err != nil {
		return ctrl.Result{}, err
	}

	u := updater.New(r.client, obj)

	releaseMigrator := r.migratorGetter.MigratorFor(obj)
	if err := releaseMigrator.Migrate(); err != nil {
		u.UpdateStatus(updater.EnsureCondition(conditions.Irreconcilable(err)))
		return ctrl.Result{}, err
	}

	rel, state, err := r.getReleaseState(helmClient, obj, vals.AsMap())
	if err != nil {
		u.UpdateStatus(updater.EnsureCondition(conditions.Irreconcilable(err)))
		return ctrl.Result{}, err
	}
	u.UpdateStatus(updater.RemoveCondition(conditions.TypeIrreconcilable))

	if state == stateAlreadyUninstalled {
		log.Info("Resource is terminated, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	if state == stateNeedsUninstall {
		if err := func() error {
			defer func() {
				applyErr := u.Apply(ctx, obj)
				if err == nil {
					err = applyErr
				}
			}()
			return r.doUninstall(helmClient, &u, obj, log)
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
		applyErr := u.Apply(ctx, obj)
		if err == nil {
			err = applyErr
		}
	}()
	u.Update(updater.EnsureFinalizer(uninstallFinalizer))
	u.UpdateStatus(
		updater.EnsureCondition(conditions.Initialized()),
		updater.EnsureDeployedRelease(rel),
	)

	switch state {
	case stateNeedsInstall:
		rel, err = r.doInstall(helmClient, &u, obj, vals.AsMap(), log)
		if err != nil {
			return ctrl.Result{}, err
		}

	case stateNeedsUpgrade:
		rel, err = r.doUpgrade(helmClient, &u, obj, vals.AsMap(), log)
		if err != nil {
			return ctrl.Result{}, err
		}

	case stateUnchanged:
		if err := r.doReconcile(helmClient, &u, rel, log); err != nil {
			return ctrl.Result{}, err
		}
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

	return ctrl.Result{RequeueAfter: r.reconcilePeriod}, nil
}

func (r *reconciler) getValues(obj *unstructured.Unstructured) (*chartutil.Values, error) {
	crVals, err := values.FromUnstructured(obj)
	if err != nil {
		return nil, err
	}
	if err := crVals.ApplyOverrides(r.overrideValues); err != nil {
		return nil, err
	}
	vals, err := chartutil.CoalesceValues(r.chrt, crVals.Map())
	return &vals, err
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

func (r *reconciler) doInstall(helmClient helmclient.ActionInterface, u *updater.Updater, obj *unstructured.Unstructured, vals map[string]interface{}, log logr.Logger) (*release.Release, error) {
	rel, err := helmClient.Install(obj.GetName(), obj.GetNamespace(), r.chrt, vals)
	if err != nil {
		u.UpdateStatus(updater.EnsureCondition(conditions.ReleaseFailed(conditions.ReasonInstallError, err)))
		return nil, err
	}
	for k, v := range r.overrideValues {
		r.eventRecorder.Eventf(obj, "Warning", "ValueOverridden",
			"Chart value %q overridden to %q by operator", k, v)
	}
	u.UpdateStatus(updater.EnsureCondition(conditions.Deployed(corev1.ConditionTrue, conditions.ReasonInstallSuccessful, rel.Info.Notes)))
	log.Info("Release installed", "name", rel.Name, "version", rel.Version)
	return rel, nil
}

func (r *reconciler) doUpgrade(helmClient helmclient.ActionInterface, u *updater.Updater, obj *unstructured.Unstructured, vals map[string]interface{}, log logr.Logger) (*release.Release, error) {
	rel, err := helmClient.Upgrade(obj.GetName(), obj.GetNamespace(), r.chrt, vals)
	if err != nil {
		u.UpdateStatus(updater.EnsureCondition(conditions.ReleaseFailed(conditions.ReasonUpgradeError, err)))
		return nil, err
	}
	for k, v := range r.overrideValues {
		r.eventRecorder.Eventf(obj, "Warning", "ValueOverridden",
			"Chart value %q overridden to %q by operator", k, v)
	}
	u.UpdateStatus(updater.EnsureCondition(conditions.Deployed(corev1.ConditionTrue, conditions.ReasonUpgradeSuccessful, rel.Info.Notes)))
	log.Info("Release upgraded", "name", rel.Name, "version", rel.Version)
	return rel, nil
}

func (r *reconciler) doReconcile(helmClient helmclient.ActionInterface, u *updater.Updater, rel *release.Release, log logr.Logger) error {
	// If a change is made to the CR spec that causes a release failure, a
	// ConditionReleaseFailed is added to the status conditions. If that change
	// is then reverted to its previous state, the operator will stop
	// attempting the release and will resume reconciling. In this case, we
	// need to remove the ConditionReleaseFailed because the failing release is
	// no longer being attempted.
	u.UpdateStatus(updater.RemoveCondition(conditions.TypeReleaseFailed))

	if err := helmClient.Reconcile(rel); err != nil {
		u.UpdateStatus(updater.EnsureCondition(conditions.Irreconcilable(err)))
		return err
	}
	u.UpdateStatus(updater.RemoveCondition(conditions.TypeIrreconcilable))
	log.Info("Release reconciled", "name", rel.Name, "version", rel.Version)
	return nil
}

func (r *reconciler) doUninstall(helmClient helmclient.ActionInterface, u *updater.Updater, obj *unstructured.Unstructured, log logr.Logger) error {
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
}

func (r *reconciler) validate() error {
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
	if r.log == nil {
		r.log = ctrl.Log.WithName("controllers").WithName("Helm")
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
	if r.migratorGetter == nil {
		migratorGetter, err := migrator.NewMigratorGetter(mgr.GetConfig())
		if err != nil {
			return err
		}
		r.migratorGetter = migratorGetter
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
		r.hooks = append([]hook.Hook{internalhook.NewDependentResourceWatcher(c, mgr.GetRESTMapper(), obj, r.log)}, r.hooks...)
	}
	return nil
}
