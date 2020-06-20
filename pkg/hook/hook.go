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

// Package hook contains pre and post hooks are just custom go functions that can be provided
// when the reconciler is created in main.go. Note that chart hooks are handled in the helm libraries by
// the action.Install, action.Upgrade, and action.Uninstall actions.
package hook

import (
	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// PreHook just before performing any actions (e.g. install, upgrade, uninstall, or reconciliation).
type PreHook interface {
	Exec(*unstructured.Unstructured, chartutil.Values, logr.Logger) error
}

// PreHookFunc used to set a reconciler PreHook
type PreHookFunc func(*unstructured.Unstructured, chartutil.Values, logr.Logger) error

// Exec used to executed a reconciler PreHook
func (f PreHookFunc) Exec(obj *unstructured.Unstructured, vals chartutil.Values, log logr.Logger) error {
	return f(obj, vals, log)
}

// PostHook just after performing any actions (e.g. install, upgrade, uninstall, or reconciliation).
type PostHook interface {
	Exec(*unstructured.Unstructured, release.Release, logr.Logger) error
}

// PostHookFunc used to set a reconciler PostHook
type PostHookFunc func(*unstructured.Unstructured, release.Release, logr.Logger) error

// Exec used to executed a reconciler PostHook
func (f PostHookFunc) Exec(obj *unstructured.Unstructured, rel release.Release, log logr.Logger) error {
	return f(obj, rel, log)
}
