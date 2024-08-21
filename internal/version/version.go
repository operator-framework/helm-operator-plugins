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

package version

import "fmt"

const (
	Unknown    = "unknown"
	modulePath = "github.com/operator-framework/helm-operator-plugins"
)

var (
	GitVersion = Unknown
	GitCommit  = Unknown
)

var Version = Context{
	Name:    modulePath,
	Version: GitVersion,
	Commit:  GitCommit,
}

type Context struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
}

func (vc *Context) String() string {
	return fmt.Sprintf("%s <commit: %s>", vc.Version, vc.Commit)
}
