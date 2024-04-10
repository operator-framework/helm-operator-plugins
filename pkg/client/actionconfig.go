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

package client

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/kube"
	"helm.sh/helm/v3/pkg/storage"
	"helm.sh/helm/v3/pkg/storage/driver"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ActionConfigGetter interface {
	ActionConfigFor(ctx context.Context, obj client.Object) (*action.Configuration, error)
}

func NewActionConfigGetter(baseRestConfig *rest.Config, rm meta.RESTMapper, opts ...ActionConfigGetterOption) (ActionConfigGetter, error) {
	dc, err := discovery.NewDiscoveryClientForConfig(baseRestConfig)
	if err != nil {
		return nil, fmt.Errorf("create discovery client: %v", err)
	}
	cdc := memory.NewMemCacheClient(dc)

	acg := &actionConfigGetter{
		baseRestConfig:  baseRestConfig,
		restMapper:      rm,
		discoveryClient: cdc,
	}
	for _, o := range opts {
		o(acg)
	}
	if acg.objectToClientNamespace == nil {
		acg.objectToClientNamespace = getObjectNamespace
	}
	if acg.objectToStorageNamespace == nil {
		acg.objectToStorageNamespace = getObjectNamespace
	}
	if acg.objectToRestConfig == nil {
		acg.objectToRestConfig = func(_ context.Context, _ client.Object, baseRestConfig *rest.Config) (*rest.Config, error) {
			return rest.CopyConfig(baseRestConfig), nil
		}
	}
	return acg, nil
}

var _ ActionConfigGetter = &actionConfigGetter{}

type ActionConfigGetterOption func(getter *actionConfigGetter)

type ObjectToStringMapper func(client.Object) (string, error)

func ClientNamespaceMapper(m ObjectToStringMapper) ActionConfigGetterOption { // nolint:revive
	return func(getter *actionConfigGetter) {
		getter.objectToClientNamespace = m
	}
}

func StorageNamespaceMapper(m ObjectToStringMapper) ActionConfigGetterOption {
	return func(getter *actionConfigGetter) {
		getter.objectToStorageNamespace = m
	}
}

func DisableStorageOwnerRefInjection(v bool) ActionConfigGetterOption {
	return func(getter *actionConfigGetter) {
		getter.disableStorageOwnerRefInjection = v
	}
}

func RestConfigMapper(f func(context.Context, client.Object, *rest.Config) (*rest.Config, error)) ActionConfigGetterOption {
	return func(getter *actionConfigGetter) {
		getter.objectToRestConfig = f
	}
}

func getObjectNamespace(obj client.Object) (string, error) {
	return obj.GetNamespace(), nil
}

type actionConfigGetter struct {
	baseRestConfig  *rest.Config
	restMapper      meta.RESTMapper
	discoveryClient discovery.CachedDiscoveryInterface

	objectToClientNamespace         ObjectToStringMapper
	objectToStorageNamespace        ObjectToStringMapper
	objectToRestConfig              func(context.Context, client.Object, *rest.Config) (*rest.Config, error)
	disableStorageOwnerRefInjection bool
}

func (acg *actionConfigGetter) ActionConfigFor(ctx context.Context, obj client.Object) (*action.Configuration, error) {
	storageNs, err := acg.objectToStorageNamespace(obj)
	if err != nil {
		return nil, fmt.Errorf("get storage namespace for object: %v", err)
	}

	restConfig, err := acg.objectToRestConfig(ctx, obj, acg.baseRestConfig)
	if err != nil {
		return nil, fmt.Errorf("get rest config for object: %v", err)
	}

	clientNamespace, err := acg.objectToClientNamespace(obj)
	if err != nil {
		return nil, fmt.Errorf("get client namespace for object: %v", err)
	}

	rcg := newRESTClientGetter(restConfig, acg.restMapper, acg.discoveryClient, clientNamespace)
	kc := kube.New(rcg)
	kc.Namespace = clientNamespace

	kcs, err := kc.Factory.KubernetesClientSet()
	if err != nil {
		return nil, fmt.Errorf("create kubernetes clientset: %v", err)
	}

	// Setup the debug log function that Helm will use
	debugLog := getDebugLogger(ctx)

	secretClient := kcs.CoreV1().Secrets(storageNs)
	if !acg.disableStorageOwnerRefInjection {
		ownerRef := metav1.NewControllerRef(obj, obj.GetObjectKind().GroupVersionKind())
		secretClient = &ownerRefSecretClient{
			SecretInterface: secretClient,
			refs:            []metav1.OwnerReference{*ownerRef},
		}
	}
	d := driver.NewSecrets(secretClient)
	d.Log = debugLog

	// Initialize the storage backend
	s := storage.Init(d)

	return &action.Configuration{
		RESTClientGetter: rcg,
		Releases:         s,
		KubeClient:       kc,
		Log:              debugLog,
	}, nil
}

func getDebugLogger(ctx context.Context) func(format string, v ...interface{}) {
	logger, err := logr.FromContext(ctx)
	if err != nil {
		return func(format string, v ...interface{}) {}
	}
	return func(format string, v ...interface{}) {
		logger.V(1).Info(fmt.Sprintf(format, v...))
	}
}

var _ v1.SecretInterface = &ownerRefSecretClient{}

type ownerRefSecretClient struct {
	v1.SecretInterface
	refs []metav1.OwnerReference
}

func (c *ownerRefSecretClient) Create(ctx context.Context, in *corev1.Secret, opts metav1.CreateOptions) (*corev1.Secret, error) {
	in.OwnerReferences = append(in.OwnerReferences, c.refs...)
	return c.SecretInterface.Create(ctx, in, opts)
}

func (c *ownerRefSecretClient) Update(ctx context.Context, in *corev1.Secret, opts metav1.UpdateOptions) (*corev1.Secret, error) {
	in.OwnerReferences = append(in.OwnerReferences, c.refs...)
	return c.SecretInterface.Update(ctx, in, opts)
}
