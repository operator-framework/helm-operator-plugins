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

package hook

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/operator-framework/helm-operator-plugins/pkg/extension"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type PreHookFunc func(context.Context, *unstructured.Unstructured, logr.Logger) error

func WrapPreHookFunc(f PreHookFunc) PreHookFunc {
	wrappedF := func(ctx context.Context, obj *unstructured.Unstructured, log logr.Logger) error {
		err := f(ctx, obj, log)
		if err != nil {
			log.Error(err, "pre-release hook failed")
		}
		return nil
	}

	return PreHookFunc(wrappedF)
}

func WrapPostHookFunc(f PostHookFunc) PostHookFunc {
	wrappedF := func(ctx context.Context, obj *unstructured.Unstructured, rel release.Release, vals chartutil.Values, log logr.Logger) error {
		err := f(ctx, obj, rel, vals, log)
		if err != nil {
			log.Error(err, "post-release hook failed", "name", rel.Name, "version", rel.Version)
		}
		return nil
	}

	return PostHookFunc(wrappedF)
}

func (h PreHookFunc) Name() string {
	return "pre-hook"
}

func (h PreHookFunc) BeginReconcile(ctx context.Context, reconciliationContext *extension.Context, obj *unstructured.Unstructured) error {
	log := logr.FromContextOrDiscard(ctx)
	return h(ctx, obj, log)
}

func (h PreHookFunc) EndReconcile(ctx context.Context, reconciliationContext *extension.Context, obj *unstructured.Unstructured) error {
	return nil
}

var _ extension.ReconcilerExtension = (PreHookFunc)(nil)

type PostHookFunc func(context.Context, *unstructured.Unstructured, release.Release, chartutil.Values, logr.Logger) error

func (h PostHookFunc) Name() string {
	return "post-hook"
}

func (f PostHookFunc) BeginReconcile(ctx context.Context, reconciliationContext *extension.Context, obj *unstructured.Unstructured) error {
	return nil
}

func (f PostHookFunc) EndReconcile(ctx context.Context, reconciliationContext *extension.Context, obj *unstructured.Unstructured) error {
	log := logr.FromContextOrDiscard(ctx)
	return f(ctx, obj, reconciliationContext.GetHelmRelease(), reconciliationContext.GetHelmValues(), log)
}

var _ extension.ReconcilerExtension = (*PostHookFunc)(nil)
