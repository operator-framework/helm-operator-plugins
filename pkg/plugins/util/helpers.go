package util

import (
	"sigs.k8s.io/kubebuilder/v4/pkg/config"
)

// HasDifferentCRDVersion returns true if any other CRD version is tracked in the project configuration.
func HasDifferentCRDVersion(config config.Config, crdVersion string) bool {
	return hasDifferentAPIVersion(config.ListCRDVersions(), crdVersion)
}

func hasDifferentAPIVersion(versions []string, version string) bool {
	return !(len(versions) == 0 || (len(versions) == 1 && versions[0] == version))
}
