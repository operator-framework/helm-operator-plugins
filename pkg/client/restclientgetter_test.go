package client

import (
	. "github.com/onsi/ginkgo"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/rest"
)

var _ = PDescribe("restClientGetter", func() {
	var _ = PDescribe("newRESTClientGetter", func() {
		var (
			cfg *rest.Config
			rm  meta.RESTMapper
		)

		BeforeEach(func() {

		})

		It("returns a working genericclioptions.RESTClientGetter", func() {
			newRESTClientGetter(cfg, rm, "")
		})
	})
})
