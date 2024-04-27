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
	"bytes"
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/kube"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/cli-runtime/pkg/resource"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/operator-framework/helm-operator-plugins/pkg/internal/testutil"
)

var _ = Describe("ActionConfig", func() {
	var _ = Describe("NewActionConfigGetter", func() {
		var (
			rm  meta.RESTMapper
			sch *runtime.Scheme
		)
		BeforeEach(func() {
			var err error

			httpClient, err := rest.HTTPClientFor(cfg)
			Expect(err).NotTo(HaveOccurred())

			rm, err = apiutil.NewDynamicRESTMapper(cfg, httpClient)
			Expect(err).ToNot(HaveOccurred())

			sch = runtime.NewScheme()
		})

		It("should return a valid ActionConfigGetter", func() {
			acg, err := NewActionConfigGetter(cfg, nil, nil)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(acg).NotTo(BeNil())
		})

		When("passing options", func() {
			var (
				obj client.Object
				cl  client.Client
			)

			BeforeEach(func() {
				obj = testutil.BuildTestCR(gvk)

				var err error
				cl, err = client.New(cfg, client.Options{Scheme: clientgoscheme.Scheme})
				Expect(err).ToNot(HaveOccurred())
			})

			It("should use a custom client namespace", func() {
				clientNs := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("client-%s", rand.String(8))}}
				clientNsMapper := func(_ client.Object) (string, error) { return clientNs.Name, nil }
				acg, err := NewActionConfigGetter(cfg, rm, sch, ClientNamespaceMapper(clientNsMapper))
				Expect(err).ToNot(HaveOccurred())
				ac, err := acg.ActionConfigFor(context.Background(), obj)
				Expect(err).ToNot(HaveOccurred())
				Expect(ac.KubeClient.(*kube.Client).Namespace).To(Equal(clientNs.Name))
				Expect(ac.RESTClientGetter.(*namespacedRCG).namespaceConfig.Namespace()).To(Equal(clientNs.Name))
				resources, err := ac.KubeClient.Build(bytes.NewBufferString(`---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: sa`), false)
				Expect(err).ToNot(HaveOccurred())
				Expect(resources.Visit(func(info *resource.Info, err error) error {
					Expect(err).ToNot(HaveOccurred())
					Expect(info.Namespace).To(Equal(clientNs.Name))
					return nil
				})).To(Succeed())
			})

			It("should use a custom storage namespace", func() {
				storageNs := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("storage-%s", rand.String(8))}}
				storageNsMapper := func(_ client.Object) (string, error) { return storageNs.Name, nil }
				acg, err := NewActionConfigGetter(cfg, rm, sch, StorageNamespaceMapper(storageNsMapper))
				Expect(err).ToNot(HaveOccurred())

				ac, err := acg.ActionConfigFor(context.Background(), obj)
				Expect(err).ToNot(HaveOccurred())

				By("Creating the storage namespace")
				Expect(cl.Create(context.Background(), storageNs)).To(Succeed())

				By("Installing a release")
				i := action.NewInstall(ac)
				i.ReleaseName = fmt.Sprintf("release-name-%s", rand.String(8))
				i.Namespace = obj.GetNamespace()
				rel, err := i.Run(&chrt, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(rel.Namespace).To(Equal(obj.GetNamespace()))

				By("Verifying the release secret is created in the storage namespace")
				secretKey := types.NamespacedName{
					Namespace: storageNs.Name,
					Name:      fmt.Sprintf("sh.helm.release.v1.%s.v1", i.ReleaseName),
				}
				secret := &corev1.Secret{}
				Expect(cl.Get(context.Background(), secretKey, secret)).To(Succeed())
				Expect(secret.OwnerReferences).To(HaveLen(1))

				By("Uninstalling the release")
				_, err = action.NewUninstall(ac).Run(i.ReleaseName)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting the storage namespace")
				Expect(cl.Delete(context.Background(), storageNs)).To(Succeed())
			})

			It("should disable storage owner ref injection", func() {
				acg, err := NewActionConfigGetter(cfg, rm, sch, DisableStorageOwnerRefInjection(true))
				Expect(err).ToNot(HaveOccurred())

				ac, err := acg.ActionConfigFor(context.Background(), obj)
				Expect(err).ToNot(HaveOccurred())

				By("Installing a release")
				i := action.NewInstall(ac)
				i.ReleaseName = fmt.Sprintf("release-name-%s", rand.String(8))
				i.Namespace = obj.GetNamespace()
				rel, err := i.Run(&chrt, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(rel.Namespace).To(Equal(obj.GetNamespace()))

				By("Verifying the release secret has no owner references")
				secretKey := types.NamespacedName{
					Namespace: obj.GetNamespace(),
					Name:      fmt.Sprintf("sh.helm.release.v1.%s.v1", i.ReleaseName),
				}
				secret := &corev1.Secret{}
				Expect(cl.Get(context.Background(), secretKey, secret)).To(Succeed())
				Expect(secret.OwnerReferences).To(BeEmpty())

				By("Uninstalling the release")
				_, err = action.NewUninstall(ac).Run(i.ReleaseName)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should use a custom rest config mapping", func() {
				restConfigMapper := func(ctx context.Context, obj client.Object, cfg *rest.Config) (*rest.Config, error) {
					return &rest.Config{
						BearerToken: obj.GetName(),
					}, nil
				}
				acg, err := NewActionConfigGetter(cfg, rm, sch, RestConfigMapper(restConfigMapper))
				Expect(err).ToNot(HaveOccurred())

				testObject := func(name string) client.Object {
					u := unstructured.Unstructured{}
					u.SetName(name)
					return &u
				}

				ac1, err := acg.ActionConfigFor(context.Background(), testObject("test1"))
				Expect(err).ToNot(HaveOccurred())
				Expect(ac1.RESTClientGetter.ToRESTConfig()).To(WithTransform(func(c *rest.Config) string { return c.BearerToken }, Equal("test1")))

				ac2, err := acg.ActionConfigFor(context.Background(), testObject("test2"))
				Expect(err).ToNot(HaveOccurred())
				Expect(ac2.RESTClientGetter.ToRESTConfig()).To(WithTransform(func(c *rest.Config) string { return c.BearerToken }, Equal("test2")))
			})
		})
	})

	var _ = Describe("GetActionConfig", func() {
		var obj client.Object
		BeforeEach(func() {
			obj = testutil.BuildTestCR(gvk)
		})
		It("should return a valid action.Configuration", func() {
			httpClient, err := rest.HTTPClientFor(cfg)
			Expect(err).NotTo(HaveOccurred())

			rm, err := apiutil.NewDynamicRESTMapper(cfg, httpClient)
			Expect(err).ToNot(HaveOccurred())
			sch := runtime.NewScheme()

			acg, err := NewActionConfigGetter(cfg, rm, sch)
			Expect(err).ShouldNot(HaveOccurred())
			ac, err := acg.ActionConfigFor(context.Background(), obj)
			Expect(err).ToNot(HaveOccurred())
			Expect(ac).NotTo(BeNil())
		})
	})
})
