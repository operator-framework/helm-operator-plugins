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

package values_test

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/chartutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	. "github.com/operator-framework/helm-operator-plugins/pkg/reconciler/internal/values"
)

var _ = Describe("ApplyOverrides", func() {
	var u *unstructured.Unstructured

	When("Unstructured object is invalid", func() {
		It("should error with nil unstructured", func() {
			u = nil
			Expect(ApplyOverrides(nil, u)).NotTo(BeNil())
		})

		It("should error with nil object", func() {
			u = &unstructured.Unstructured{}
			Expect(ApplyOverrides(nil, u)).NotTo(BeNil())
		})

		It("should error with missing spec", func() {
			u = &unstructured.Unstructured{Object: map[string]interface{}{}}
			Expect(ApplyOverrides(nil, u)).NotTo(BeNil())
		})

		It("should error with non-map spec", func() {
			u = &unstructured.Unstructured{Object: map[string]interface{}{"spec": 0}}
			Expect(ApplyOverrides(nil, u)).NotTo(BeNil())
		})
	})

	When("Unstructured object is valid", func() {

		BeforeEach(func() {
			u = &unstructured.Unstructured{Object: map[string]interface{}{"spec": map[string]interface{}{}}}
		})

		It("should succeed with empty values", func() {
			Expect(ApplyOverrides(map[string]string{"foo": "bar"}, u)).To(Succeed())
			Expect(u.Object).To(Equal(map[string]interface{}{"spec": map[string]interface{}{"foo": "bar"}}))
		})

		It("should succeed with non-empty values", func() {
			u.Object["spec"].(map[string]interface{})["foo"] = "bar"
			Expect(ApplyOverrides(map[string]string{"foo": "baz"}, u)).To(Succeed())
			Expect(u.Object).To(Equal(map[string]interface{}{"spec": map[string]interface{}{"foo": "baz"}}))
		})

		It("should fail with invalid overrides", func() {
			Expect(ApplyOverrides(map[string]string{"foo[": "test"}, u)).ToNot(BeNil())
		})
	})
})

var _ = Describe("DefaultMapper", func() {
	It("returns values untouched", func() {
		in := chartutil.Values{"foo": map[string]interface{}{"bar": "baz"}}
		out := DefaultMapper.Map(in)
		Expect(out).To(Equal(in))
	})
})

var _ = Describe("DefaultTranslator", func() {
	var m map[string]interface{}

	It("returns empty spec untouched", func() {
		m = map[string]interface{}{}
	})

	It("returns filled spec untouched", func() {
		m = map[string]interface{}{"something": 0}
	})

	AfterEach(func() {
		u := &unstructured.Unstructured{Object: map[string]interface{}{"spec": m}}
		Expect(DefaultTranslator.Translate(context.Background(), u)).To(Equal(chartutil.Values(m)))
	})
})
