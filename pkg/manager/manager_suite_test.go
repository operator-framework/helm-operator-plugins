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
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestManager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Manager Suite")
}

var (
	testenv *envtest.Environment
	cfg     *rest.Config
)

var _ = BeforeSuite(func(done Done) {
	// todo: do we really need to set it? Can we not use the k8s and kubectl from the local setup?
	// check if would be a better solution
	// also, it shows that were are doing here e2e test. Could we make clear what is e2e and not?
	// Also, is not the go of `NewActionClientGetter` allow us test it as unit and in this way we would not require the bin/env at all?
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

// Unable to run locally. Probably becaue the testbin is not working for me.
// Unexpected error:
//      <*fmt.wrapError | 0xc0001209c0>: {
//          msg: "failed to start the controlplane. retried 5 times: fork/exec ../../testbin/etcd: no such file or directory",
//          err: {
//              Op: "fork/exec",
//              Path: "../../testbin/etcd",
//              Err: 0x2,
//          },
//      }
//      failed to start the controlplane. retried 5 times: fork/exec ../../testbin/etcd: no such file or directory
//  occurred
