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
	"context"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	sdkhandler "github.com/operator-framework/operator-lib/handler"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	"github.com/operator-framework/helm-operator-plugins/pkg/extension"
	"github.com/operator-framework/helm-operator-plugins/pkg/internal/fake"
	internalhook "github.com/operator-framework/helm-operator-plugins/pkg/reconciler/internal/hook"
)

var _ = Describe("Hook", func() {
	Describe("dependentResourceWatcher", func() {
		var (
			drw   extension.ReconcilerExtension
			c     *fake.Controller
			rm    *meta.DefaultRESTMapper
			owner *unstructured.Unstructured
			rel   *release.Release
		)

		BeforeEach(func() {
			rm = meta.NewDefaultRESTMapper([]schema.GroupVersion{})
			c = &fake.Controller{}
		})

		Context("with unknown APIs", func() {
			BeforeEach(func() {
				owner = &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name":      "testDeployment",
							"namespace": "ownerNamespace",
						},
					},
				}
				rel = &release.Release{
					Manifest: strings.Join([]string{rsOwnerNamespace}, "---\n"),
				}
				drw = internalhook.NewDependentResourceWatcher(c, rm)
			})
			It("should fail with an invalid release manifest", func() {
				rel.Manifest = "---\nfoobar"
				err := drw.EndReconcile(context.TODO(), &extension.Context{HelmRelease: rel}, owner)
				Expect(err).NotTo(BeNil())
			})
			It("should fail with unknown owner kind", func() {
				Expect(drw.EndReconcile(context.TODO(), &extension.Context{HelmRelease: rel}, owner)).To(MatchError(&meta.NoKindMatchError{
					GroupKind:        schema.GroupKind{Group: "apps", Kind: "Deployment"},
					SearchedVersions: []string{"v1"},
				}))
			})
			It("should fail with unknown dependent kind", func() {
				rm.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, meta.RESTScopeNamespace)
				Expect(drw.EndReconcile(context.TODO(), &extension.Context{HelmRelease: rel}, owner)).To(MatchError(&meta.NoKindMatchError{
					GroupKind:        schema.GroupKind{Group: "apps", Kind: "ReplicaSet"},
					SearchedVersions: []string{"v1"},
				}))
			})
		})

		Context("with known APIs", func() {
			BeforeEach(func() {
				rm.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, meta.RESTScopeNamespace)
				rm.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "ReplicaSet"}, meta.RESTScopeNamespace)
				rm.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"}, meta.RESTScopeNamespace)
				rm.Add(schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRole"}, meta.RESTScopeRoot)
				rm.Add(schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRoleBinding"}, meta.RESTScopeRoot)
			})

			It("should watch resource kinds only once each", func() {
				owner = &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "rbac.authorization.k8s.io/v1",
						"kind":       "ClusterRole",
						"metadata": map[string]interface{}{
							"name": "testClusterRole",
						},
					},
				}
				rel = &release.Release{
					Manifest: strings.Join([]string{clusterRole, clusterRole, rsOwnerNamespace, rsOwnerNamespace}, "---\n"),
				}
				drw = internalhook.NewDependentResourceWatcher(c, rm)
				Expect(drw.EndReconcile(context.TODO(), &extension.Context{HelmRelease: rel}, owner)).To(Succeed())
				Expect(c.WatchCalls).To(HaveLen(2))
				Expect(c.WatchCalls[0].Handler).To(BeAssignableToTypeOf(&handler.EnqueueRequestForOwner{}))
				Expect(c.WatchCalls[1].Handler).To(BeAssignableToTypeOf(&handler.EnqueueRequestForOwner{}))
			})

			Context("when the owner is cluster-scoped", func() {
				BeforeEach(func() {
					owner = &unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "rbac.authorization.k8s.io/v1",
							"kind":       "ClusterRole",
							"metadata": map[string]interface{}{
								"name": "testClusterRole",
							},
						},
					}
				})
				It("should watch namespace-scoped resources with ownerRef handler", func() {
					rel = &release.Release{
						Manifest: strings.Join([]string{rsOwnerNamespace, ssOtherNamespace}, "---\n"),
					}
					drw = internalhook.NewDependentResourceWatcher(c, rm)
					Expect(drw.EndReconcile(context.TODO(), &extension.Context{HelmRelease: rel}, owner)).To(Succeed())
					Expect(c.WatchCalls).To(HaveLen(2))
					Expect(c.WatchCalls[0].Handler).To(BeAssignableToTypeOf(&handler.EnqueueRequestForOwner{}))
					Expect(c.WatchCalls[1].Handler).To(BeAssignableToTypeOf(&handler.EnqueueRequestForOwner{}))

				})
				It("should watch cluster-scoped resources with ownerRef handler", func() {
					rel = &release.Release{
						Manifest: strings.Join([]string{clusterRole, clusterRoleBinding}, "---\n"),
					}
					drw = internalhook.NewDependentResourceWatcher(c, rm)
					Expect(drw.EndReconcile(context.TODO(), &extension.Context{HelmRelease: rel}, owner)).To(Succeed())
					Expect(c.WatchCalls).To(HaveLen(2))
					Expect(c.WatchCalls[0].Handler).To(BeAssignableToTypeOf(&handler.EnqueueRequestForOwner{}))
					Expect(c.WatchCalls[1].Handler).To(BeAssignableToTypeOf(&handler.EnqueueRequestForOwner{}))
				})
				It("should watch resource policy keep resources with annotation handler", func() {
					rel = &release.Release{
						Manifest: strings.Join([]string{rsOwnerNamespaceWithKeep, ssOtherNamespaceWithKeep, clusterRoleWithKeep, clusterRoleBindingWithKeep}, "---\n"),
					}
					drw = internalhook.NewDependentResourceWatcher(c, rm)
					Expect(drw.EndReconcile(context.TODO(), &extension.Context{HelmRelease: rel}, owner)).To(Succeed())
					Expect(c.WatchCalls).To(HaveLen(4))
					Expect(c.WatchCalls[0].Handler).To(BeAssignableToTypeOf(&sdkhandler.EnqueueRequestForAnnotation{}))
					Expect(c.WatchCalls[1].Handler).To(BeAssignableToTypeOf(&sdkhandler.EnqueueRequestForAnnotation{}))
					Expect(c.WatchCalls[2].Handler).To(BeAssignableToTypeOf(&sdkhandler.EnqueueRequestForAnnotation{}))
					Expect(c.WatchCalls[3].Handler).To(BeAssignableToTypeOf(&sdkhandler.EnqueueRequestForAnnotation{}))
				})
			})

			Context("when the owner is namespace-scoped", func() {
				BeforeEach(func() {
					owner = &unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "apps/v1",
							"kind":       "Deployment",
							"metadata": map[string]interface{}{
								"name":      "testDeployment",
								"namespace": "ownerNamespace",
							},
						},
					}
				})
				It("should watch namespace-scoped dependent resources in the same namespace with ownerRef handler", func() {
					rel = &release.Release{
						Manifest: strings.Join([]string{rsOwnerNamespace}, "---\n"),
					}
					drw = internalhook.NewDependentResourceWatcher(c, rm)
					Expect(drw.EndReconcile(context.TODO(), &extension.Context{HelmRelease: rel}, owner)).To(Succeed())
					Expect(c.WatchCalls).To(HaveLen(1))
					Expect(c.WatchCalls[0].Handler).To(BeAssignableToTypeOf(&handler.EnqueueRequestForOwner{}))
				})
				It("should watch cluster-scoped resources with annotation handler", func() {
					rel = &release.Release{
						Manifest: strings.Join([]string{clusterRole}, "---\n"),
					}
					drw = internalhook.NewDependentResourceWatcher(c, rm)
					Expect(drw.EndReconcile(context.TODO(), &extension.Context{HelmRelease: rel}, owner)).To(Succeed())
					Expect(c.WatchCalls).To(HaveLen(1))
					Expect(c.WatchCalls[0].Handler).To(BeAssignableToTypeOf(&sdkhandler.EnqueueRequestForAnnotation{}))
				})
				It("should watch namespace-scoped resources in a different namespace with annotation handler", func() {
					rel = &release.Release{
						Manifest: strings.Join([]string{ssOtherNamespace}, "---\n"),
					}
					drw = internalhook.NewDependentResourceWatcher(c, rm)
					Expect(drw.EndReconcile(context.TODO(), &extension.Context{HelmRelease: rel}, owner)).To(Succeed())
					Expect(c.WatchCalls).To(HaveLen(1))
					Expect(c.WatchCalls[0].Handler).To(BeAssignableToTypeOf(&sdkhandler.EnqueueRequestForAnnotation{}))
				})
				It("should watch resource policy keep resources with annotation handler", func() {
					rel = &release.Release{
						Manifest: strings.Join([]string{rsOwnerNamespaceWithKeep, ssOtherNamespaceWithKeep, clusterRoleWithKeep}, "---\n"),
					}
					drw = internalhook.NewDependentResourceWatcher(c, rm)
					Expect(drw.EndReconcile(context.TODO(), &extension.Context{HelmRelease: rel}, owner)).To(Succeed())
					Expect(c.WatchCalls).To(HaveLen(3))
					Expect(c.WatchCalls[0].Handler).To(BeAssignableToTypeOf(&sdkhandler.EnqueueRequestForAnnotation{}))
					Expect(c.WatchCalls[1].Handler).To(BeAssignableToTypeOf(&sdkhandler.EnqueueRequestForAnnotation{}))
					Expect(c.WatchCalls[2].Handler).To(BeAssignableToTypeOf(&sdkhandler.EnqueueRequestForAnnotation{}))
				})
				It("should iterate the kind list and be able to set watches on each item", func() {
					rel = &release.Release{
						Manifest: strings.Join([]string{replicaSetList}, "---\n"),
					}
					drw = internalhook.NewDependentResourceWatcher(c, rm)
					Expect(drw.EndReconcile(context.TODO(), &extension.Context{HelmRelease: rel}, owner)).To(Succeed())
					Expect(c.WatchCalls).To(HaveLen(2))
					Expect(c.WatchCalls[0].Handler).To(BeAssignableToTypeOf(&handler.EnqueueRequestForOwner{}))
					Expect(c.WatchCalls[1].Handler).To(BeAssignableToTypeOf(&handler.EnqueueRequestForOwner{}))
				})
				It("should error when unable to list objects", func() {
					rel = &release.Release{
						Manifest: strings.Join([]string{errReplicaSetList}, "---\n"),
					}
					drw = internalhook.NewDependentResourceWatcher(c, rm)
					err := drw.EndReconcile(context.TODO(), &extension.Context{HelmRelease: rel}, owner)
					Expect(err).To(HaveOccurred())
				})
			})
		})
	})
})

var (
	rsOwnerNamespace = `
apiVersion: apps/v1
kind: ReplicaSet
metadata:
  name: testReplicaSet
  namespace: ownerNamespace
`
	rsOwnerNamespaceWithKeep = `
apiVersion: apps/v1
kind: ReplicaSet
metadata:
  name: testReplicaSet
  namespace: ownerNamespace
  annotations:
    helm.sh/resource-policy: keep
`
	ssOtherNamespace = `
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: otherTestStatefulSet
  namespace: otherNamespace
`
	ssOtherNamespaceWithKeep = `
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: otherTestStatefulSet
  namespace: otherNamespace
  annotations:
    helm.sh/resource-policy: keep
`
	clusterRole = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: testClusterRole
`
	clusterRoleWithKeep = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: testClusterRole
  annotations:
    helm.sh/resource-policy: keep
`
	clusterRoleBinding = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: testClusterRoleBinding
`
	clusterRoleBindingWithKeep = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: testClusterRoleBinding
  annotations:
    helm.sh/resource-policy: keep
`
	replicaSetList = `
apiVersion: v1
kind: List
items:
  - apiVersion: apps/v1
    kind: ReplicaSet
    metadata: 
      name: testReplicaSet1
      namespace: ownerNamespace
  - apiVersion: apps/v1
    kind: ReplicaSet
    metadata: 
      name: testReplicaSet2
      namespace: ownerNamespace
`
	errReplicaSetList = `
apiVersion: v1
kind: List
items:
`
)
