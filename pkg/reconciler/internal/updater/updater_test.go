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

package updater

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/release"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/joelanford/helm-operator/pkg/reconciler/internal/conditions"
)

const testFinalizer = "testFinalizer"

var _ = Describe("Updater", func() {
	var (
		client client.Client
		u      Updater
		obj    *unstructured.Unstructured
	)

	BeforeEach(func() {
		client = fake.NewFakeClientWithScheme(scheme.Scheme)
		u = New(client)
		obj = &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "testDeployment",
				"namespace": "testNamespace",
			},
			"spec": map[string]interface{}{},
		}}
		Expect(client.Create(context.TODO(), obj)).To(Succeed())
	})

	When("the object does not exist", func() {
		It("should fail", func() {
			Expect(client.Delete(context.TODO(), obj)).To(Succeed())
			u.Update(EnsureFinalizer(testFinalizer))
			err := u.Apply(context.TODO(), obj)
			Expect(err).NotTo(BeNil())
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})
	})

	When("an update is a change", func() {
		It("should apply an update function", func() {
			u.Update(EnsureFinalizer(testFinalizer))
			resourceVersion := obj.GetResourceVersion()

			Expect(u.Apply(context.TODO(), obj)).To(Succeed())
			Expect(client.Get(context.TODO(), types.NamespacedName{Namespace: "testNamespace", Name: "testDeployment"}, obj)).To(Succeed())
			Expect(obj.GetFinalizers()).To(Equal([]string{testFinalizer}))
			Expect(obj.GetResourceVersion()).NotTo(Equal(resourceVersion))
		})

		It("should apply an update status function", func() {
			u.UpdateStatus(EnsureCondition(conditions.Deployed(corev1.ConditionTrue, "", "")))
			resourceVersion := obj.GetResourceVersion()

			Expect(u.Apply(context.TODO(), obj)).To(Succeed())
			Expect(client.Get(context.TODO(), types.NamespacedName{Namespace: "testNamespace", Name: "testDeployment"}, obj)).To(Succeed())
			Expect((obj.Object["status"].(map[string]interface{}))["conditions"]).To(HaveLen(1))
			Expect(obj.GetResourceVersion()).NotTo(Equal(resourceVersion))
		})
	})
})

var _ = Describe("EnsureFinalizer", func() {
	var obj *unstructured.Unstructured

	BeforeEach(func() {
		obj = &unstructured.Unstructured{}
	})

	It("should add finalizer if not present", func() {
		Expect(EnsureFinalizer(testFinalizer)(obj)).To(BeTrue())
		Expect(obj.GetFinalizers()).To(Equal([]string{testFinalizer}))
	})

	It("should not add duplicate finalizer", func() {
		obj.SetFinalizers([]string{testFinalizer})
		Expect(EnsureFinalizer(testFinalizer)(obj)).To(BeFalse())
		Expect(obj.GetFinalizers()).To(Equal([]string{testFinalizer}))
	})
})

var _ = Describe("RemoveFinalizer", func() {
	var obj *unstructured.Unstructured

	BeforeEach(func() {
		obj = &unstructured.Unstructured{}
	})

	It("should remove finalizer if present", func() {
		obj.SetFinalizers([]string{testFinalizer})
		Expect(RemoveFinalizer(testFinalizer)(obj)).To(BeTrue())
		Expect(obj.GetFinalizers()).To(BeEmpty())
	})

	It("should return false if finalizer is not present", func() {
		Expect(RemoveFinalizer(testFinalizer)(obj)).To(BeFalse())
		Expect(obj.GetFinalizers()).To(BeEmpty())
	})
})

var _ = Describe("EnsureCondition", func() {
	var obj *helmAppStatus

	BeforeEach(func() {
		obj = &helmAppStatus{}
	})

	It("should add condition if not present", func() {
		Expect(EnsureCondition(conditions.Deployed(corev1.ConditionTrue, "", ""))(obj)).To(BeTrue())
		Expect(obj.Conditions.IsTrueFor(conditions.TypeDeployed)).To(BeTrue())
	})

	It("should not add duplicate condition", func() {
		obj.Conditions.SetCondition(conditions.Deployed(corev1.ConditionTrue, "", ""))
		Expect(EnsureCondition(conditions.Deployed(corev1.ConditionTrue, "", ""))(obj)).To(BeFalse())
		Expect(obj.Conditions.IsTrueFor(conditions.TypeDeployed)).To(BeTrue())
	})
})

var _ = Describe("EnsureDeployedRelease", func() {
	var obj *helmAppStatus
	var rel *release.Release
	var statusRelease *helmAppRelease

	BeforeEach(func() {
		obj = &helmAppStatus{}
		rel = &release.Release{
			Name:     "initialName",
			Manifest: "initialManifest",
		}
		statusRelease = &helmAppRelease{
			Name:     "initialName",
			Manifest: "initialManifest",
		}
	})

	It("should add deployed release if not present", func() {
		Expect(EnsureDeployedRelease(rel)(obj)).To(BeTrue())
		Expect(obj.DeployedRelease).To(Equal(statusRelease))
	})

	It("should not update identical deployed release", func() {
		obj.DeployedRelease = statusRelease
		Expect(EnsureDeployedRelease(rel)(obj)).To(BeFalse())
		Expect(obj.DeployedRelease).To(Equal(statusRelease))
	})

	It("should update deployed release if different name", func() {
		obj.DeployedRelease = statusRelease
		Expect(EnsureDeployedRelease(&release.Release{Name: "newName", Manifest: "initialManifest"})(obj)).To(BeTrue())
		Expect(obj.DeployedRelease).To(Equal(&helmAppRelease{Name: "newName", Manifest: "initialManifest"}))
	})

	It("should update deployed release if different manifest", func() {
		obj.DeployedRelease = statusRelease
		Expect(EnsureDeployedRelease(&release.Release{Name: "initialName", Manifest: "newManifest"})(obj)).To(BeTrue())
		Expect(obj.DeployedRelease).To(Equal(&helmAppRelease{Name: "initialName", Manifest: "newManifest"}))
	})
})

var _ = Describe("RemoveDeployedRelease", func() {
	var obj *helmAppStatus
	var statusRelease *helmAppRelease

	BeforeEach(func() {
		obj = &helmAppStatus{}
		statusRelease = &helmAppRelease{
			Name:     "initialName",
			Manifest: "initialManifest",
		}
	})

	It("should remove deployed release if present", func() {
		obj.DeployedRelease = statusRelease
		Expect(RemoveDeployedRelease()(obj)).To(BeTrue())
		Expect(obj.DeployedRelease).To(BeNil())
	})

	It("should not update if deployed release is already nil", func() {
		Expect(RemoveDeployedRelease()(obj)).To(BeFalse())
		Expect(obj.DeployedRelease).To(BeNil())
	})
})

var _ = Describe("statusFor", func() {
	var obj *unstructured.Unstructured

	BeforeEach(func() {
		obj = &unstructured.Unstructured{Object: map[string]interface{}{}}
	})

	It("should handle nil", func() {
		obj.Object = nil
		Expect(statusFor(obj)).To(BeNil())

		obj = nil
		Expect(statusFor(obj)).To(BeNil())
	})

	It("should handle status not present", func() {
		Expect(statusFor(obj)).To(Equal(&helmAppStatus{}))
	})

	It("should handle *helmAppsStatus", func() {
		obj.Object["status"] = &helmAppStatus{}
		Expect(statusFor(obj)).To(Equal(&helmAppStatus{}))
	})

	It("should handle helmAppsStatus", func() {
		obj.Object["status"] = helmAppStatus{}
		Expect(statusFor(obj)).To(Equal(&helmAppStatus{}))
	})

	It("should handle map[string]interface{}", func() {
		obj.Object["status"] = map[string]interface{}{}
		Expect(statusFor(obj)).To(Equal(&helmAppStatus{}))
	})

	It("should handle arbitrary types", func() {
		obj.Object["status"] = 10
		Expect(statusFor(obj)).To(Equal(&helmAppStatus{}))

		obj.Object["status"] = "hello"
		Expect(statusFor(obj)).To(Equal(&helmAppStatus{}))
	})
})
