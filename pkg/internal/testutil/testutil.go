package testutil

import (
	"fmt"
	"strings"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func BuildTestCRD(gvk schema.GroupVersionKind) apiextv1.CustomResourceDefinition {
	trueVal := true
	singular := strings.ToLower(gvk.Kind)
	plural := fmt.Sprintf("%ss", singular)
	return apiextv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s.%s", plural, gvk.Group),
		},
		Spec: apiextv1.CustomResourceDefinitionSpec{
			Group: gvk.Group,
			Names: apiextv1.CustomResourceDefinitionNames{
				Kind:     gvk.Kind,
				ListKind: fmt.Sprintf("%sList", gvk.Kind),
				Singular: singular,
				Plural:   plural,
			},
			Scope: apiextv1.NamespaceScoped,
			Versions: []apiextv1.CustomResourceDefinitionVersion{
				{
					Name: "v1",
					Schema: &apiextv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextv1.JSONSchemaProps{
							Type:                   "object",
							XPreserveUnknownFields: &trueVal,
						},
					},
					Subresources: &apiextv1.CustomResourceSubresources{
						Status: &apiextv1.CustomResourceSubresourceStatus{},
					},
					Served:  true,
					Storage: true,
				},
			},
		},
	}
}

func MustLoadChart(path string) chart.Chart {
	chrt, err := loader.Load(path)
	if err != nil {
		panic(err)
	}
	return *chrt
}

func BuildTestCR(gvk schema.GroupVersionKind) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"spec": map[string]interface{}{"replicas": 2},
	}}
	obj.SetName("test")
	obj.SetNamespace("default")
	obj.SetGroupVersionKind(gvk)
	obj.SetUID("test-uid")
	obj.SetAnnotations(map[string]string{
		"helm.sdk.operatorframework.io/install-description":   "test install description",
		"helm.sdk.operatorframework.io/upgrade-description":   "test upgrade description",
		"helm.sdk.operatorframework.io/uninstall-description": "test uninstall description",
	})
	return obj
}
