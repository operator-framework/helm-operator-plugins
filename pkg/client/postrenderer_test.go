package client

import (
	"bytes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/kube"
	"helm.sh/helm/v3/pkg/postrender"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/operator-framework/helm-operator-plugins/pkg/internal/testutil"
)

var _ = Describe("chainedPostRenderer", func() {
	var (
		cpr           chainedPostRenderer
		pr1, pr2, pr3 postrender.PostRenderer
	)
	BeforeEach(func() {
		cpr = nil
		pr1 = PostRendererFunc(func(in *bytes.Buffer) (*bytes.Buffer, error) {
			in.WriteString("pr1\n")
			return in, nil
		})
		pr2 = PostRendererFunc(func(in *bytes.Buffer) (*bytes.Buffer, error) {
			in.WriteString("pr2\n")
			return in, nil
		})
		pr3 = PostRendererFunc(func(in *bytes.Buffer) (*bytes.Buffer, error) {
			in.WriteString("pr3\n")
			return in, nil
		})
	})

	When("nothing is in chain", func() {
		It("leaves input unmodified", func() {
			out, err := cpr.Run(bytes.NewBufferString("original"))
			Expect(err).To(BeNil())
			Expect(out.String()).To(Equal("original"))
		})
	})

	When("one postrenderer is in chain", func() {
		BeforeEach(func() {
			cpr = append(cpr, pr1)
		})
		It("runs the postrenderer", func() {
			out, err := cpr.Run(bytes.NewBufferString("original\n"))
			Expect(err).To(BeNil())
			Expect(out.String()).To(Equal("original\npr1\n"))
		})
	})

	When("multiple postrenderers are in chain", func() {
		BeforeEach(func() {
			cpr = append(cpr, pr1, pr2, pr3)
		})
		It("runs the postrenderer", func() {
			out, err := cpr.Run(bytes.NewBufferString("original\n"))
			Expect(err).To(BeNil())
			Expect(out.String()).To(Equal("original\npr1\npr2\npr3\n"))
		})
	})
})

var _ = Describe("PostRender install options", func() {
	var (
		install *action.Install
		add     postrender.PostRenderer
	)

	BeforeEach(func() {
		base := PostRendererFunc(func(in *bytes.Buffer) (*bytes.Buffer, error) {
			in.WriteString("base\n")
			return in, nil
		})
		install = &action.Install{PostRenderer: base}
		add = PostRendererFunc(func(in *bytes.Buffer) (*bytes.Buffer, error) {
			in.WriteString("add\n")
			return in, nil
		})
	})

	Describe("WithInstallPostRenderer", func() {
		It("overrides the default postrenderer", func() {
			Expect(WithInstallPostRenderer(add)(install)).To(Succeed())
			out, err := install.PostRenderer.Run(&bytes.Buffer{})
			Expect(err).To(BeNil())
			Expect(out.String()).To(Equal("add\n"))
		})
	})
	Describe("AppendInstallPostRenderer", func() {
		It("runs the extra post renderer after the default", func() {
			Expect(AppendInstallPostRenderer(add)(install)).To(Succeed())
			out, err := install.PostRenderer.Run(&bytes.Buffer{})
			Expect(err).To(BeNil())
			Expect(out.String()).To(Equal("base\nadd\n"))
		})
		It("appends extra post renders if the existing post-render is a chainedPostRender", func() {
			Expect(AppendInstallPostRenderer(add)(install)).To(Succeed())
			Expect(AppendInstallPostRenderer(add)(install)).To(Succeed())
			out, err := install.PostRenderer.Run(&bytes.Buffer{})
			Expect(err).To(BeNil())
			Expect(out.String()).To(Equal("base\nadd\nadd\n"))
			Expect(install.PostRenderer).To(BeAssignableToTypeOf(chainedPostRenderer{}))
			Expect(install.PostRenderer).To(HaveLen(3))
		})
	})
})

var _ = Describe("PostRender upgrade options", func() {
	var (
		upgrade *action.Upgrade
		add     postrender.PostRenderer
	)

	BeforeEach(func() {
		base := PostRendererFunc(func(in *bytes.Buffer) (*bytes.Buffer, error) {
			in.WriteString("base\n")
			return in, nil
		})
		upgrade = &action.Upgrade{PostRenderer: base}
		add = PostRendererFunc(func(in *bytes.Buffer) (*bytes.Buffer, error) {
			in.WriteString("add\n")
			return in, nil
		})
	})

	Describe("WithUpgradePostRenderer", func() {
		It("overrides the default postrenderer", func() {
			Expect(WithUpgradePostRenderer(add)(upgrade)).To(Succeed())
			out, err := upgrade.PostRenderer.Run(&bytes.Buffer{})
			Expect(err).To(BeNil())
			Expect(out.String()).To(Equal("add\n"))
		})
	})
	Describe("AppendUpgradePostRenderer", func() {
		It("runs the extra post renderer after the default", func() {
			Expect(AppendUpgradePostRenderer(add)(upgrade)).To(Succeed())
			out, err := upgrade.PostRenderer.Run(&bytes.Buffer{})
			Expect(err).To(BeNil())
			Expect(out.String()).To(Equal("base\nadd\n"))
		})
		It("appends extra post renders if the existing post-render is a chainedPostRender", func() {
			Expect(AppendUpgradePostRenderer(add)(upgrade)).To(Succeed())
			Expect(AppendUpgradePostRenderer(add)(upgrade)).To(Succeed())
			out, err := upgrade.PostRenderer.Run(&bytes.Buffer{})
			Expect(err).To(BeNil())
			Expect(out.String()).To(Equal("base\nadd\nadd\n"))
			Expect(upgrade.PostRenderer).To(BeAssignableToTypeOf(chainedPostRenderer{}))
			Expect(upgrade.PostRenderer).To(HaveLen(3))
		})
	})
})

var _ = Describe("ownerPostRenderer", func() {
	var (
		pr    ownerPostRenderer
		owner client.Object
	)

	BeforeEach(func() {
		rm, err := apiutil.NewDynamicRESTMapper(cfg)
		Expect(err).To(BeNil())

		owner = newTestDeployment([]corev1.Container{{
			Name: "test",
		}})
		pr = ownerPostRenderer{
			owner:      owner,
			rm:         rm,
			kubeClient: kube.New(newRESTClientGetter(cfg, rm, owner.GetNamespace())),
		}
	})

	It("injects an owner reference", func() {
		buf, err := pr.Run(bytes.NewBufferString(getTestManifest()))
		Expect(err).To(BeNil())
		objs := manifestToObjects(buf.String())
		for _, obj := range objs {
			Expect(obj.GetOwnerReferences()).To(HaveLen(1))
		}
	})

	It("fails on invalid input", func() {
		_, err := pr.Run(bytes.NewBufferString("test"))
		Expect(err).NotTo(BeNil())
	})
})

func getTestManifest() string {
	testChart := testutil.MustLoadChart("../../pkg/internal/testdata/test-chart-1.2.0.tgz")
	i := action.NewInstall(&action.Configuration{})
	i.DryRun = true
	i.Replace = true
	i.ReleaseName = "release-name"
	i.ClientOnly = true
	rel, err := i.Run(&testChart, nil)
	Expect(err).To(BeNil())
	return rel.Manifest
}
