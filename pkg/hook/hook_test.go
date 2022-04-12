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

package hook_test

import (
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	. "github.com/operator-framework/helm-operator-plugins/pkg/hook"
)

var _ = Describe("Hook", func() {
	var _ = Describe("PreHookFunc", func() {
		It("should implement the PreHook interface", func() {
			called := false
			var h PreHook = PreHookFunc(func(*unstructured.Unstructured, chartutil.Values, logr.Logger) error {
				called = true
				return nil
			})
			Expect(h.Exec(nil, nil, logr.Discard())).To(Succeed())
			Expect(called).To(BeTrue())
		})
	})
	var _ = Describe("PostHookFunc", func() {
		It("should implement the PostHook interface", func() {
			called := false
			var h PostHook = PostHookFunc(func(*unstructured.Unstructured, release.Release, logr.Logger) error {
				called = true
				return nil
			})
			Expect(h.Exec(nil, release.Release{}, logr.Discard())).To(Succeed())
			Expect(called).To(BeTrue())
		})
	})
})
