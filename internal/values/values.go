package values

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"helm.sh/helm/v3/pkg/strvals"
)

type Values struct {
	m map[string]interface{}
}

func FromUnstructured(obj *unstructured.Unstructured) (*Values, error) {
	spec, ok := obj.Object["spec"]
	if !ok {
		return nil, fmt.Errorf("spec not found")
	}
	specMap, ok := spec.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("spec must be a map")
	}
	return New(specMap), nil
}

func New(m map[string]interface{}) *Values {
	return &Values{m: m}
}

func (v *Values) Map() map[string]interface{} {
	if v == nil {
		return nil
	}
	return v.m
}

func (v *Values) ApplyOverrides(in map[string]string) error {
	for inK, inV := range in {
		val := fmt.Sprintf("%s=%s", inK, inV)
		if err := strvals.ParseInto(val, v.m); err != nil {
			return err
		}
	}
	return nil
}
