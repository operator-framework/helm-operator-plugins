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

package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ghodss/yaml"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"helm.sh/helm/v3/pkg/kube"

	"helm.sh/helm/v3/pkg/postrender"

	"gomodules.xyz/jsonpatch/v2"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/resource"
)

type ActionClientGetter interface {
	ActionClientFor(obj Object) (ActionInterface, error)
}

type ActionInterface interface {
	Get(name string, opts ...GetOption) (*release.Release, error)
	Install(name, namespace string, chrt *chart.Chart, vals map[string]interface{}, opts ...InstallOption) (*release.Release, error)
	Upgrade(name, namespace string, chrt *chart.Chart, vals map[string]interface{}, opts ...UpgradeOption) (*release.Release, error)
	Uninstall(name string, opts ...UninstallOption) (*release.UninstallReleaseResponse, error)
	Reconcile(rel *release.Release) error
}

type GetOption func(*action.Get) error
type InstallOption func(*action.Install) error
type UpgradeOption func(*action.Upgrade) error
type UninstallOption func(*action.Uninstall) error

func NewActionClientGetter(acg ActionConfigGetter) ActionClientGetter {
	return &actionClientGetter{acg}
}

type actionClientGetter struct {
	acg ActionConfigGetter
}

func (hcg *actionClientGetter) ActionClientFor(obj Object) (ActionInterface, error) {
	actionConfig, err := hcg.acg.ActionConfigFor(obj)
	if err != nil {
		return nil, err
	}
	postRenderer := createPostRenderer(actionConfig.KubeClient, obj)
	return &actionClient{actionConfig, postRenderer}, nil
}

type actionClient struct {
	conf         *action.Configuration
	postRenderer postrender.PostRenderer
}

func (c *actionClient) Get(name string, opts ...GetOption) (*release.Release, error) {
	get := action.NewGet(c.conf)
	for _, o := range opts {
		if err := o(get); err != nil {
			return nil, err
		}
	}
	return get.Run(name)
}

func (c *actionClient) Install(name, namespace string, chrt *chart.Chart, vals map[string]interface{}, opts ...InstallOption) (*release.Release, error) {
	install := action.NewInstall(c.conf)
	install.PostRenderer = c.postRenderer
	for _, o := range opts {
		if err := o(install); err != nil {
			return nil, err
		}
	}
	install.ReleaseName = name
	install.Namespace = namespace
	c.conf.Log("Starting install")
	rel, err := install.Run(chrt, vals)
	if err != nil {
		c.conf.Log("Install failed")
		if rel != nil {
			// Uninstall the failed release installation so that we can retry
			// the installation again during the next reconciliation. In many
			// cases, the issue is unresolvable without a change to the CR, but
			// controller-runtime will backoff on retries after failed attempts.
			//
			// In certain cases, Install will return a partial release in
			// the response even when it doesn't record the release in its release
			// store (e.g. when there is an error rendering the release manifest).
			// In that case the rollback will fail with a not found error because
			// there was nothing to rollback.
			//
			// Only return an error about a rollback failure if the failure was
			// caused by something other than the release not being found.
			_, uninstallErr := c.Uninstall(name)
			if !errors.Is(uninstallErr, driver.ErrReleaseNotFound) {
				return nil, fmt.Errorf("uninstall failed: %v: original install error: %w", uninstallErr, err)
			}
		}
		return nil, err
	}
	return rel, nil
}

func (c *actionClient) Upgrade(name, namespace string, chrt *chart.Chart, vals map[string]interface{}, opts ...UpgradeOption) (*release.Release, error) {
	upgrade := action.NewUpgrade(c.conf)
	upgrade.PostRenderer = c.postRenderer
	for _, o := range opts {
		if err := o(upgrade); err != nil {
			return nil, err
		}
	}
	upgrade.Namespace = namespace
	rel, err := upgrade.Run(name, chrt, vals)
	if err != nil {
		if rel != nil {
			rollback := action.NewRollback(c.conf)
			rollback.Force = true

			// As of Helm 2.13, if Upgrade returns a non-nil release, that
			// means the release was also recorded in the release store.
			// Therefore, we should perform the rollback when we have a non-nil
			// release. Any rollback error here would be unexpected, so always
			// log both the update and rollback errors.
			rollbackErr := rollback.Run(name)
			if rollbackErr != nil {
				return nil, fmt.Errorf("rollback failed: %v: original upgrade error: %w", rollbackErr, err)
			}
		}
		return nil, err
	}
	return rel, nil
}

func (c *actionClient) Uninstall(name string, opts ...UninstallOption) (*release.UninstallReleaseResponse, error) {
	uninstall := action.NewUninstall(c.conf)
	for _, o := range opts {
		if err := o(uninstall); err != nil {
			return nil, err
		}
	}
	return uninstall.Run(name)
}

func (c *actionClient) Reconcile(rel *release.Release) error {
	infos, err := c.conf.KubeClient.Build(bytes.NewBufferString(rel.Manifest), false)
	if err != nil {
		return err
	}
	return infos.Visit(func(expected *resource.Info, err error) error {
		if err != nil {
			return err
		}

		helper := resource.NewHelper(expected.Client, expected.Mapping)

		existing, err := helper.Get(expected.Namespace, expected.Name, false)
		if apierrors.IsNotFound(err) {
			if _, err := helper.Create(expected.Namespace, true, expected.Object, &metav1.CreateOptions{}); err != nil {
				return fmt.Errorf("create error: %w", err)
			}
			return nil
		} else if err != nil {
			return err
		}

		patch, err := generatePatch(existing, expected.Object)
		if err != nil {
			return fmt.Errorf("failed to marshal JSON patch: %w", err)
		}

		if patch == nil {
			return nil
		}

		_, err = helper.Patch(expected.Namespace, expected.Name, apitypes.JSONPatchType, patch, &metav1.PatchOptions{})
		if err != nil {
			return fmt.Errorf("patch error: %w", err)
		}
		return nil
	})
}

func generatePatch(existing, expected runtime.Object) ([]byte, error) {
	existingJSON, err := json.Marshal(existing)
	if err != nil {
		return nil, err
	}
	expectedJSON, err := json.Marshal(expected)
	if err != nil {
		return nil, err
	}

	ops, err := jsonpatch.CreatePatch(existingJSON, expectedJSON)
	if err != nil {
		return nil, err
	}

	// We ignore the "remove" operations from the full patch because they are
	// fields added by Kubernetes or by the user after the existing release
	// resource has been applied. The goal for this patch is to make sure that
	// the fields managed by the Helm chart are applied.
	patchOps := make([]jsonpatch.JsonPatchOperation, 0)
	for _, op := range ops {
		if op.Operation != "remove" {
			patchOps = append(patchOps, op)
		}
	}

	// If there are no patch operations, return nil. Callers are expected
	// to check for a nil response and skip the patch operation to avoid
	// unnecessary chatter with the API server.
	if len(patchOps) == 0 {
		return nil, nil
	}

	return json.Marshal(patchOps)
}

func createPostRenderer(kubeClient kube.Interface, obj Object) postrender.PostRenderer {
	ownerRef := metav1.NewControllerRef(obj, obj.GetObjectKind().GroupVersionKind())
	return &ownerRefPostRenderer{kubeClient, ownerRef}
}

type ownerRefPostRenderer struct {
	kubeClient kube.Interface
	ownerRef   *metav1.OwnerReference
}

func (pr *ownerRefPostRenderer) Run(in *bytes.Buffer) (*bytes.Buffer, error) {
	resourceList, err := pr.kubeClient.Build(in, false)
	if err != nil {
		return nil, err
	}
	out := bytes.Buffer{}

	err = resourceList.Visit(func(r *resource.Info, err error) error {
		if err != nil {
			return err
		}
		objMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(r.Object)
		if err != nil {
			return err
		}
		u := &unstructured.Unstructured{Object: objMap}
		if r.ResourceMapping().Scope == meta.RESTScopeNamespace {
			ownerRefs := append(u.GetOwnerReferences(), *pr.ownerRef)
			u.SetOwnerReferences(ownerRefs)
		}
		outData, err := yaml.Marshal(u.Object)
		if err != nil {
			return err
		}
		if _, err := out.Write(outData); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}
