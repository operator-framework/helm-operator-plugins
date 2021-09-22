// Copyright 2020 The Operator-SDK Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hybrid

import (
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"

	"github.com/operator-framework/helm-operator-plugins/hack/generate/samples/internal/pkg"
)

// Memcached defines the Memcached Sample in Helm
type Memcached struct {
	ctx *pkg.SampleContext
}

// GenerateMemcachedSample will call all actions to create the directory and generate the sample
// The Context to run the samples are not the same in the e2e test. In this way, note that it should NOT
// be called in the e2e tests since it will call the Prepare() to set the sample context and generate the files
// in the testdata directory. The e2e tests only ought to use the Run() method with the TestContext.
func GenerateMemcachedSample(binaryPath, samplesPath string) {
	ctx, err := pkg.NewSampleContext(binaryPath, filepath.Join(samplesPath, "memcached-operator"),
		"GO111MODULE=on")
	pkg.CheckError("generating Helm memcached context", err)

	memcached := Memcached{&ctx}
	memcached.Prepare()
	memcached.Run()
}

// Prepare the Context for the Memcached Helm Sample
// Note that sample directory will be re-created and the context data for the sample
// will be set such as the domain and GVK.
func (mh *Memcached) Prepare() {
	log.Infof("destroying directory for memcached helm samples")
	mh.ctx.Destroy()

	log.Infof("creating directory")
	err := mh.ctx.Prepare()
	pkg.CheckError("creating directory", err)

	log.Infof("setting domain and GVK")
	mh.ctx.Version = "v1alpha1"
	mh.ctx.Group = "cache"
	mh.ctx.Kind = "Memcached"
}

func (mh *Memcached) Run() {
	// When we scaffold Helm based projects, it tries to use the discovery API of a Kubernetes
	// cluster to intelligently build the RBAC rules that the operator will require based on the
	// content of the helm chart.
	//
	// Here, we intentionally set KUBECONFIG to a broken value to ensure that operator-sdk will be
	// unable to reach a real cluster, and thus will generate a default RBAC rule set. This is
	// required to make Helm project generation idempotent because contributors and CI environments
	// can all have slightly different environments that can affect the content of the generated
	// role and cause sanity testing to fail.
	os.Setenv("KUBECONFIG", "broken_so_we_generate_static_default_rules")
	log.Infof("using init command and scaffolding the project")

	err := mh.ctx.Init(
		"--plugins", "hybrid/v1-alpha",
		"--repo", "github.com/example/memcached-operator",
	)

	pkg.CheckError("creating the project", err)

	// TODO: Uncomment this code after helm plugin is migrated
	// err = mh.ctx.CreateAPI(
	// 	"--plugins", "helm.sdk.operatorframework.io/v1",
	// 	"--group", mh.ctx.Group,
	// 	"--version", mh.ctx.Version,
	// 	"--kind", mh.ctx.Kind,
	// )

	// pkg.CheckError("creating helm api", err)

	err = mh.ctx.CreateAPI(
		"--plugins", "base.go.kubebuilder.io/v3",
		"--group", mh.ctx.Group,
		"--version", mh.ctx.Version,
		"--kind", mh.ctx.Kind,
		"--resource", "--controller",
	)

	pkg.CheckError("creating go api", err)
}
