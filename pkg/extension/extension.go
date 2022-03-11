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
	BeginReconcile(ctx context.Context, reconciliationContext *Context, obj *unstructured.Unstructured) error
}

// EndReconciliationExtension defiens the extension point to execute at the end of a reconciliation flow.
type EndReconciliationExtension interface {
	EndReconcile(ctx context.Context, reconciliationContext *Context, obj *unstructured.Unstructured) error
}

// NoOpReconcilerExtension implements all extension methods as no-ops and can be used for convenience
// when only a subset of the available methods needs to be implemented.
type NoOpReconcilerExtension struct{}

func (e NoOpReconcilerExtension) BeginReconcile(ctx context.Context, reconciliationContext *Context, obj *unstructured.Unstructured) error {
	return nil
}

func (e NoOpReconcilerExtension) EndReconcile(ctx context.Context, reconciliationContext *Context, obj *unstructured.Unstructured) error {
	return nil
}

// Context can be used for providing extension methods will more context about the reconciliation flow.
// This contains data objects which can be useful for specific extensions but might not be required for all.
type Context struct {
	KubernetesConfig *rest.Config
	HelmRelease      *release.Release
	HelmValues       chartutil.Values
}

// GetHelmRelease is a nil-safe getter retrieving the Helm release information from a reconciliation context, if available.
// Returns an empty Helm release if the release information is not available at the time the extension point is called.
func (c *Context) GetHelmRelease() release.Release {
	if c == nil || c.HelmRelease == nil {
		return release.Release{}
	}
	return *c.HelmRelease
}

// GetHelmValues is a nil-safe getter retrieving the Helm values information from a reconciliation context, if available.
// Returns nil if the Helm value are not available at the time the extension point is called.
func (c *Context) GetHelmValues() chartutil.Values {
	if c == nil {
		return nil
	}
	return c.HelmValues
}

// GetKubernetesConfig is a nil-safe getter for retrieving the Kubernetes config from a reconciliation context, if available.
func (c *Context) GetKubernetesConfig() *rest.Config {
	if c == nil {
		return nil
	}
	return c.KubernetesConfig
}
