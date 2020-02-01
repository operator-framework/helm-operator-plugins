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
	"strings"
	"sync"

	"github.com/go-logr/logr"
	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/operator-framework/helm-operator/internal/controllerutil"
	"github.com/operator-framework/helm-operator/internal/predicate"
	"github.com/operator-framework/helm-operator/pkg/reconcilerutil"
)

// helm reconciles a Helm object
type helm struct {
	client        client.Client
	scheme        *runtime.Scheme
	aig           reconcilerutil.ActionClientGetter
	eventRecorder record.EventRecorder

	log           logr.Logger
	gvk           *schema.GroupVersionKind
	chrt          *chart.Chart
	addWatchesFor func(*release.Release) error
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

func WithActionInterfaceGetter(aig reconcilerutil.ActionClientGetter) HelmOption {
	return func(r *helm) error {
		r.aig = aig
		return nil
	}
}

func WithLog(log logr.Logger) HelmOption {
	return func(r *helm) error {
		r.log = log
		return nil
	}
}

func WithGVK(gvk schema.GroupVersionKind) HelmOption {
	return func(r *helm) error {
		r.gvk = &gvk
		return nil
	}
}

func WithChartPath(chartPath string) HelmOption {
	return func(r *helm) error {
		chartLoader, err := loader.Loader(chartPath)
		if err != nil {
			return err
		}

		chrt, err := chartLoader.Load()
		if err != nil {
			return err
		}
		r.chrt = chrt
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
	log := r.log.WithValues(strings.ToLower(r.gvk.Kind), req.NamespacedName)

	obj := unstructured.Unstructured{}
	obj.SetGroupVersionKind(*r.gvk)
	err := r.client.Get(ctx, req.NamespacedName, &obj)

	if apierrors.IsNotFound(err) {
		return ctrl.Result{}, nil
	}

	if err != nil {
		return ctrl.Result{}, plainError{err}
	}

	client, err := r.aig.ActionClientFor(&obj)
	if err != nil {
		return ctrl.Result{}, plainError{err}
	}

	deployedRelease, state, err := r.getReleaseStatus(client, obj)
	if err != nil {
		return ctrl.Result{}, plainError{err}
	}

	vals := obj.Object["spec"].(map[string]interface{})

	if state == statusNoAction {
		return ctrl.Result{}, nil
	}

	if state != statusNeedsUninstall {
		if err := r.ensureUninstallFinalizer(ctx, &obj); err != nil {
			return ctrl.Result{}, plainError{err}
		}
	}

	switch state {
	case statusNeedsInstall:
		rel, err := client.Install(obj.GetName(), obj.GetNamespace(), r.chrt, vals)
		if err != nil {
			return ctrl.Result{}, plainError{err}
		}
		if err := r.addWatchesFor(rel); err != nil {
			log.Error(err, "failed to watch release resources", "name", rel.Name, "version", rel.Version)
		}
		log.Info("Release installed", "name", rel.Name, "version", rel.Version)
	case statusNeedsUpgrade:
		rel, err := client.Upgrade(obj.GetName(), obj.GetNamespace(), r.chrt, vals)
		if err != nil {
			return ctrl.Result{}, plainError{err}
		}
		if err := r.addWatchesFor(rel); err != nil {
			log.Error(err, "failed to watch release resources", "name", rel.Name, "version", rel.Version)
		}
		log.Info("Release upgraded", "name", rel.Name, "version", rel.Version)
	case statusNeedsUninstall:
		resp, err := client.Uninstall(obj.GetName())
		if err != nil {
			if !errors.Is(err, driver.ErrReleaseNotFound) {
				return ctrl.Result{}, plainError{err}
			}
		} else {
			log.Info("Release uninstalled", "name", resp.Release.Name, "version", resp.Release.Version)
		}
		if err := r.removeUninstallFinalizer(ctx, &obj); err != nil {
			return ctrl.Result{}, plainError{err}
		}
	case statusUnchanged:
		if err := client.Reconcile(deployedRelease); err != nil {
			return ctrl.Result{}, plainError{err}
		}
		if err := r.addWatchesFor(deployedRelease); err != nil {
			log.Error(err, "failed to watch release resources", "name", deployedRelease.Name, "version", deployedRelease.Version)
		}
		log.Info("Release reconciled", "name", deployedRelease.Name, "version", deployedRelease.Version)
	default:
		return ctrl.Result{}, fmt.Errorf("unexpected release state: %s", state)
	}

	return ctrl.Result{}, nil
}

func (r *helm) SetupWithManager(mgr ctrl.Manager) error {
	if r.client == nil {
		r.client = mgr.GetClient()
	}
	if r.aig == nil {
		acg := reconcilerutil.NewActionConfigGetter(mgr.GetConfig(), mgr.GetRESTMapper(), r.log)
		r.aig = reconcilerutil.NewActionClientGetter(acg)
	}
	if r.scheme == nil {
		r.scheme = mgr.GetScheme()
	}

	mgr.GetScheme().AddKnownTypeWithName(*r.gvk, &unstructured.Unstructured{})
	metav1.AddToGroupVersion(mgr.GetScheme(), r.gvk.GroupVersion())

	controllerName := fmt.Sprintf("%v-controller", strings.ToLower(r.gvk.Kind))
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

	if err := c.Watch(
		&source.Kind{Type: &corev1.Secret{}},
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

type helmReleaseStatus string

const (
	statusNeedsInstall   helmReleaseStatus = "needs install"
	statusNeedsUpgrade   helmReleaseStatus = "needs upgrade"
	statusNeedsUninstall helmReleaseStatus = "needs uninstall"
	statusUnchanged      helmReleaseStatus = "unchanged"
	statusNoAction       helmReleaseStatus = "no action"
	statusError          helmReleaseStatus = "error"
)

func (r *helm) getReleaseStatus(client reconcilerutil.ActionInterface, obj unstructured.Unstructured) (*release.Release, helmReleaseStatus, error) {
	deployedRelease, err := client.Status(obj.GetName())
	if err != nil && !errors.Is(err, driver.ErrReleaseNotFound) {
		return nil, statusError, err
	}

	if obj.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(&obj, r.getUninstallFinalizer()) {
			return deployedRelease, statusNeedsUninstall, nil
		}
		return deployedRelease, statusNoAction, nil
	}
	if errors.Is(err, driver.ErrReleaseNotFound) {
		return nil, statusNeedsInstall, nil
	}

	vals := obj.Object["spec"].(map[string]interface{})
	specRelease, err := client.Upgrade(obj.GetName(), obj.GetNamespace(), r.chrt, vals, func(u *action.Upgrade) error {
		u.DryRun = true
		return nil
	})
	if err != nil {
		return deployedRelease, statusError, err
	}
	if specRelease.Manifest != deployedRelease.Manifest {
		return deployedRelease, statusNeedsUpgrade, nil
	}
	return deployedRelease, statusUnchanged, nil
}

func (r *helm) ensureUninstallFinalizer(ctx context.Context, obj *unstructured.Unstructured) error {
	controllerutil.AddFinalizer(obj, r.getUninstallFinalizer())
	return r.client.Update(ctx, obj)
}

func (r *helm) removeUninstallFinalizer(ctx context.Context, obj *unstructured.Unstructured) error {
	controllerutil.RemoveFinalizer(obj, r.getUninstallFinalizer())
	return r.client.Update(ctx, obj)
}

func (r *helm) getUninstallFinalizer() string {
	return fmt.Sprintf("%s/uninstall", r.gvk.Group)
}

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
	addWatchesFunc := func(rel *release.Release) error {
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
				r.log.Info("Cannot watch cluster-scoped dependent resource for namespace-scoped owner. Changes to this dependent resource type will not be reconciled",
					"ownerApiVersion", r.gvk.GroupVersion(), "ownerKind", r.gvk.Kind, "apiVersion", gvk.GroupVersion(), "kind", gvk.Kind)
				continue
			}

			err = c.Watch(&source.Kind{Type: &obj}, &handler.EnqueueRequestForOwner{OwnerType: owner}, dependentPredicate)
			if err != nil {
				return err
			}

			m.Lock()
			watches[gvk] = struct{}{}
			m.Unlock()
			r.log.V(1).Info("Watching dependent resource", "ownerApiVersion", r.gvk.GroupVersion(), "ownerKind", r.gvk.Kind, "apiVersion", gvk.GroupVersion(), "kind", gvk.Kind)
		}
	}
	r.addWatchesFor = addWatchesFunc
}
