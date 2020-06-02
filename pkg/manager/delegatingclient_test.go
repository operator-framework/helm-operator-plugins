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

package manager_test

import (
	"context"
	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/joelanford/helm-operator/pkg/manager"
)

var _ = Describe("NewDelegatingClientFunc", func() {
	var podNs *v1.Namespace
	var pod *v1.Pod

	BeforeEach(func() {
		podNs = &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pod-ns",
			},
		}
		pod = &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod",
				Namespace: "pod-ns",
			},
			Spec: v1.PodSpec{Containers: []v1.Container{
				{Name: "test", Image: "test"},
			}},
		}
	})

	It("should return a function that returns a working delegating client", func() {
		clientFunc := NewDelegatingClientFunc()
		Expect(clientFunc).NotTo(BeNil())

		c, err := cache.New(cfg, cache.Options{})
		Expect(err).To(BeNil())

		cl, err := clientFunc(c, cfg, client.Options{})
		Expect(err).To(BeNil())

		Expect(cl.Create(context.TODO(), podNs)).To(Succeed())
		Expect(cl.Create(context.TODO(), pod)).To(Succeed())
		Expect(cl.Get(context.TODO(), client.ObjectKey{Namespace: "pod-ns", Name: "pod"}, pod)).To(BeAssignableToTypeOf(&cache.ErrCacheNotStarted{}))

		done := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			Expect(c.Start(done)).To(Succeed())
			wg.Done()
		}()
		Expect(c.WaitForCacheSync(done)).To(BeTrue())

		Expect(cl.Get(context.TODO(), client.ObjectKey{Namespace: "pod-ns", Name: "pod"}, pod)).To(Succeed())
		close(done)
		wg.Wait()
	})

	It("should fail with an invalid config", func() {
		clientFunc := NewDelegatingClientFunc()
		Expect(clientFunc).NotTo(BeNil())

		c, err := cache.New(cfg, cache.Options{})
		Expect(err).To(BeNil())

		badConfig := rest.Config{
			Host: "/path/to/foobar",
		}
		_, err = clientFunc(c, &badConfig, client.Options{})
		Expect(err).NotTo(BeNil())
	})
})
