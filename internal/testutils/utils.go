// Copyright 2021 The Operator-SDK Authors
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

package testutils

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	kbtestutils "sigs.k8s.io/kubebuilder/v3/test/e2e/utils"
)

// TestContext wraps kubebuilder's e2e TestContext.
// Note: Though the testcontext has options to enable OLM and prometheus while starting test suite, we currently aren't
// using it for hybrid helm plugin.
type TestContext struct {
	*kbtestutils.TestContext
	// BundleImageName store the image to use to build the bundle
	BundleImageName string
	// ProjectName store the project name
	ProjectName string
	// isPrometheusManagedBySuite is true when the suite tests is installing/uninstalling the Prometheus
	isPrometheusManagedBySuite bool
	// isOLMManagedBySuite is true when the suite tests is installing/uninstalling the OLM
	isOLMManagedBySuite bool
}

// NewTestContext returns a TestContext containing a new kubebuilder TestContext.
// Construct if your environment is connected to a live cluster, ex. for e2e tests.
func NewTestContext(binaryName string, env ...string) (tc TestContext, err error) {
	if tc.TestContext, err = kbtestutils.NewTestContext(binaryName, env...); err != nil {
		return tc, err
	}
	tc.ProjectName = strings.ToLower(filepath.Base(tc.Dir))
	tc.ImageName = makeImageName(tc.ProjectName)
	tc.BundleImageName = makeBundleImageName(tc.ProjectName)
	tc.isOLMManagedBySuite = true
	tc.isPrometheusManagedBySuite = true
	return tc, nil
}

// NewPartialTestContext returns a TestContext containing a partial kubebuilder TestContext.
// This object needs to be populated with GVK information. The underlying TestContext is
// created directly rather than through a constructor so cluster-based setup is skipped.
func NewPartialTestContext(binaryName, dir string, env ...string) (tc TestContext, err error) {
	cc := &kbtestutils.CmdContext{
		Env: env,
	}
	if cc.Dir, err = filepath.Abs(dir); err != nil {
		return tc, err
	}
	projectName := strings.ToLower(filepath.Base(dir))

	return TestContext{
		TestContext: &kbtestutils.TestContext{
			CmdContext: cc,
			BinaryName: binaryName,
			ImageName:  makeImageName(projectName),
		},
		ProjectName:     projectName,
		BundleImageName: makeBundleImageName(projectName),
	}, nil
}

func makeImageName(projectName string) string {
	return fmt.Sprintf("quay.io/example/%s:v0.0.1", projectName)
}

func makeBundleImageName(projectName string) string {
	return fmt.Sprintf("quay.io/example/%s-bundle:v0.0.1", projectName)
}

// ReplaceInFile replaces all instances of old with new in the file at path.
// todo(camilamacedo86): this func can be pushed to upstream/kb
func ReplaceInFile(path, old, new string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	if !strings.Contains(string(b), old) {
		return errors.New("unable to find the content to be replaced")
	}
	s := strings.Replace(string(b), old, new, -1)
	err = ioutil.WriteFile(path, []byte(s), info.Mode())
	if err != nil {
		return err
	}
	return nil
}

// ReplaceRegexInFile finds all strings that match `match` and replaces them
// with `replace` in the file at path.
// todo(camilamacedo86): this func can be pushed to upstream/kb
func ReplaceRegexInFile(path, match, replace string) error {
	matcher, err := regexp.Compile(match)
	if err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	s := matcher.ReplaceAllString(string(b), replace)
	if s == string(b) {
		return errors.New("unable to find the content to be replaced")
	}
	err = ioutil.WriteFile(path, []byte(s), info.Mode())
	if err != nil {
		return err
	}
	return nil
}

// UncommentCode searches for target in the file and remove the comment prefix
// of the target content. The target content may span multiple lines.
// todo(camilamacedo86): this func exists in upstream/kb but there the error is not thrown. We need to
// push this change. See: https://github.com/kubernetes-sigs/kubebuilder/blob/master/test/e2e/utils/util.go
func UncommentCode(filename, target, prefix string) error {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	strContent := string(content)

	idx := strings.Index(strContent, target)
	if idx < 0 {
		// todo: push this check to upstream for we do not need have this func here
		return fmt.Errorf("unable to find the code %s to be uncomment", target)
	}

	out := new(bytes.Buffer)
	_, err = out.Write(content[:idx])
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(bytes.NewBufferString(target))
	if !scanner.Scan() {
		return nil
	}
	for {
		_, err := out.WriteString(strings.TrimPrefix(scanner.Text(), prefix))
		if err != nil {
			return err
		}
		// Avoid writing a newline in case the previous line was the last in target.
		if !scanner.Scan() {
			break
		}
		if _, err := out.WriteString("\n"); err != nil {
			return err
		}
	}

	_, err = out.Write(content[idx+len(target):])
	if err != nil {
		return err
	}
	// false positive
	// nolint:gosec
	return ioutil.WriteFile(filename, out.Bytes(), 0644)
}
