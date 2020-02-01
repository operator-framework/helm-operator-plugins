package reconcilerutil

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	cached "k8s.io/client-go/discovery/cached"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var _ genericclioptions.RESTClientGetter = &restClientGetter{}

type restClientGetter struct {
	restConfig      *rest.Config
	discoveryClient discovery.CachedDiscoveryInterface
	restMapper      meta.RESTMapper
	namespaceConfig clientcmd.ClientConfig
}

func newRESTClientGetter(ns string, cfg *rest.Config, rm meta.RESTMapper) (genericclioptions.RESTClientGetter, error) {
	dc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, err
	}
	cdc := cached.NewMemCacheClient(dc)

	return &restClientGetter{
		restConfig:      cfg,
		discoveryClient: cdc,
		restMapper:      rm,
		namespaceConfig: &namespaceClientConfig{ns},
	}, nil
}

func (c *restClientGetter) ToRESTConfig() (*rest.Config, error) {
	return c.restConfig, nil
}

func (c *restClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	return c.discoveryClient, nil
}

func (c *restClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	return c.restMapper, nil
}

func (c *restClientGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	return c.namespaceConfig
}

var _ clientcmd.ClientConfig = &namespaceClientConfig{}

type namespaceClientConfig struct {
	namespace string
}

func (c namespaceClientConfig) RawConfig() (clientcmdapi.Config, error) {
	panic("not implemented")
}

func (c namespaceClientConfig) ClientConfig() (*rest.Config, error) {
	panic("not implemented")
}

func (c namespaceClientConfig) Namespace() (string, bool, error) {
	return c.namespace, false, nil
}

func (c namespaceClientConfig) ConfigAccess() clientcmd.ConfigAccess {
	panic("not implemented")
}
