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

package main

import (
	"fmt"
	"log"
	"runtime"

	"github.com/spf13/cobra"
	"sigs.k8s.io/kubebuilder/v2/pkg/cli"
	"sigs.k8s.io/kubebuilder/v2/pkg/model/config"

	"github.com/joelanford/helm-operator/internal/cmd/run"
	"github.com/joelanford/helm-operator/internal/version"
	pluginv1 "github.com/joelanford/helm-operator/pkg/plugins/v1"
)

func main() {
	commands := []*cobra.Command{
		run.NewCmd(),
	}
	c, err := cli.New(
		cli.WithCommandName("helm-operator"),
		cli.WithVersion(getVersion()),
		cli.WithPlugins(
			&pluginv1.Plugin{},
		),
		cli.WithDefaultProjectVersion(config.Version3Alpha),
		cli.WithDefaultPlugins(config.Version3Alpha, &pluginv1.Plugin{}),
		cli.WithExtraCommands(commands...),
	)
	if err != nil {
		log.Fatal(err)
	}

	if err := c.Run(); err != nil {
		log.Fatal(err)
	}
}

func getVersion() string {
	return fmt.Sprintf("helm-operator version: %q, commit: %q, go version: %q, GOOS: %q, GOARCH: %q\n",
		version.Version, version.GitCommit, runtime.Version(), runtime.GOOS, runtime.GOARCH)

}
