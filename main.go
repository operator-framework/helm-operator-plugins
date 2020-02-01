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
	"flag"
	"os"

	"go.uber.org/zap"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	zapl "sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/operator-framework/helm-operator/pkg/reconciler"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	logLvl := zap.NewAtomicLevelAt(zap.InfoLevel)
	sttLvl := zap.NewAtomicLevelAt(zap.PanicLevel)
	ctrl.SetLogger(zapl.New(
		zapl.UseDevMode(true),
		zapl.Level(&logLvl),
		zapl.StacktraceLevel(&sttLvl),
	))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		LeaderElection:     enableLeaderElection,
		Port:               9443,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	r, err := reconciler.NewHelm(
		reconciler.WithLog(ctrl.Log.WithName("controllers").WithName("Helm")),
		reconciler.WithChartPath("./helm-charts/tomcat"),
		reconciler.WithGVK(schema.GroupVersionKind{
			Group:   "apache.sdk.operator-framework.io",
			Version: "v1",
			Kind:    "Tomcat",
		}),
	)
	if err != nil {
		setupLog.Error(err, "unable to create helm reconciler", "controller", "Helm")
		os.Exit(1)
	}

	if err := r.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Helm")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
