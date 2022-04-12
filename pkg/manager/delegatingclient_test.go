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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"

	. "github.com/operator-framework/helm-operator-plugins/pkg/manager"
)

var _ = Describe("NewCachingClientBuilder", func() {
	var ns *unstructured.Unstructured
	var pod *v1.Pod
	var cfgMap *v1.ConfigMap
	var clientFunc cluster.NewClientFunc

	BeforeEach(func() {
		ns = &unstructured.Unstructured{}
		ns.SetGroupVersionKind(schema.GroupVersionKind{
			Version: "v1",
			Kind:    "Namespace",
		})
		ns.SetName("ns-" + rand.String(4))
		pod = &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-" + rand.String(4),
				Namespace: ns.GetName(),
			},
			Spec: v1.PodSpec{Containers: []v1.Container{
				{Name: "test", Image: "test"},
			}},
		}
		cfgMap = &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "config-" + rand.String(4),
				Namespace: ns.GetName(),
			},
			Data: map[string]string{"foo": "bar"},
		}
		clientFunc = NewCachingClientFunc()
		Expect(clientFunc).NotTo(BeNil())
	})

	When("the ClientBuilder is valid", func() {
		var (
			c  cache.Cache
			cl client.Client
		)

		BeforeEach(func() {
			var err error
			c, err = cache.New(cfg, cache.Options{})
			Expect(err).To(BeNil())

			cl, err = clientFunc(c, cfg, client.Options{}, cfgMap)
			Expect(err).To(BeNil())

			Expect(cl.Create(context.TODO(), ns)).To(Succeed())
			Expect(cl.Create(context.TODO(), pod)).To(Succeed())
			Expect(cl.Create(context.TODO(), cfgMap)).To(Succeed())
		})
		AfterEach(func() {
			Eventually(func() error { return client.IgnoreNotFound(cl.Delete(context.TODO(), pod)) }).Should(BeNil())
			Eventually(func() error { return client.IgnoreNotFound(cl.Delete(context.TODO(), cfgMap)) }).Should(BeNil())
			Eventually(func() error { return client.IgnoreNotFound(cl.Delete(context.TODO(), ns)) }).Should(BeNil())
		})

		When("caches are not started", func() {
			It("should succeed on uncached objects", func() {
				Expect(cl.Get(context.TODO(), client.ObjectKeyFromObject(cfgMap), cfgMap)).To(Succeed())
			})
			It("should error on cached unstructured objects (PENDING: https://github.com/kubernetes-sigs/controller-runtime/pull/1332)", func() {
				Expect(cl.Get(context.TODO(), client.ObjectKeyFromObject(ns), ns)).To(BeAssignableToTypeOf(&cache.ErrCacheNotStarted{}))
			})
			It("should error on cached structured objects", func() {
				Expect(cl.Get(context.TODO(), client.ObjectKeyFromObject(pod), pod)).To(BeAssignableToTypeOf(&cache.ErrCacheNotStarted{}))
			})
		})

		When("caches are started", func() {
			var (
				ctx    context.Context
				cancel context.CancelFunc
				wg     *sync.WaitGroup
			)

			BeforeEach(func() {
				ctx, cancel = context.WithCancel(context.Background())
				wg = &sync.WaitGroup{}
				wg.Add(1)
				go func() {
					Expect(c.Start(ctx)).To(Succeed())
					wg.Done()
				}()
				Expect(c.WaitForCacheSync(ctx)).To(BeTrue())
			})
			AfterEach(func() {
				cancel()
				wg.Wait()
			})
			It("should return all objects", func() {
				Expect(cl.Get(context.TODO(), client.ObjectKeyFromObject(ns), ns)).To(Succeed())
				Expect(cl.Get(context.TODO(), client.ObjectKeyFromObject(pod), pod)).To(Succeed())
				Expect(cl.Get(context.TODO(), client.ObjectKeyFromObject(cfgMap), cfgMap)).To(Succeed())
			})
		})
	})

	It("should fail with an invalid config", func() {
		c, err := cache.New(cfg, cache.Options{})
		Expect(err).To(BeNil())

		badConfig := rest.Config{
			Host: "/path/to/foobar",
		}
		_, err = clientFunc(c, &badConfig, client.Options{})
		Expect(err).NotTo(BeNil())
	})
})
