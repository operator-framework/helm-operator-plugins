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

package reconciler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/operator-framework/helm-operator/internal/controllerutil"
	"github.com/operator-framework/helm-operator/internal/predicate"
	"github.com/operator-framework/helm-operator/internal/status"
	"github.com/operator-framework/helm-operator/internal/updater"
	"github.com/operator-framework/helm-operator/internal/values"
	"github.com/operator-framework/helm-operator/pkg/reconcilerutil"
)

// helm reconciles a Helm object
type helm struct {
	client             client.Client
	scheme             *runtime.Scheme
	actionClientGetter reconcilerutil.ActionClientGetter
	eventRecorder      record.EventRecorder

	log             logr.Logger
	gvk             *schema.GroupVersionKind
	chrt            *chart.Chart
	overrideValues  map[string]string
	addWatchesFor   func(*release.Release, logr.Logger) error
	watchDependents *bool
}

type HelmOption func(r *helm) error

func WithClient(cl client.Client) HelmOption {
	return func(r *helm) error {
		r.client = cl
		return nil
	}
}

func WithScheme(scheme *runtime.Scheme) HelmOption {
	return func(r *helm) error {
		r.scheme = scheme
		return nil
	}
}

func WithActionClientGetter(actionClientGetter reconcilerutil.ActionClientGetter) HelmOption {
	return func(r *helm) error {
		r.actionClientGetter = actionClientGetter
		return nil
	}
}

func WithEventRecorder(er record.EventRecorder) HelmOption {
	return func(r *helm) error {
		r.eventRecorder = er
		return nil
	}
}

func WithLog(log logr.Logger) HelmOption {
	return func(r *helm) error {
		r.log = log
		return nil
	}
}

func WithGroupVersionKind(gvk schema.GroupVersionKind) HelmOption {
	return func(r *helm) error {
		r.gvk = &gvk
		return nil
	}
}

func WithChart(chrt *chart.Chart) HelmOption {
	return func(r *helm) error {
		r.chrt = chrt
		return nil
	}
}

func WithOverrideValues(overrides map[string]string) HelmOption {
	return func(r *helm) error {
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

func WithDependentWatchesEnabled(enable bool) HelmOption {
	return func(r *helm) error {
		r.watchDependents = &enable
		return nil
	}
}

func NewHelm(opts ...HelmOption) (*helm, error) {
	r := &helm{}
	for _, o := range opts {
		if err := o(r); err != nil {
			return nil, err
		}
	}

	trueVal := true
	if r.watchDependents == nil {
		r.watchDependents = &trueVal
	}
	if r.log == nil {
		return nil, errors.New("log must not be nil")
	}
	if r.gvk == nil {
		return nil, errors.New("gvk must not be nil")
	}
	if r.chrt == nil {
		return nil, errors.New("chart must not be nil")
	}

	return r, nil
}

func (r *helm) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.log.WithValues(strings.ToLower(r.gvk.Kind), req.NamespacedName, "id", rand.Int())

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(*r.gvk)
	err := r.client.Get(ctx, req.NamespacedName, obj)
	if apierrors.IsNotFound(err) {
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, plainError{err}
	}

	res, err := r.doReconcile(ctx, obj, log)
	if err != nil {
		return res, plainError{err}
	}
	return res, nil
}

func (r *helm) doReconcile(ctx context.Context, obj *unstructured.Unstructured, log logr.Logger) (res ctrl.Result, err error) {
	u := updater.New(r.client, obj)

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

	rel, state, err := r.getReleaseState(helmClient, obj, vals.AsMap())
	if err != nil {
		u.UpdateStatus(updater.EnsureCondition(irreconcilableCondition(err)))
		return ctrl.Result{}, err
	}
	u.UpdateStatus(updater.RemoveCondition(irreconcilableConditionType))

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
				u.UpdateStatus(updater.EnsureCondition(status.Condition{
					Type:    "ReleaseFailed",
					Status:  corev1.ConditionTrue,
					Reason:  "UninstallError",
					Message: err.Error(),
				}))
				return err
			} else {
				log.Info("Release uninstalled", "name", resp.Release.Name, "version", resp.Release.Version)
			}
			u.Update(updater.RemoveFinalizer(uninstallFinalizer))
			u.UpdateStatus(
				updater.RemoveCondition("ReleaseFailed"),
				updater.EnsureCondition(status.Condition{
					Type:   "Deployed",
					Status: corev1.ConditionFalse,
					Reason: "UninstallSuccessful",
				}),
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
		if err := r.waitForDeletion(obj); err != nil {
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
		updater.EnsureCondition(status.Condition{
			Type:   "Initialized",
			Status: corev1.ConditionTrue,
		}),
		updater.EnsureDeployedRelease(rel),
	)

	switch state {
	case stateNeedsInstall:
		rel, err = helmClient.Install(obj.GetName(), obj.GetNamespace(), r.chrt, vals.AsMap())
		if err != nil {
			u.UpdateStatus(updater.EnsureCondition(status.Condition{
				Type:    "ReleaseFailed",
				Status:  corev1.ConditionTrue,
				Reason:  "InstallError",
				Message: err.Error(),
			}))
			return ctrl.Result{}, err
		}
		for k, v := range r.overrideValues {
			r.eventRecorder.Eventf(obj, "Warning", "ValueOverridden",
				"Chart value %q overridden to %q by operator", k, v)
		}
		u.UpdateStatus(updater.EnsureCondition(status.Condition{
			Type:    "Deployed",
			Status:  corev1.ConditionTrue,
			Reason:  "InstallSuccessful",
			Message: rel.Info.Notes,
		}))
		log.Info("Release installed", "name", rel.Name, "version", rel.Version)

	case stateNeedsUpgrade:
		rel, err = helmClient.Upgrade(obj.GetName(), obj.GetNamespace(), r.chrt, vals.AsMap())
		if err != nil {
			u.UpdateStatus(updater.EnsureCondition(status.Condition{
				Type:    "ReleaseFailed",
				Status:  corev1.ConditionTrue,
				Reason:  "UpgradeError",
				Message: err.Error(),
			}))
			return ctrl.Result{}, err
		}
		for k, v := range r.overrideValues {
			r.eventRecorder.Eventf(obj, "Warning", "ValueOverridden",
				"Chart value %q overridden to %q by operator", k, v)
		}
		u.UpdateStatus(updater.EnsureCondition(status.Condition{
			Type:    "Deployed",
			Status:  corev1.ConditionTrue,
			Reason:  "UpgradeSuccessful",
			Message: rel.Info.Notes,
		}))
		log.Info("Release upgraded", "name", rel.Name, "version", rel.Version)

	case stateUnchanged:
		// If a change is made to the CR spec that causes a release failure, a
		// ConditionReleaseFailed is added to the status conditions. If that change
		// is then reverted to its previous state, the operator will stop
		// attempting the release and will resume reconciling. In this case, we
		// need to remove the ConditionReleaseFailed because the failing release is
		// no longer being attempted.
		u.UpdateStatus(updater.RemoveCondition("ReleaseFailed"))

		if err := helmClient.Reconcile(rel); err != nil {
			u.UpdateStatus(updater.EnsureCondition(irreconcilableCondition(err)))
			return ctrl.Result{}, err
		}
		u.UpdateStatus(updater.RemoveCondition(irreconcilableConditionType))

		log.Info("Release reconciled", "name", rel.Name, "version", rel.Version)
	default:
		return ctrl.Result{}, fmt.Errorf("unexpected release state: %s", state)
	}

	if *r.watchDependents {
		if err := r.addWatchesFor(rel, log); err != nil {
			log.Error(err, "failed to watch release resources", "name", rel.Name, "version", rel.Version)
		}
	}
	u.UpdateStatus(
		updater.RemoveCondition("ReleaseFailed"),
		updater.EnsureDeployedRelease(rel),
	)

	return ctrl.Result{}, nil
}

func (r *helm) SetupWithManager(mgr ctrl.Manager) error {
	controllerName := fmt.Sprintf("%v-controller", strings.ToLower(r.gvk.Kind))

	if r.client == nil {
		r.client = mgr.GetClient()
	}
	if r.actionClientGetter == nil {
		actionConfigGetter, err := reconcilerutil.NewActionConfigGetter(mgr.GetConfig(), mgr.GetRESTMapper(), r.log)
		if err != nil {
			return err
		}
		r.actionClientGetter = reconcilerutil.NewActionClientGetter(actionConfigGetter)
	}
	if r.scheme == nil {
		r.scheme = mgr.GetScheme()
	}
	if r.eventRecorder == nil {
		r.eventRecorder = mgr.GetEventRecorderFor(controllerName)
	}

	mgr.GetScheme().AddKnownTypeWithName(*r.gvk, &unstructured.Unstructured{})
	metav1.AddToGroupVersion(mgr.GetScheme(), r.gvk.GroupVersion())

	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r, MaxConcurrentReconciles: 4})
	if err != nil {
		return err
	}

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

	r.setupDependentWatches(mgr, c)

	r.log.Info("Watching resource",
		"group", r.gvk.Group,
		"version", r.gvk.Version,
		"kind", r.gvk.Kind,
	)

	return nil
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

func (r *helm) getReleaseState(client reconcilerutil.ActionInterface, obj *unstructured.Unstructured, vals map[string]interface{}) (*release.Release, helmReleaseState, error) {
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

const uninstallFinalizer = "uninstall-helm-release"

type plainError struct {
	e error
}

func (pe plainError) Error() string {
	return pe.e.Error()
}

// setupDependentWatches sets up a function that the helm
// uses to add watches for resources in deployed Helm charts.
func (r *helm) setupDependentWatches(mgr manager.Manager, c controller.Controller) {
	owner := &unstructured.Unstructured{}
	owner.SetGroupVersionKind(*r.gvk)

	// using predefined functions for filtering events
	dependentPredicate := predicate.DependentPredicateFuncs()

	var m sync.RWMutex
	watches := map[schema.GroupVersionKind]struct{}{}
	addWatchesFunc := func(rel *release.Release, log logr.Logger) error {
		dec := yaml.NewDecoder(bytes.NewBufferString(rel.Manifest))
		for {
			var obj unstructured.Unstructured
			err := dec.Decode(&obj.Object)
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}

			gvk := obj.GroupVersionKind()
			m.RLock()
			_, ok := watches[gvk]
			m.RUnlock()
			if ok {
				continue
			}

			restMapper := mgr.GetRESTMapper()
			depMapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
			if err != nil {
				return err
			}
			ownerMapping, err := restMapper.RESTMapping(owner.GroupVersionKind().GroupKind(), owner.GroupVersionKind().Version)
			if err != nil {
				return err
			}

			depClusterScoped := depMapping.Scope.Name() == meta.RESTScopeNameRoot
			ownerClusterScoped := ownerMapping.Scope.Name() == meta.RESTScopeNameRoot

			if !ownerClusterScoped && depClusterScoped {
				m.Lock()
				watches[gvk] = struct{}{}
				m.Unlock()
				log.Info("Cannot watch cluster-scoped dependent resource for namespace-scoped owner. Changes to this dependent resource type will not be reconciled",
					"dependentAPIVersion", gvk.GroupVersion(), "dependentKind", gvk.Kind)
				continue
			}

			err = c.Watch(&source.Kind{Type: &obj}, &handler.EnqueueRequestForOwner{OwnerType: owner}, dependentPredicate)
			if err != nil {
				return err
			}

			m.Lock()
			watches[gvk] = struct{}{}
			m.Unlock()
			log.V(1).Info("Watching dependent resource", "dependentAPIVersion", gvk.GroupVersion(), "dependentKind", gvk.Kind)
		}
	}
	r.addWatchesFor = addWatchesFunc
}

func (r *helm) waitForDeletion(o runtime.Object) error {
	key, err := client.ObjectKeyFromObject(o)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	return wait.PollImmediateUntil(time.Millisecond*10, func() (bool, error) {
		err := r.client.Get(ctx, key, o)
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		return false, nil
	}, ctx.Done())
}

const irreconcilableConditionType = "Irreconcilable"

func irreconcilableCondition(err error) status.Condition {
	return status.Condition{
		Type:    irreconcilableConditionType,
		Status:  corev1.ConditionTrue,
		Reason:  "ReconcileError",
		Message: err.Error(),
	}
}
