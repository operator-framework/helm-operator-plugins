// Copyright 2020 The Operator-SDK Authors
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

package hook

import (
	"bytes"
	"io"
	"sync"

	"github.com/go-logr/logr"
	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/joelanford/helm-operator/pkg/hook"
	"github.com/joelanford/helm-operator/pkg/reconciler/internal/predicate"
)

func NewDependentResourceWatcher(c controller.Controller, rm meta.RESTMapper, owner runtime.Object) hook.PostHook {
	return &dependentResourceWatcher{
		controller: c,
		owner:      owner,
		restMapper: rm,
		m:          sync.Mutex{},
		watches:    make(map[schema.GroupVersionKind]struct{}),
	}
}

type dependentResourceWatcher struct {
	controller controller.Controller
	owner      runtime.Object
	restMapper meta.RESTMapper

	m       sync.Mutex
	watches map[schema.GroupVersionKind]struct{}
}

func (d *dependentResourceWatcher) Exec(_ *unstructured.Unstructured, rel *release.Release, log logr.Logger) error {
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
		if _, ok := d.watches[depGVK]; ok || depGVK.Empty() {
			continue
		}

		if ok, err := isValidRelationship(d.restMapper, d.owner.GetObjectKind().GroupVersionKind(), depGVK); err != nil {
			return err
		} else if !ok {
			d.watches[depGVK] = struct{}{}
			log.Info("Cannot watch cluster-scoped dependent resource for namespace-scoped owner. Changes to this dependent resource type will not be reconciled",
				"dependentAPIVersion", depGVK.GroupVersion(), "dependentKind", depGVK.Kind)
			continue
		}

		err = d.controller.Watch(&source.Kind{Type: &obj}, &handler.EnqueueRequestForOwner{OwnerType: d.owner}, dependentPredicate)
		if err != nil {
			return err
		}

		d.watches[depGVK] = struct{}{}
		log.V(1).Info("Watching dependent resource", "dependentAPIVersion", depGVK.GroupVersion(), "dependentKind", depGVK.Kind)
	}
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
