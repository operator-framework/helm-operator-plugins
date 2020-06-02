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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/chartutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	. "github.com/joelanford/helm-operator/pkg/reconciler/internal/values"
)

var _ = Describe("Values", func() {
	var _ = Describe("FromUnstructured", func() {
		It("should error with nil object", func() {
			u := &unstructured.Unstructured{}
			v, err := FromUnstructured(u)
			Expect(v).To(BeNil())
			Expect(err).NotTo(BeNil())
		})

		It("should error with missing spec", func() {
			u := &unstructured.Unstructured{Object: map[string]interface{}{}}
			v, err := FromUnstructured(u)
			Expect(v).To(BeNil())
			Expect(err).NotTo(BeNil())
		})

		It("should error with non-map spec", func() {
			u := &unstructured.Unstructured{Object: map[string]interface{}{"spec": 0}}
			v, err := FromUnstructured(u)
			Expect(v).To(BeNil())
			Expect(err).NotTo(BeNil())
		})

		It("should succeed with valid spec", func() {
			values := New(map[string]interface{}{"foo": "bar"})
			u := &unstructured.Unstructured{Object: map[string]interface{}{"spec": values.Map()}}
			Expect(FromUnstructured(u)).To(Equal(values))
		})
	})

	var _ = Describe("New", func() {
		It("should return new values", func() {
			m := map[string]interface{}{"foo": "bar"}
			v := New(m)
			Expect(v.Map()).To(Equal(m))
		})
	})

	var _ = Describe("Map", func() {
		It("should return nil with nil values", func() {
			var v *Values
			Expect(v.Map()).To(BeNil())
		})

		It("should return values as a map", func() {
			m := map[string]interface{}{"foo": "bar"}
			v := New(m)
			Expect(v.Map()).To(Equal(m))
		})
	})

	var _ = Describe("ApplyOverrides", func() {
		It("should succeed with empty values", func() {
			v := New(map[string]interface{}{})
			Expect(v.ApplyOverrides(map[string]string{"foo": "bar"})).To(Succeed())
			Expect(v.Map()).To(Equal(map[string]interface{}{"foo": "bar"}))
		})

		It("should succeed with empty values", func() {
			v := New(map[string]interface{}{"foo": "bar"})
			Expect(v.ApplyOverrides(map[string]string{"foo": "baz"})).To(Succeed())
			Expect(v.Map()).To(Equal(map[string]interface{}{"foo": "baz"}))
		})

		It("should fail with invalid overrides", func() {
			v := New(map[string]interface{}{"foo": "bar"})
			Expect(v.ApplyOverrides(map[string]string{"foo[": "test"})).ToNot(BeNil())
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
