package extension

import (
	"context"

	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
)

// ReconcilerExtension defines an extension for the reconciler.
// It consists of several sub-interfaces.
type ReconcilerExtension interface {
	Name() string
	BeginReconciliationExtension
	EndReconciliationExtension
}

// BeginReconciliationExtension defines the extension point to execute at the beginning of a reconciliation flow.
type BeginReconciliationExtension interface {
	BeginReconcile(ctx context.Context, obj *unstructured.Unstructured) error
}

// EndReconciliationExtension defiens the extension point to execute at the end of a reconciliation flow.
type EndReconciliationExtension interface {
	EndReconcile(ctx context.Context, obj *unstructured.Unstructured) error
}

// NoOpReconcilerExtension implements all extension methods as no-ops and can be used for convenience
// when only a subset of the available methods needs to be implemented.
type NoOpReconcilerExtension struct{}

func (e NoOpReconcilerExtension) BeginReconcile(ctx context.Context, obj *unstructured.Unstructured) error {
	return nil
}

func (e NoOpReconcilerExtension) EndReconcile(ctx context.Context, obj *unstructured.Unstructured) error {
	return nil
}

type helmReleaseType int

var helmReleaseKey helmReleaseType

type helmValuesType int

var helmValuesKey helmValuesType

type kubernetesConfigType int

var kubernetesConfigKey kubernetesConfigType

type Context struct {
	KubernetesConfig *rest.Config
	HelmRelease      *release.Release
	HelmValues       chartutil.Values
}

func newContextWithValues(ctx context.Context, vals ...interface{}) context.Context {
	if len(vals)%2 == 1 {
		// Uneven number of vals, which is supposed to consist of key-value pairs.
		// Add trailing nil value to fix it.
		vals = append(vals, nil)
	}
	for i := 0; i < len(vals); i += 2 {
		k := vals[i]
		v := vals[i+1]
		ctx = context.WithValue(ctx, k, v)
	}
	return ctx
}

func NewContext(ctx context.Context, reconciliationContext *Context) context.Context {
	if reconciliationContext == nil {
		return ctx
	}
	return newContextWithValues(ctx,
		helmReleaseKey, reconciliationContext.HelmRelease,
		helmValuesKey, reconciliationContext.HelmValues,
		kubernetesConfigKey, reconciliationContext.KubernetesConfig,
	)
}

func HelmReleaseFromContext(ctx context.Context) release.Release {
	v, ok := ctx.Value(helmReleaseKey).(*release.Release)
	if !ok {
		return release.Release{}
	}
	if v == nil {
		return release.Release{}
	}
	return *v
}

func KubernetesConfigFromContext(ctx context.Context) *rest.Config {
	v, ok := ctx.Value(kubernetesConfigKey).(*rest.Config)
	if !ok {
		return nil
	}
	return v
}

func HelmValuesFromContext(ctx context.Context) chartutil.Values {
	v, ok := ctx.Value(helmValuesKey).(chartutil.Values)
	if !ok {
		return nil
	}
	return v
}
