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

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/kube"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/cli-runtime/pkg/resource"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/operator-framework/helm-operator-plugins/pkg/internal/testutil"
)

var _ = Describe("ActionConfig", func() {
	var _ = Describe("NewActionConfigGetter", func() {
		var rm meta.RESTMapper

		BeforeEach(func() {
			var err error
			rm, err = apiutil.NewDiscoveryRESTMapper(cfg)
			Expect(err).To(BeNil())
		})

		It("should return a valid ActionConfigGetter", func() {
			acg, err := NewActionConfigGetter(cfg, nil, logr.Discard())
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
				Expect(err).To(BeNil())
			})

			It("should use a custom client namespace", func() {
				clientNs := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("client-%s", rand.String(8))}}
				clientNsMapper := func(_ client.Object) (string, error) { return clientNs.Name, nil }
				acg, err := NewActionConfigGetter(cfg, rm, logr.Discard(),
					ClientNamespaceMapper(clientNsMapper),
				)
				Expect(err).To(BeNil())
				ac, err := acg.ActionConfigFor(obj)
				Expect(err).To(BeNil())
				Expect(ac.KubeClient.(*kube.Client).Namespace).To(Equal(clientNs.Name))
				Expect(ac.RESTClientGetter.(*namespacedRCG).namespaceConfig.Namespace()).To(Equal(clientNs.Name))
				resources, err := ac.KubeClient.Build(bytes.NewBufferString(`---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: sa`), false)
				Expect(err).To(BeNil())
				Expect(resources.Visit(func(info *resource.Info, err error) error {
					Expect(err).To(BeNil())
					Expect(info.Namespace).To(Equal(clientNs.Name))
					return nil
				})).To(Succeed())
			})

			It("should use a custom storage namespace", func() {
				storageNs := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("storage-%s", rand.String(8))}}
				storageNsMapper := func(_ client.Object) (string, error) { return storageNs.Name, nil }
				acg, err := NewActionConfigGetter(cfg, rm, logr.Discard(),
					StorageNamespaceMapper(storageNsMapper),
				)
				Expect(err).To(BeNil())

				ac, err := acg.ActionConfigFor(obj)
				Expect(err).To(BeNil())

				By("Creating the storage namespace")
				Expect(cl.Create(context.Background(), storageNs)).To(Succeed())

				By("Installing a release")
				i := action.NewInstall(ac)
				i.ReleaseName = fmt.Sprintf("release-name-%s", rand.String(8))
				i.Namespace = obj.GetNamespace()
				rel, err := i.Run(&chrt, nil)
				Expect(err).To(BeNil())
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
				Expect(err).To(BeNil())

				By("Deleting the storage namespace")
				Expect(cl.Delete(context.Background(), storageNs)).To(Succeed())
			})

			It("should disable storage owner ref injection", func() {
				acg, err := NewActionConfigGetter(cfg, rm, logr.Discard(),
					DisableStorageOwnerRefInjection(true),
				)
				Expect(err).To(BeNil())

				ac, err := acg.ActionConfigFor(obj)
				Expect(err).To(BeNil())

				By("Installing a release")
				i := action.NewInstall(ac)
				i.ReleaseName = fmt.Sprintf("release-name-%s", rand.String(8))
				i.Namespace = obj.GetNamespace()
				rel, err := i.Run(&chrt, nil)
				Expect(err).To(BeNil())
				Expect(rel.Namespace).To(Equal(obj.GetNamespace()))

				By("Verifying the release secret has no owner references")
				secretKey := types.NamespacedName{
					Namespace: obj.GetNamespace(),
					Name:      fmt.Sprintf("sh.helm.release.v1.%s.v1", i.ReleaseName),
				}
				secret := &corev1.Secret{}
				Expect(cl.Get(context.Background(), secretKey, secret)).To(Succeed())
				Expect(secret.OwnerReferences).To(HaveLen(0))

				By("Uninstalling the release")
				_, err = action.NewUninstall(ac).Run(i.ReleaseName)
				Expect(err).To(BeNil())
			})
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

			acg, err := NewActionConfigGetter(cfg, rm, logr.Discard())
			Expect(err).ShouldNot(HaveOccurred())
			ac, err := acg.ActionConfigFor(obj)
			Expect(err).To(BeNil())
			Expect(ac).NotTo(BeNil())
		})
	})
})
