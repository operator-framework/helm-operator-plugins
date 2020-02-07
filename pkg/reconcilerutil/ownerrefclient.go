package reconcilerutil

import (
	"io"

	"helm.sh/helm/v3/pkg/kube"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/resource"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

var _ kube.Interface = &ownerRefInjectingClient{}

func newOwnerRefInjectingClient(base kube.Client, ownerRef metav1.OwnerReference) *ownerRefInjectingClient {
	return &ownerRefInjectingClient{
		refs:   []metav1.OwnerReference{ownerRef},
		Client: base,
	}
}

type ownerRefInjectingClient struct {
	refs []metav1.OwnerReference
	kube.Client
}

func (c *ownerRefInjectingClient) Build(reader io.Reader, validate bool) (kube.ResourceList, error) {
	resourceList, err := c.Client.Build(reader, validate)
	if err != nil {
		return resourceList, err
	}
	err = resourceList.Visit(func(r *resource.Info, err error) error {
		if err != nil {
			return err
		}
		objMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(r.Object)
		if err != nil {
			return err
		}
		u := &unstructured.Unstructured{Object: objMap}
		if r.ResourceMapping().Scope == meta.RESTScopeNamespace {
			u.SetOwnerReferences(c.refs)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return resourceList, nil
}

var _ v1.SecretInterface = &ownerRefSecretClient{}

type ownerRefSecretClient struct {
	v1.SecretInterface
	refs []metav1.OwnerReference
}

func (c *ownerRefSecretClient) Create(in *corev1.Secret) (*corev1.Secret, error) {
	in.OwnerReferences = append(in.OwnerReferences, c.refs...)
	return c.SecretInterface.Create(in)
}

func (c *ownerRefSecretClient) Update(in *corev1.Secret) (*corev1.Secret, error) {
	in.OwnerReferences = append(in.OwnerReferences, c.refs...)
	return c.SecretInterface.Update(in)
}
