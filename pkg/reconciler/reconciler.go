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

	"github.com/joelanford/helm-operator/pkg/annotation"
	helmclient "github.com/joelanford/helm-operator/pkg/client"
	"github.com/joelanford/helm-operator/pkg/hook"
	"github.com/joelanford/helm-operator/pkg/reconciler/internal/conditions"
	"github.com/joelanford/helm-operator/pkg/reconciler/internal/controllerutil"
	internalhook "github.com/joelanford/helm-operator/pkg/reconciler/internal/hook"
	"github.com/joelanford/helm-operator/pkg/reconciler/internal/updater"
	"github.com/joelanford/helm-operator/pkg/reconciler/internal/values"
)

const uninstallFinalizer = "uninstall-helm-release"

// Reconciler reconciles a Helm object
type Reconciler struct {
	client             client.Client
	actionClientGetter helmclient.ActionClientGetter
	valueMapper        ValueMapper
	eventRecorder      record.EventRecorder
	preHooks           []hook.PreHook
	postHooks          []hook.PostHook

	log                     logr.Logger
	gvk                     *schema.GroupVersionKind
	chrt                    *chart.Chart
	overrideValues          map[string]string
	watchDependents         *bool
	maxConcurrentReconciles int
	reconcilePeriod         time.Duration

	annotations          map[string]struct{}
	installAnnotations   map[string]annotation.Install
	upgradeAnnotations   map[string]annotation.Upgrade
	uninstallAnnotations map[string]annotation.Uninstall
}

// New creates a new Reconciler that reconciles custom resources that define a
// Helm release. New takes variadic Option arguments that are used to configure
// the Reconciler.
//
// Required options are:
//   - WithGroupVersionKind
//   - WithChart
//
// Other options are defaulted to sane defaults when SetupWithManager is called.
//
// If an error occurs configuring or validating the Reconciler, it is returned.
func New(opts ...Option) (*Reconciler, error) {
	r := &Reconciler{
		annotations:          make(map[string]struct{}),
		installAnnotations:   make(map[string]annotation.Install),
		upgradeAnnotations:   make(map[string]annotation.Upgrade),
		uninstallAnnotations: make(map[string]annotation.Uninstall),
	}
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

// SetupWithManager configures a controller for the Reconciler and registers
// watches. It also uses the passed Manager to initialize default values for the
// Reconciler and sets up the manager's scheme with the Reconciler's configured
// GroupVersionKind.
//
// If an error occurs setting up the Reconciler with the manager, it is
// returned.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
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

// Option is a function that configures the helm Reconciler.
type Option func(r *Reconciler) error

// WithClient is an Option that configures a Reconciler's client.
//
// By default, manager.GetClient() is used if this option is not configured.
func WithClient(cl client.Client) Option {
	return func(r *Reconciler) error {
		r.client = cl
		return nil
	}
}

// WithActionClientGetter is an Option that configures a Reconciler's
// ActionClientGetter.
//
// A default ActionClientGetter is used if this option is not configured.
func WithActionClientGetter(actionClientGetter helmclient.ActionClientGetter) Option {
	return func(r *Reconciler) error {
		r.actionClientGetter = actionClientGetter
		return nil
	}
}

// WithEventRecorder is an Option that configures a Reconciler's EventRecorder.
//
// By default, manager.GetEventRecorderFor() is used if this option is not
// configured.
func WithEventRecorder(er record.EventRecorder) Option {
	return func(r *Reconciler) error {
		r.eventRecorder = er
		return nil
	}
}

// WithLog is an Option that configures a Reconciler's logger.
//
// A default logger is used if this option is not configured.
func WithLog(log logr.Logger) Option {
	return func(r *Reconciler) error {
		r.log = log
		return nil
	}
}

// WithGroupVersionKind is an Option that configures a Reconciler's
// GroupVersionKind.
//
// This option is required.
func WithGroupVersionKind(gvk schema.GroupVersionKind) Option {
	return func(r *Reconciler) error {
		r.gvk = &gvk
		return nil
	}
}

// WithChart is an Option that configures a Reconciler's helm chart.
//
// This option is required.
func WithChart(chrt *chart.Chart) Option {
	return func(r *Reconciler) error {
		r.chrt = chrt
		return nil
	}
}

// WithOverrideValues is an Option that configures a Reconciler's override
// values.
//
// Override values can be used to enforce that certain values provided by the
// chart's default values.yaml or by a CR spec are always overridden when
// rendering the chart. If a value in overrides is set by a CR, it is
// overridden by the override value. The override value can be static but can
// also refer to an environment variable.
//
// If an environment variable reference is listed in override values but is not
// present in the environment when this function runs, it will resolve to an
// empty string and override all other values. Therefore, when using
// environment variable expansion, ensure that the environment variable is set.
func WithOverrideValues(overrides map[string]string) Option {
	return func(r *Reconciler) error {
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

// WithDependentWatchesEnabled is an Option that configures whether the
// Reconciler will register watches for dependent objects in releases and
// trigger reconciliations when they change.
//
// By default, dependent watches are enabled.
func WithDependentWatchesEnabled(enable bool) Option {
	return func(r *Reconciler) error {
		r.watchDependents = &enable
		return nil
	}
}

// WithMaxConcurrentReconciles is an Option that configures the number of
// concurrent reconciles that the controller will run.
//
// The default is 1.
func WithMaxConcurrentReconciles(max int) Option {
	return func(r *Reconciler) error {
		if max < 1 {
			return errors.New("maxConcurrentReconciles must be at least 1")
		}
		r.maxConcurrentReconciles = max
		return nil
	}
}

// WithReconcilePeriod is an Option that configures the reconcile period of the
// controller. This will cause the controller to reconcile CRs at least once
// every period. By default, the reconcile period is set to 0, which means no
// time-based reconciliations will occur.
func WithReconcilePeriod(rp time.Duration) Option {
	return func(r *Reconciler) error {
		if rp < 0 {
			return errors.New("reconcile period must not be negative")
		}
		r.reconcilePeriod = rp
		return nil
	}
}

// WithInstallAnnotation is an Option that configures an Install annotation
// to enable custom action.Install fields to be set based on the value of
// annotations found in the custom resource watched by this reconciler.
// Duplicate annotation names will result in an error.
func WithInstallAnnotation(a annotation.Install) Option {
	return func(r *Reconciler) error {
		name := a.Name()
		if _, ok := r.annotations[name]; ok {
			return fmt.Errorf("annotation %q already exists", name)
		}

		r.annotations[name] = struct{}{}
		r.installAnnotations[name] = a
		return nil
	}
}

// WithUpgradeAnnotation is an Option that configures an Upgrade annotation
// to enable custom action.Upgrade fields to be set based on the value of
// annotations found in the custom resource watched by this reconciler.
// Duplicate annotation names will result in an error.
func WithUpgradeAnnotation(a annotation.Upgrade) Option {
	return func(r *Reconciler) error {
		name := a.Name()
		if _, ok := r.annotations[name]; ok {
			return fmt.Errorf("annotation %q already exists", name)
		}

		r.annotations[name] = struct{}{}
		r.upgradeAnnotations[name] = a
		return nil
	}
}

// WithUninstallAnnotation is an Option that configures an Uninstall annotation
// to enable custom action.Uninstall fields to be set based on the value of
// annotations found in the custom resource watched by this reconciler.
// Duplicate annotation names will result in an error.
func WithUninstallAnnotation(a annotation.Uninstall) Option {
	return func(r *Reconciler) error {
		name := a.Name()
		if _, ok := r.annotations[name]; ok {
			return fmt.Errorf("annotation %q already exists", name)
		}

		r.annotations[name] = struct{}{}
		r.uninstallAnnotations[name] = a
		return nil
	}
}

// WithPreHook is an Option that configures the reconciler to run the given
// PreHook just before performing any actions (e.g. install, upgrade, uninstall,
// or reconciliation).
func WithPreHook(h hook.PreHook) Option {
	return func(r *Reconciler) error {
		r.preHooks = append(r.preHooks, h)
		return nil
	}
}

// WithPostHook is an Option that configures the reconciler to run the given
// PostHook just after performing any non-uninstall release actions.
func WithPostHook(h hook.PostHook) Option {
	return func(r *Reconciler) error {
		r.postHooks = append(r.postHooks, h)
		return nil
	}
}

// WithValueMapper is an Option that configures a function that maps values
// from a custom resource spec to the values passed to Helm
func WithValueMapper(m ValueMapper) Option {
	return func(r *Reconciler) error {
		r.valueMapper = m
		return nil
	}
}

// Reconcile reconciles a CR that defines a Helm v3 release.
//
//   - If a release does not exist for this CR, a new release is installed.
//   - If a release exists and the CR spec has changed since the last,
//     reconciliation, the release is upgraded.
//   - If a release exists and the CR spec has not changed since the last
//     reconciliation, the release is reconciled. Any dependent resources that
//     have diverged from the release manifest are re-created or patched so that
//     they are re-aligned with the release.
//   - If the CR has been deleted, the release will be uninstalled. The
//     Reconciler uses a finalizer to ensure the release uninstall succeeds
//     before CR deletion occurs.
//
// If an error occurs during release installation or upgrade, the change will be
// rolled back to restore the previous state.
//
// Reconcile also manages the status field of the custom resource. It includes
// the release name and manifest in `status.deployedRelease`, and it updates
// `status.conditions` based on reconciliation progress and success. Condition
// types include:
//
//   - Initialized - initial Reconciler-managed fields added (e.g. the uninstall
//                   finalizer
//   - Deployed - a release for this CR is deployed (but not necessarily ready).
//   - ReleaseFailed - an installation or upgrade failed.
//   - Irreconcilable - an error occurred during reconciliation
func (r *Reconciler) Reconcile(req ctrl.Request) (res ctrl.Result, err error) {
	ctx := context.Background()
	log := r.log.WithValues(strings.ToLower(r.gvk.Kind), req.NamespacedName)

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(*r.gvk)
	err = r.client.Get(ctx, req.NamespacedName, obj)
	if apierrors.IsNotFound(err) {
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	actionClient, err := r.actionClientGetter.ActionClientFor(obj)
	if err != nil {
		return ctrl.Result{}, err
	}

	vals, err := r.getValues(obj)
	if err != nil {
		return ctrl.Result{}, err
	}

	u := updater.New(r.client, obj)

	for _, h := range r.preHooks {
		if err := h.Exec(obj, &vals, log); err != nil {
			log.Error(err, "failed to execute pre-release hook")
		}
	}

	rel, state, err := r.getReleaseState(actionClient, obj, vals.AsMap())
	if err != nil {
		u.UpdateStatus(updater.EnsureCondition(conditions.Irreconcilable(err)))
		return ctrl.Result{}, err
	}
	u.UpdateStatus(updater.RemoveCondition(conditions.TypeIrreconcilable))

	switch state {
	case stateAlreadyUninstalled:
		log.Info("Resource is terminated, skipping reconciliation")
		return ctrl.Result{}, nil
	case stateNeedsUninstall:
		if err := func() error {
			defer func() {
				applyErr := u.Apply(ctx, obj)
				if err == nil {
					err = applyErr
				}
			}()
			return r.doUninstall(actionClient, &u, obj, log)
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
	u.UpdateStatus(
		updater.EnsureCondition(conditions.Initialized()),
		updater.EnsureDeployedRelease(rel),
	)

	switch state {
	case stateNeedsInstall:
		rel, err = r.doInstall(actionClient, &u, obj, vals.AsMap(), log)
		if err != nil {
			return ctrl.Result{}, err
		}

	case stateNeedsUpgrade:
		rel, err = r.doUpgrade(actionClient, &u, obj, vals.AsMap(), log)
		if err != nil {
			return ctrl.Result{}, err
		}

	case stateUnchanged:
		if err := r.doReconcile(actionClient, &u, rel, log); err != nil {
			return ctrl.Result{}, err
		}
	default:
		return ctrl.Result{}, fmt.Errorf("unexpected release state: %s", state)
	}

	u.Update(updater.EnsureFinalizer(uninstallFinalizer))
	u.UpdateStatus(
		updater.RemoveCondition(conditions.TypeReleaseFailed),
		updater.EnsureDeployedRelease(rel),
	)

	for _, h := range r.postHooks {
		if err := h.Exec(obj, rel, log); err != nil {
			log.Error(err, "failed to execute post-release hook", "name", rel.Name, "version", rel.Version)
		}
	}

	return ctrl.Result{RequeueAfter: r.reconcilePeriod}, nil
}

func (r *Reconciler) getValues(obj *unstructured.Unstructured) (chartutil.Values, error) {
	crVals, err := values.FromUnstructured(obj)
	if err != nil {
		return chartutil.Values{}, err
	}
	if err := crVals.ApplyOverrides(r.overrideValues); err != nil {
		return chartutil.Values{}, err
	}
	vals := crVals.Map()
	if r.valueMapper != nil {
		vals = r.valueMapper.MapValues(vals)
	}
	vals, err = chartutil.CoalesceValues(r.chrt, vals)
	if err != nil {
		return chartutil.Values{}, err
	}
	return vals, nil
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

func (r *Reconciler) getReleaseState(client helmclient.ActionInterface, obj metav1.Object, vals map[string]interface{}) (*release.Release, helmReleaseState, error) {
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

func (r *Reconciler) doInstall(actionClient helmclient.ActionInterface, u *updater.Updater, obj *unstructured.Unstructured, vals map[string]interface{}, log logr.Logger) (*release.Release, error) {
	var opts []helmclient.InstallOption
	for name, annot := range r.installAnnotations {
		if v, ok := obj.GetAnnotations()[name]; ok {
			opts = append(opts, annot.InstallOption(v))
		}
	}
	rel, err := actionClient.Install(obj.GetName(), obj.GetNamespace(), r.chrt, vals, opts...)
	if err != nil {
		u.UpdateStatus(updater.EnsureCondition(conditions.ReleaseFailed(conditions.ReasonInstallError, err)))
		return nil, err
	}
	r.reportOverrideEvents(obj)
	u.UpdateStatus(updater.EnsureCondition(conditions.Deployed(corev1.ConditionTrue, conditions.ReasonInstallSuccessful, rel.Info.Notes)))
	log.Info("Release installed", "name", rel.Name, "version", rel.Version)
	return rel, nil
}

func (r *Reconciler) doUpgrade(actionClient helmclient.ActionInterface, u *updater.Updater, obj *unstructured.Unstructured, vals map[string]interface{}, log logr.Logger) (*release.Release, error) {
	var opts []helmclient.UpgradeOption
	for name, annot := range r.upgradeAnnotations {
		if v, ok := obj.GetAnnotations()[name]; ok {
			opts = append(opts, annot.UpgradeOption(v))
		}
	}

	rel, err := actionClient.Upgrade(obj.GetName(), obj.GetNamespace(), r.chrt, vals, opts...)
	if err != nil {
		u.UpdateStatus(updater.EnsureCondition(conditions.ReleaseFailed(conditions.ReasonUpgradeError, err)))
		return nil, err
	}
	r.reportOverrideEvents(obj)
	u.UpdateStatus(updater.EnsureCondition(conditions.Deployed(corev1.ConditionTrue, conditions.ReasonUpgradeSuccessful, rel.Info.Notes)))
	log.Info("Release upgraded", "name", rel.Name, "version", rel.Version)
	return rel, nil
}

func (r *Reconciler) reportOverrideEvents(obj runtime.Object) {
	for k, v := range r.overrideValues {
		r.eventRecorder.Eventf(obj, "Warning", "ValueOverridden",
			"Chart value %q overridden to %q by operator", k, v)
	}
}

func (r *Reconciler) doReconcile(actionClient helmclient.ActionInterface, u *updater.Updater, rel *release.Release, log logr.Logger) error {
	// If a change is made to the CR spec that causes a release failure, a
	// ConditionReleaseFailed is added to the status conditions. If that change
	// is then reverted to its previous state, the operator will stop
	// attempting the release and will resume reconciling. In this case, we
	// need to remove the ConditionReleaseFailed because the failing release is
	// no longer being attempted.
	u.UpdateStatus(updater.RemoveCondition(conditions.TypeReleaseFailed))

	if err := actionClient.Reconcile(rel); err != nil {
		u.UpdateStatus(updater.EnsureCondition(conditions.Irreconcilable(err)))
		return err
	}
	u.UpdateStatus(updater.RemoveCondition(conditions.TypeIrreconcilable))
	log.Info("Release reconciled", "name", rel.Name, "version", rel.Version)
	return nil
}

func (r *Reconciler) doUninstall(actionClient helmclient.ActionInterface, u *updater.Updater, obj *unstructured.Unstructured, log logr.Logger) error {
	var opts []helmclient.UninstallOption
	for name, annot := range r.uninstallAnnotations {
		if v, ok := obj.GetAnnotations()[name]; ok {
			opts = append(opts, annot.UninstallOption(v))
		}
	}

	resp, err := actionClient.Uninstall(obj.GetName(), opts...)
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

func (r *Reconciler) validate() error {
	if r.gvk == nil {
		return errors.New("gvk must not be nil")
	}
	if r.chrt == nil {
		return errors.New("chart must not be nil")
	}
	return nil
}

func (r *Reconciler) addDefaults(mgr ctrl.Manager, controllerName string) error {
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
	if r.eventRecorder == nil {
		r.eventRecorder = mgr.GetEventRecorderFor(controllerName)
	}
	return nil
}

func (r *Reconciler) setupScheme(mgr ctrl.Manager) {
	mgr.GetScheme().AddKnownTypeWithName(*r.gvk, &unstructured.Unstructured{})
	metav1.AddToGroupVersion(mgr.GetScheme(), r.gvk.GroupVersion())
}

func (r *Reconciler) setupWatches(mgr ctrl.Manager, c controller.Controller) error {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(*r.gvk)
	if err := c.Watch(
		&source.Kind{Type: obj},
		&handler.EnqueueRequestForObject{},
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
		r.postHooks = append([]hook.PostHook{internalhook.NewDependentResourceWatcher(c, mgr.GetRESTMapper(), obj)}, r.postHooks...)
	}
	return nil
}

type ValueMapper interface {
	MapValues(chartutil.Values) chartutil.Values
}