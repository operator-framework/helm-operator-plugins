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

package watches

import (
	"bytes"
	"io/ioutil"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ = Describe("LoadReader", func() {
	var (
		expectedWatches   []Watch
		data              string
		falseVal, trueVal = false, true
	)
	It("should create valid watches", func() {
		data = `---
- group: mygroup
  version: v1alpha1
  kind: MyKind
  chart: ../../pkg/internal/testdata/test-chart
  watchDependentResources: false
  selector:
    matchExpressions:
     - {key: testLabel, operator: Exists, values: []}
  overrideValues:
    key: value
`
		expectedWatches = []Watch{
			{
				GroupVersionKind:        schema.GroupVersionKind{Group: "mygroup", Version: "v1alpha1", Kind: "MyKind"},
				ChartPath:               "../../pkg/internal/testdata/test-chart",
				WatchDependentResources: &falseVal,
				OverrideValues:          map[string]string{"key": "value"},
				Selector: &v1.LabelSelector{
					MatchLabels: nil,
					MatchExpressions: []v1.LabelSelectorRequirement{{
						Key:      "testLabel",
						Operator: v1.LabelSelectorOpExists,
						Values:   []string{},
					}},
				},
			},
		}

		watchesData := bytes.NewBufferString(data)
		watches, err := LoadReader(watchesData)
		Expect(err).NotTo(HaveOccurred())
		verifyEqualWatches(expectedWatches, watches)
	})

	It("should create valid watches with override env expansion", func() {
		data = `---
- group: mygroup
  version: v1alpha1
  kind: MyKind
  chart: ../../pkg/internal/testdata/test-chart
  watchDependentResources: false
  overrideValues:
    key: $MY_VALUE
`
		expectedWatches = []Watch{
			{
				GroupVersionKind:        schema.GroupVersionKind{Group: "mygroup", Version: "v1alpha1", Kind: "MyKind"},
				ChartPath:               "../../pkg/internal/testdata/test-chart",
				WatchDependentResources: &falseVal,
				OverrideValues:          map[string]string{"key": "value"},
			},
		}

		err := os.Setenv("MY_VALUE", "value")
		Expect(err).NotTo(HaveOccurred())

		watchesData := bytes.NewBufferString(data)
		watches, err := LoadReader(watchesData)
		Expect(err).NotTo(HaveOccurred())
		verifyEqualWatches(expectedWatches, watches)
	})

	It("should create valid watches with MaxConcurrentReconciles and ReconcilePeriod", func() {
		concurrentReconciles := 2
		data = `---
- group: mygroup
  version: v1alpha1
  kind: MyKind
  chart: ../../pkg/internal/testdata/test-chart
  watchDependentResources: false
  reconcilePeriod: 1s
  maxConcurrentReconciles: 2
  overrideValues:
    key: $MY_VALUE
`
		expectedWatches = []Watch{
			{
				GroupVersionKind:        schema.GroupVersionKind{Group: "mygroup", Version: "v1alpha1", Kind: "MyKind"},
				ChartPath:               "../../pkg/internal/testdata/test-chart",
				WatchDependentResources: &falseVal,
				OverrideValues:          map[string]string{"key": "value"},
				MaxConcurrentReconciles: &concurrentReconciles,
				ReconcilePeriod:         &v1.Duration{Duration: 1000000000},
			},
		}

		err := os.Setenv("MY_VALUE", "value")
		Expect(err).NotTo(HaveOccurred())

		watchesData := bytes.NewBufferString(data)
		watches, err := LoadReader(watchesData)
		Expect(err).NotTo(HaveOccurred())
		verifyEqualWatches(expectedWatches, watches)
	})

	It("should create valid watches file with override template expansion", func() {
		data = `---
- group: mygroup
  version: v1alpha1
  kind: MyKind
  chart: ../../pkg/internal/testdata/test-chart
  watchDependentResources: false
  overrideValues:
    repo: '{{ ("$MY_IMAGE" | split ":")._0 }}'
    tag: '{{ ("$MY_IMAGE" | split ":")._1 }}'
`
		expectedWatches = []Watch{
			{
				GroupVersionKind:        schema.GroupVersionKind{Group: "mygroup", Version: "v1alpha1", Kind: "MyKind"},
				ChartPath:               "../../pkg/internal/testdata/test-chart",
				WatchDependentResources: &falseVal,
				OverrideValues: map[string]string{
					"repo": "quay.io/operator-framework/helm-operator",
					"tag":  "latest",
				},
			},
		}

		err := os.Setenv("MY_IMAGE", "quay.io/operator-framework/helm-operator:latest")
		Expect(err).NotTo(HaveOccurred())

		watchesData := bytes.NewBufferString(data)
		watches, err := LoadReader(watchesData)
		Expect(err).NotTo(HaveOccurred())
		verifyEqualWatches(expectedWatches, watches)
	})

	It("should error for invalid watches file with override expansion", func() {
		data = `---
- group: mygroup
  version: v1alpha1
  kind: MyKind
  chart: ../../pkg/internal/testdata/test-chart
  overrideValues:
    repo: '{{ ("$MY_IMAGE" | split ":")._0 }}'
    tag: '{{ ("$MY_IMAGE" | split ":")._1'
`
		err := os.Setenv("MY_IMAGE", "quay.io/operator-framework/helm-operator:latest")
		Expect(err).NotTo(HaveOccurred())

		watchesData := bytes.NewBufferString(data)
		watches, err := LoadReader(watchesData)
		Expect(err).To(HaveOccurred())
		Expect(watches).To(BeNil())
	})

	It("should not error with multiple gvk", func() {
		data = `---
- group: mygroup
  version: v1alpha1
  kind: MyFirstKind
  chart: ../../pkg/internal/testdata/test-chart
- group: mygroup
  version: v1alpha1
  kind: MySecondKind
  chart: ../../pkg/internal/testdata/test-chart
`
		expectedWatches = []Watch{
			{
				GroupVersionKind:        schema.GroupVersionKind{Group: "mygroup", Version: "v1alpha1", Kind: "MyFirstKind"},
				ChartPath:               "../../pkg/internal/testdata/test-chart",
				WatchDependentResources: &trueVal,
			},
			{
				GroupVersionKind:        schema.GroupVersionKind{Group: "mygroup", Version: "v1alpha1", Kind: "MySecondKind"},
				ChartPath:               "../../pkg/internal/testdata/test-chart",
				WatchDependentResources: &trueVal,
			},
		}
		watchesData := bytes.NewBufferString(data)
		watches, err := LoadReader(watchesData)
		Expect(err).NotTo(HaveOccurred())
		verifyEqualWatches(expectedWatches, watches)
	})

	It("should error because of duplicate gvk", func() {
		data = `---
- group: mygroup
  version: v1alpha1
  kind: MyKind
  chart: ../../pkg/internal/testdata/test-chart
- group: mygroup
  version: v1alpha1
  kind: MyKind
  chart: ../../pkg/internal/testdata/test-chart
`
		watchesData := bytes.NewBufferString(data)
		watches, err := LoadReader(watchesData)
		Expect(err).To(HaveOccurred())
		Expect(watches).To(BeNil())
	})

	It("should error because of no version", func() {
		data = `---
- group: mygroup
  kind: MyKind
  chart: ../../pkg/internal/testdata/test-chart
`
		watchesData := bytes.NewBufferString(data)
		watches, err := LoadReader(watchesData)
		Expect(err).To(HaveOccurred())
		Expect(watches).To(BeNil())
	})

	It("should error because no kind is specified", func() {
		data = `---
- group: mygroup
  version: v1alpha1
  chart: ../../pkg/internal/testdata/test-chart
`
		watchesData := bytes.NewBufferString(data)
		watches, err := LoadReader(watchesData)
		Expect(err).To(HaveOccurred())
		Expect(watches).To(BeNil())
	})

	It("shoule error when bad chart path is specified", func() {
		data = `---
- group: mygroup
  version: v1alpha1
  kind: MyKind
  chart: nonexistent/path/to/chart
`
		watchesData := bytes.NewBufferString(data)
		watches, err := LoadReader(watchesData)
		Expect(err).To(HaveOccurred())
		Expect(watches).To(BeNil())
	})

	It("should error when invalid overrides are specified", func() {
		data = `---
- group: mygroup
  version: v1alpha1
  kind: MyKind
  chart: ../../pkg/internal/testdata/test-chart
  overrideValues:
    key1:
      key2: value
`
		watchesData := bytes.NewBufferString(data)
		watches, err := LoadReader(watchesData)
		Expect(err).To(HaveOccurred())
		Expect(watches).To(BeNil())
	})

	It("should error because of invalid yaml", func() {
		data = `---
foo: bar
`
		watchesData := bytes.NewBufferString(data)
		watches, err := LoadReader(watchesData)
		Expect(err).To(HaveOccurred())
		Expect(watches).To(BeNil())
	})
})

var _ = Describe("Load", func() {
	var (
		expectedWatches []Watch
		data            string
		falseVal        = false
	)

	It("should return valid watches", func() {
		data = `---
- group: mygroup
  version: v1alpha1
  kind: MyKind
  chart: ../../pkg/internal/testdata/test-chart
  watchDependentResources: false
  overrideValues:
    key: value
`

		expectedWatches = []Watch{
			{
				GroupVersionKind:        schema.GroupVersionKind{Group: "mygroup", Version: "v1alpha1", Kind: "MyKind"},
				ChartPath:               "../../pkg/internal/testdata/test-chart",
				WatchDependentResources: &falseVal,
				OverrideValues:          map[string]string{"key": "value"},
			},
		}

		f, err := ioutil.TempFile("", "osdk-test-load")
		Expect(err).NotTo(HaveOccurred())

		defer removeFile(f)
		_, err = f.WriteString(data)
		Expect(err).NotTo(HaveOccurred())

		watches, err := Load(f.Name())
		Expect(err).NotTo(HaveOccurred())
		verifyEqualWatches(expectedWatches, watches)
	})
})

func verifyEqualWatches(expectedWatch, obtainedWatch []Watch) {
	Expect(len(expectedWatch)).To(BeEquivalentTo(len(obtainedWatch)))
	for i := range expectedWatch {
		Expect(expectedWatch[i]).NotTo(BeNil())
		Expect(obtainedWatch[i]).NotTo(BeNil())
		Expect(expectedWatch[i].GroupVersionKind).To(BeEquivalentTo(obtainedWatch[i].GroupVersionKind))
		Expect(expectedWatch[i].ChartPath).To(BeEquivalentTo(obtainedWatch[i].ChartPath))
		Expect(expectedWatch[i].WatchDependentResources).To(BeEquivalentTo(obtainedWatch[i].WatchDependentResources))
		Expect(expectedWatch[i].OverrideValues).To(BeEquivalentTo(obtainedWatch[i].OverrideValues))
		Expect(expectedWatch[i].MaxConcurrentReconciles).To(BeEquivalentTo(obtainedWatch[i].MaxConcurrentReconciles))
		Expect(expectedWatch[i].ReconcilePeriod).To(BeEquivalentTo(obtainedWatch[i].ReconcilePeriod))
		Expect(expectedWatch[i].Selector).To(BeEquivalentTo(obtainedWatch[i].Selector))
	}
}

func removeFile(f *os.File) {
	err := f.Close()
	Expect(err).NotTo(HaveOccurred())

	err = os.Remove(f.Name())
	Expect(err).NotTo(HaveOccurred())
}
