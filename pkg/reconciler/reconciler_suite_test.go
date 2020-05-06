package reconciler

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestReconciler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Reconciler Suite")
}

var (
	testenv *envtest.Environment
	cfg     *rest.Config
)

var _ = BeforeSuite(func(done Done) {
	Expect(os.Setenv("TEST_ASSET_KUBE_APISERVER", "../../testbin/kube-apiserver")).To(Succeed())
	Expect(os.Setenv("TEST_ASSET_ETCD", "../../testbin/etcd")).To(Succeed())
	Expect(os.Setenv("TEST_ASSET_KUBECTL", "../../testbin/kubectl")).To(Succeed())

	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	testenv = &envtest.Environment{}

	var err error
	cfg, err = testenv.Start()
	Expect(err).NotTo(HaveOccurred())

	close(done)
}, 60)

var _ = AfterSuite(func() {
	Expect(testenv.Stop()).To(Succeed())

	Expect(os.Unsetenv("TEST_ASSET_KUBE_APISERVER")).To(Succeed())
	Expect(os.Unsetenv("TEST_ASSET_ETCD")).To(Succeed())
	Expect(os.Unsetenv("TEST_ASSET_KUBECTL")).To(Succeed())

})
