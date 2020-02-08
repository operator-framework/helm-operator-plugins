package hooks

import (
	"bytes"
	"io"
	"sync"

	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/apimachinery/pkg/api/meta"

	"github.com/go-logr/logr"
	"github.com/operator-framework/helm-operator/internal/predicate"
	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type ReleaseHook interface {
	Exec(*release.Release) error
}

type ReleaseHookFunc func(*release.Release) error

func (r ReleaseHookFunc) Exec(rel *release.Release) error {
	return r(rel)
}

func NewDependentResourceWatcher(c controller.Controller, rm meta.RESTMapper, owner runtime.Object, log logr.Logger) ReleaseHook {
	return &dependentResourceWatcher{
		Controller: c,
		Owner:      owner,
		RESTMapper: rm,
		Log:        log,
		m:          sync.Mutex{},
		watches:    make(map[schema.GroupVersionKind]struct{}),
	}
}

func (d *dependentResourceWatcher) Exec(rel *release.Release) error {
	// using predefined functions for filtering events
	dependentPredicate := predicate.DependentPredicateFuncs()

	dec := yaml.NewDecoder(bytes.NewBufferString(rel.Manifest))
	d.m.Lock()
	defer d.m.Unlock()
	for {
		var obj unstructured.Unstructured
		err := dec.Decode(&obj.Object)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		depGVK := obj.GroupVersionKind()
		if _, ok := d.watches[depGVK]; ok {
			continue
		}

		if ok, err := isValidRelationship(d.RESTMapper, d.Owner.GetObjectKind().GroupVersionKind(), depGVK); err != nil {
			return err
		} else if !ok {
			d.watches[depGVK] = struct{}{}
			d.Log.Info("Cannot watch cluster-scoped dependent resource for namespace-scoped owner. Changes to this dependent resource type will not be reconciled",
				"dependentAPIVersion", depGVK.GroupVersion(), "dependentKind", depGVK.Kind)
			continue
		}

		err = d.Controller.Watch(&source.Kind{Type: &obj}, &handler.EnqueueRequestForOwner{OwnerType: d.Owner}, dependentPredicate)
		if err != nil {
			return err
		}

		d.watches[depGVK] = struct{}{}
		d.Log.V(1).Info("Watching dependent resource", "dependentAPIVersion", depGVK.GroupVersion(), "dependentKind", depGVK.Kind)
	}
}

type dependentResourceWatcher struct {
	Controller controller.Controller
	Owner      runtime.Object
	RESTMapper meta.RESTMapper
	Log        logr.Logger

	m       sync.Mutex
	watches map[schema.GroupVersionKind]struct{}
}

func isValidRelationship(restMapper meta.RESTMapper, owner, dependent schema.GroupVersionKind) (bool, error) {
	ownerMapping, err := restMapper.RESTMapping(owner.GroupKind(), owner.Version)
	if err != nil {
		return false, err
	}

	depMapping, err := restMapper.RESTMapping(dependent.GroupKind(), dependent.Version)
	if err != nil {
		return false, err
	}

	ownerClusterScoped := ownerMapping.Scope.Name() == meta.RESTScopeNameRoot
	depClusterScoped := depMapping.Scope.Name() == meta.RESTScopeNameRoot

	if !ownerClusterScoped && depClusterScoped {
		return false, nil
	}
	return true, nil
}
