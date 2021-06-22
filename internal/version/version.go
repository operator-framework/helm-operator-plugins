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

import (
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/blang/semver/v4"
)

const (
	Unknown    = "unknown"
	modulePath = "github.com/operator-framework/helm-operator-plugins"
)

var (
	GitVersion      = Unknown
	GitCommit       = Unknown
	ScaffoldVersion = Unknown
)

func init() {
	// If the ScaffoldVersion was not set during the
	// build (e.g. if this module was imported by
	// another), try to deduce the scaffold version
	// from the binary's embedded build info.
	if ScaffoldVersion == Unknown {
		ScaffoldVersion = getScaffoldVersion()
	}
}

// getScaffoldVersion parses build info embedded in
// the binary to deduce a tag version that is
// appropriate to use when scaffolding files that
// need to reference the most recent release from
// this repository.
//
// This allows other projects to import and use the
// helm plugin and have versions populated correctly
// in the scaffolded Makefile, Dockerfile, etc.
func getScaffoldVersion() string {
	info, ok := debug.ReadBuildInfo()
	if ok {
		// Search the dependencies of the main module and
		// if we find this module, return its most recent
		// tag.
		for _, m := range info.Deps {
			if m == nil {
				continue
			}
			if m.Path == modulePath {
				return getMostRecentTag(*m)
			}
		}
	}

	// If there isn't build info or we couldn't
	// find our module in the main module's deps,
	// return Unknown.
	return Unknown
}

// getMostRecentTag translates m into a version string
// that corresponds to the tag at (or that precedes) the
// commit referenced by the m's version, accounting
// for any replacements.
//
// If it can't deduce a tag, it returns "unknown".
func getMostRecentTag(m debug.Module) string {
	// Unwind all of the replacements.
	for m.Replace != nil {
		m = *m.Replace
	}

	// We need to handle the possibility of a pseudo-version.
	// See: https://golang.org/cmd/go/#hdr-Pseudo_versions
	//
	// We'll get the first segment and attempt to parse it as
	// semver.
	split := strings.Split(m.Version, "-")
	sv, err := semver.Parse(strings.TrimPrefix(split[0], "v"))

	// If the first segment was not a valid semver string,
	// return Unknown.
	if err != nil {
		return Unknown
	}

	// If there were multiple segments, m.Version is
	// a pseudo-version, in which case Go will have
	// incremented the patch version. If the patch
	// version is greater than zero (it should always
	// be), we'll decrement it to get back to the
	// previous tag.
	//
	// This is necessary to handle projects that
	// import this project at untagged commits.
	if len(split) > 1 && sv.Patch > 0 {
		sv.Patch -= 1
	}
	return fmt.Sprintf("v%s", sv.FinalizeVersion())
}
