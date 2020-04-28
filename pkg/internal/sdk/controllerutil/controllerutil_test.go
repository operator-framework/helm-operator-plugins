package controllerutil_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/joelanford/helm-operator/pkg/internal/sdk/controllerutil"
)

var _ = Describe("Controllerutil", func() {
	Describe("WaitForDeletion", func() {
		var (
			ctx    context.Context
			cancel context.CancelFunc
			pod    *v1.Pod
			client client.Client
		)

		BeforeEach(func() {
			ctx, cancel = context.WithCancel(context.Background())
			pod = &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testName",
					Namespace: "testNamespace",
				},
			}
			client = fake.NewFakeClientWithScheme(scheme.Scheme, pod)
		})

		AfterEach(func() {
			cancel()
		})

		It("should be cancellable", func() {
			cancel()
			Expect(controllerutil.WaitForDeletion(ctx, client, pod)).To(MatchError(wait.ErrWaitTimeout))
		})

		It("should succeed after pod is deleted", func() {
			Expect(client.Delete(ctx, pod)).To(Succeed())
			Expect(controllerutil.WaitForDeletion(ctx, client, pod)).To(Succeed())
		})
	})

	PDescribe("SupportsOwnerReference", func() {})
})
