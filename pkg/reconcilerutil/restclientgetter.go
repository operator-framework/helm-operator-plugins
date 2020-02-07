package reconcilerutil

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var _ genericclioptions.RESTClientGetter = &restClientGetter{}

func newRESTClientGetter(cfg *rest.Config, cdc discovery.CachedDiscoveryInterface, rm meta.RESTMapper, ns string) genericclioptions.RESTClientGetter {
	return &restClientGetter{
		restConfig:            cfg,
		cachedDiscoveryClient: cdc,
		restMapper:            rm,
		namespaceConfig:       &namespaceClientConfig{ns},
	}
}

type restClientGetter struct {
	restConfig            *rest.Config
	cachedDiscoveryClient discovery.CachedDiscoveryInterface
	restMapper            meta.RESTMapper
	namespaceConfig       clientcmd.ClientConfig
}

func (c *restClientGetter) ToRESTConfig() (*rest.Config, error) {
	return c.restConfig, nil
}

func (c *restClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	return c.cachedDiscoveryClient, nil
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
	return clientcmdapi.Config{}, nil
}

func (c namespaceClientConfig) ClientConfig() (*rest.Config, error) {
	return nil, nil
}

func (c namespaceClientConfig) Namespace() (string, bool, error) {
	return c.namespace, false, nil
}

func (c namespaceClientConfig) ConfigAccess() clientcmd.ConfigAccess {
	return nil
}
