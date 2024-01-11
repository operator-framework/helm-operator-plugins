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
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ActionConfigGetter interface {
	ActionConfigFor(obj client.Object) (*action.Configuration, error)
}

func NewActionConfigGetter(cfg *rest.Config, rm meta.RESTMapper, log logr.Logger, opts ...ActionConfigGetterOption) (ActionConfigGetter, error) {
	rcg := newRESTClientGetter(cfg, rm, "")
	// Setup the debug log function that Helm will use
	debugLog := func(format string, v ...interface{}) {
		if log.GetSink() != nil {
			log.V(1).Info(fmt.Sprintf(format, v...))
		}
	}

	kc := kube.New(rcg)
	kc.Log = debugLog

	kcs, err := kc.Factory.KubernetesClientSet()
	if err != nil {
		return nil, fmt.Errorf("creating kubernetes client set: %w", err)
	}

	acg := &actionConfigGetter{
		kubeClient:       kc,
		kubeClientSet:    kcs,
		debugLog:         debugLog,
		restClientGetter: rcg.restClientGetter,
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

func getObjectNamespace(obj client.Object) (string, error) {
	return obj.GetNamespace(), nil
}

type actionConfigGetter struct {
	kubeClient       *kube.Client
	kubeClientSet    kubernetes.Interface
	debugLog         func(string, ...interface{})
	restClientGetter *restClientGetter

	objectToClientNamespace         ObjectToStringMapper
	objectToStorageNamespace        ObjectToStringMapper
	disableStorageOwnerRefInjection bool
}

func (acg *actionConfigGetter) ActionConfigFor(obj client.Object) (*action.Configuration, error) {
	storageNs, err := acg.objectToStorageNamespace(obj)
	if err != nil {
		return nil, fmt.Errorf("get storage namespace from object: %v", err)
	}

	secretClient := acg.kubeClientSet.CoreV1().Secrets(storageNs)
	if !acg.disableStorageOwnerRefInjection {
		ownerRef := metav1.NewControllerRef(obj, obj.GetObjectKind().GroupVersionKind())
		secretClient = &ownerRefSecretClient{
			SecretInterface: secretClient,
			refs:            []metav1.OwnerReference{*ownerRef},
		}
	}
	d := driver.NewSecrets(secretClient)

	// Also, use the debug log for the storage driver
	d.Log = acg.debugLog

	// Initialize the storage backend
	s := storage.Init(d)

	kubeClient := *acg.kubeClient
	kubeClient.Namespace, err = acg.objectToClientNamespace(obj)
	if err != nil {
		return nil, fmt.Errorf("get client namespace from object: %v", err)
	}

	return &action.Configuration{
		RESTClientGetter: acg.restClientGetter.ForNamespace(kubeClient.Namespace),
		Releases:         s,
		KubeClient:       &kubeClient,
		Log:              acg.debugLog,
	}, nil
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
