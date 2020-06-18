/*
Copyright 2018 The Kubernetes Authors.

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

package metricsauth

import (
	"fmt"
	"hash/fnv"
	"path/filepath"
	"text/template"

	"sigs.k8s.io/kubebuilder/pkg/model/file"
)

var _ file.Template = &AuthProxyPatch{}
var _ file.UseCustomFuncMap = &AuthProxyPatch{}

// AuthProxyPatch scaffolds the patch file for enabling
// prometheus metrics for manager Pod.
type AuthProxyPatch struct {
	file.TemplateMixin
	file.DomainMixin
	file.RepositoryMixin
}

// SetTemplateDefaults implements input.Template
func (f *AuthProxyPatch) SetTemplateDefaults() error {
	if f.Path == "" {
		f.Path = filepath.Join("config", "default", "manager_auth_proxy_patch.yaml")
	}

	f.TemplateBody = kustomizeAuthProxyPatchTemplate

	f.IfExistsAction = file.Error

	return nil
}

func hash(s string) (string, error) {
	hasher := fnv.New32a()
	hasher.Write([]byte(s)) // nolint:errcheck
	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

// GetFuncMap implements file.UseCustomFuncMap
func (f *AuthProxyPatch) GetFuncMap() template.FuncMap {
	fm := file.DefaultFuncMap()
	fm["hash"] = hash
	return fm
}

const kustomizeAuthProxyPatchTemplate = `# This patch inject a sidecar container which is a HTTP proxy for the 
# controller manager, it performs RBAC authorization against the Kubernetes API using SubjectAccessReviews.
apiVersion: apps/v1
kind: Deployment
metadata:
  name: controller-manager
  namespace: system
spec:
  template:
    spec:
      containers:
      - name: kube-rbac-proxy
        image: gcr.io/kubebuilder/kube-rbac-proxy:v0.5.0
        args:
        - "--secure-listen-address=0.0.0.0:8443"
        - "--upstream=http://127.0.0.1:8080/"
        - "--logtostderr=true"
        - "--v=10"
        ports:
        - containerPort: 8443
          name: https
      - name: manager
        args:
        - "--metrics-addr=127.0.0.1:8080"
        - "--enable-leader-election"
        - --leader-election-id={{ hash .Repo }}.{{ .Domain }}
`
