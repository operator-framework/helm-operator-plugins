package manager_test

import (
	"os"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	. "github.com/joelanford/helm-operator/pkg/manager"
)

var _ = Describe("ConfigureWatchNamespaces", func() {
	var (
		opts manager.Options
		log  logr.Logger = &testing.NullLogger{}
	)

	BeforeEach(func() {
		opts = manager.Options{}
		Expect(os.Unsetenv(WatchNamespacesEnvVar)).To(Succeed())
		Expect(os.Unsetenv(WatchNamespaceEnvVar)).To(Succeed())
	})

	It("should watch all namespaces when no env set", func() {
		ConfigureWatchNamespaces(&opts, log)
		Expect(opts.Namespace).To(Equal(""))
		Expect(opts.NewCache).To(BeNil())
	})

	It("should watch all namespaces when WATCH_NAMESPACES is empty", func() {
		Expect(os.Setenv(WatchNamespacesEnvVar, ""))
		ConfigureWatchNamespaces(&opts, log)
		Expect(opts.Namespace).To(Equal(""))
		Expect(opts.NewCache).To(BeNil())
	})

	It("should watch all namespaces when WATCH_NAMESPACE is empty", func() {
		Expect(os.Setenv(WatchNamespaceEnvVar, ""))
		ConfigureWatchNamespaces(&opts, log)
		Expect(opts.Namespace).To(Equal(""))
		Expect(opts.NewCache).To(BeNil())
	})

	It("should watch one namespace when WATCH_NAMESPACES is has one namespace", func() {
		Expect(os.Setenv(WatchNamespacesEnvVar, "watch"))
		ConfigureWatchNamespaces(&opts, log)
		Expect(opts.Namespace).To(Equal("watch"))
		Expect(opts.NewCache).To(BeNil())
	})

	It("should watch one namespace when WATCH_NAMESPACE is has one namespace", func() {
		Expect(os.Setenv(WatchNamespaceEnvVar, "watch"))
		ConfigureWatchNamespaces(&opts, log)
		Expect(opts.Namespace).To(Equal("watch"))
		Expect(opts.NewCache).To(BeNil())
	})

	It("should watch multiple namespaces when WATCH_NAMESPACES has multiple namespaces", func() {
		Expect(os.Setenv(WatchNamespacesEnvVar, "watch1,watch2"))
		ConfigureWatchNamespaces(&opts, log)
		Expect(opts.Namespace).To(Equal(""))

		// TODO(joelanford): Update this to use testenv.
		//   There isn't an easy way to test that the NewCache
		//   is a multi-namespace cache watching watch1 and watch2 without
		//   using testenv.
		Expect(opts.NewCache).NotTo(BeNil())
	})

	It("should watch multiple namespaces when WATCH_NAMESPACE has multiple namespaces", func() {
		Expect(os.Setenv(WatchNamespaceEnvVar, "watch1,watch2"))
		ConfigureWatchNamespaces(&opts, log)
		Expect(opts.Namespace).To(Equal(""))

		// TODO(joelanford): Update this to use testenv.
		//   There isn't an easy way to test that the NewCache
		//   is a multi-namespace cache watching watch1 and watch2 without
		//   using testenv.
		Expect(opts.NewCache).NotTo(BeNil())
	})

	It("should prefer WATCH_NAMESPACES over WATCH_NAMESPACE", func() {
		Expect(os.Setenv(WatchNamespacesEnvVar, "watches"))
		Expect(os.Setenv(WatchNamespaceEnvVar, "watch"))
		ConfigureWatchNamespaces(&opts, log)
		Expect(opts.Namespace).To(Equal("watches"))
		Expect(opts.NewCache).To(BeNil())
	})

})
