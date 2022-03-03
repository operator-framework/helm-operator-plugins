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
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/operator-framework/helm-operator-plugins/pkg/internal/testutil"
)

var _ = Describe("ActionConfig", func() {
	var _ = Describe("NewActionConfigGetter", func() {
		It("should return a valid ActionConfigGetter", func() {
			Expect(NewActionConfigGetter(nil, nil, logr.Discard())).NotTo(BeNil())
		})
	})

	var _ = Describe("GetActionConfig", func() {
		var obj client.Object
		BeforeEach(func() {
			obj = testutil.BuildTestCR(gvk)
		})
		It("should return a valid action.Configuration", func() {
			rm, err := apiutil.NewDiscoveryRESTMapper(cfg)
			Expect(err).To(BeNil())

			acg := NewActionConfigGetter(cfg, rm, logr.Discard())
			ac, err := acg.ActionConfigFor(obj)
			Expect(err).To(BeNil())
			Expect(ac).NotTo(BeNil())
		})
	})
})
