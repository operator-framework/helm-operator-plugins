package values

import (
	"fmt"

	"helm.sh/helm/v3/pkg/strvals"
)

type Values struct {
	m map[string]interface{}
}

func New(m map[string]interface{}) Values {
	return Values{m: m}
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
