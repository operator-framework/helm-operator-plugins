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
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	. "github.com/operator-framework/helm-operator-plugins/pkg/manager"
)

var _ = Describe("ConfigureWatchNamespaces", func() {
	var (
		opts manager.Options
		log  = logr.Discard()
	)

	BeforeEach(func() {
		Expect(os.Unsetenv(WatchNamespaceEnvVar)).To(Succeed())
	})

	When("should watch all namespaces", func() {
		var (
			watchedPods []corev1.Pod
			err         error
		)
		BeforeEach(func() {
			By("creating pods in watched namespaces")
			watchedPods, err = createPods(context.TODO(), 2)
			Expect(err).ToNot(HaveOccurred())
		})
		It("should watch all namespaces when no env set", func() {
			By("configuring WATCH_NAMESPACE with the namespaces of the watched pods")
			ConfigureWatchNamespaces(&opts, log)

			By("creating the manager")
			mgr, err := manager.New(cfg, opts)
			Expect(err).ToNot(HaveOccurred())

			By("starting the manager")
			ctx, cancel := context.WithCancel(context.Background())
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				Expect(mgr.Start(ctx)).To(Succeed())
				wg.Done()
			}()

			By("waiting for the cache to sync")
			c := mgr.GetCache()
			Expect(c.WaitForCacheSync(ctx)).To(BeTrue())

			By("successfully getting the watched pods")
			for i := range watchedPods {
				p := watchedPods[i]
				key := client.ObjectKeyFromObject(&p)
				Expect(c.Get(context.TODO(), key, &p)).To(Succeed())
			}
			cancel()
			wg.Wait()
		})

		It("should watch all namespaces when WATCH_NAMESPACE is empty", func() {
			By("configuring WATCH_NAMESPACE with empty string")
			Expect(os.Setenv(WatchNamespaceEnvVar, "")).To(Succeed())

			By("configuring WATCH_NAMESPACE with the namespaces of the watched pods")
			ConfigureWatchNamespaces(&opts, log)

			By("creating the manager")
			mgr, err := manager.New(cfg, opts)
			Expect(err).ToNot(HaveOccurred())

			By("starting the manager")
			ctx, cancel := context.WithCancel(context.Background())
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				Expect(mgr.Start(ctx)).To(Succeed())
				wg.Done()
			}()

			By("waiting for the cache to sync")
			c := mgr.GetCache()
			Expect(c.WaitForCacheSync(ctx)).To(BeTrue())

			By("successfully getting the watched pods")
			for i := range watchedPods {
				p := watchedPods[i]
				key := client.ObjectKeyFromObject(&p)
				Expect(c.Get(context.TODO(), key, &p)).To(Succeed())
			}
			cancel()
			wg.Wait()
		})

	})
	When("should watch specified namespaces", func() {
		It("should watch multiple namespaces when WATCH_NAMESPACE has multiple namespaces", func() {
			By("creating pods in watched namespaces")
			watchedPods, err := createPods(context.TODO(), 2)
			Expect(err).ToNot(HaveOccurred())

			By("creating pods in watched namespaces")
			unwatchedPods, err := createPods(context.TODO(), 2)
			Expect(err).ToNot(HaveOccurred())

			By("configuring WATCH_NAMESPACE with the namespaces of the watched pods")
			Expect(os.Setenv(WatchNamespaceEnvVar, strings.Join(getNamespaces(watchedPods), ","))).To(Succeed())
			ConfigureWatchNamespaces(&opts, log)

			By("creating the manager")
			mgr, err := manager.New(cfg, opts)
			Expect(err).ToNot(HaveOccurred())

			By("starting the manager")
			ctx, cancel := context.WithCancel(context.Background())
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				Expect(mgr.Start(ctx)).To(Succeed())
				wg.Done()
			}()

			By("waiting for the cache to sync")
			c := mgr.GetCache()
			Expect(c.WaitForCacheSync(ctx)).To(BeTrue())

			By("successfully getting the watched pods")
			for i := range watchedPods {
				p := watchedPods[i]
				key := client.ObjectKeyFromObject(&p)
				Expect(c.Get(context.TODO(), key, &p)).To(Succeed())
			}

			By("failing to get the unwatched pods")
			for i := range unwatchedPods {
				p := unwatchedPods[i]
				key := client.ObjectKeyFromObject(&p)
				Expect(c.Get(context.TODO(), key, &p)).NotTo(Succeed())
			}
			cancel()
			wg.Wait()
		})
	})
})

func createPods(ctx context.Context, count int) ([]corev1.Pod, error) {
	cl, err := client.New(cfg, client.Options{})
	if err != nil {
		return nil, err
	}

	pods := make([]corev1.Pod, count)
	for i := 0; i < count; i++ {
		nsName := fmt.Sprintf("watch-%s", rand.String(5))
		ns := corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: nsName,
			},
		}
		pod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: nsName,
			},
			Spec: corev1.PodSpec{Containers: []corev1.Container{
				{Name: "test", Image: "test"},
			}},
		}
		if err := cl.Create(ctx, &ns); err != nil {
			return nil, err
		}
		if err := cl.Create(ctx, &pod); err != nil {
			return nil, err
		}
		pods[i] = pod
	}
	return pods, nil
}

func getNamespaces(objs []corev1.Pod) []string {
	namespaces := sets.New[string]()
	for _, obj := range objs {
		namespaces.Insert(obj.GetNamespace())
	}
	return namespaces.UnsortedList()
}
