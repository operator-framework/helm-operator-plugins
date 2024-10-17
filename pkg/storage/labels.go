package storage

import (
	"strconv"

	"helm.sh/helm/v3/pkg/release"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
)

func newIndexLabelsAndAnnotations(owner, key string, rls *release.Release) (map[string]string, map[string]string) {
	labels := map[string]string{}
	annotations := map[string]string{}
	for k, v := range rls.Labels {
		if promoteToAnnotation(k, v) {
			annotations[k] = v
		} else {
			labels[k] = v
		}
	}

	labels["name"] = rls.Name
	labels["owner"] = owner
	labels["status"] = rls.Info.Status.String()
	labels["version"] = strconv.Itoa(rls.Version)
	labels["key"] = key
	labels["type"] = "index"
	return labels, annotations
}

func promoteToAnnotation(k, v string) bool {
	isValidLabel := len(validation.IsQualifiedName(k)) == 0 && len(validation.IsValidLabelValue(v)) == 0
	return !isValidLabel
}

func newChunkLabels(owner, key string) map[string]string {
	labels := map[string]string{}
	labels["owner"] = owner
	labels["key"] = key
	labels["type"] = "chunk"
	return labels
}

func newListIndicesLabelSelector(owner string) labels.Selector {
	return labels.Set{"owner": owner, "type": "index"}.AsSelector()
}

func newListAllForKeySelector(owner, key string) labels.Selector {
	return labels.Set{"owner": owner, "key": key}.AsSelector()
}

func newListChunksForKeySelector(owner, key string) labels.Selector {
	return labels.Set{"owner": owner, "key": key, "type": "chunk"}.AsSelector()
}

var systemLabels = sets.New[string]("name", "owner", "status", "version", "key", "type", "createdAt", "modifiedAt")

// Checks if label is system
func isSystemLabel(key string) bool {
	return systemLabels.Has(key)
}

// Removes system labels from labels map
func filterSystemLabels(lbs map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range lbs {
		if !isSystemLabel(k) {
			result[k] = v
		}
	}
	return result
}
