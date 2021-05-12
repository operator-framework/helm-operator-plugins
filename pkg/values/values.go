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

package values

import (
	"helm.sh/helm/v3/pkg/chartutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Mapper interface {
	Map(*unstructured.Unstructured, chartutil.Values) chartutil.Values
}

// MapperFunc takes the original values and the watched custom resource to modify its contents, i.e. to map a custom
// resource or set new values based on cluster state.
type MapperFunc func(*unstructured.Unstructured, chartutil.Values) chartutil.Values

func (m MapperFunc) Map(cr *unstructured.Unstructured, v chartutil.Values) chartutil.Values {
	return m(cr, v)
}
