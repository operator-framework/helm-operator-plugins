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
	"time"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	zapl "sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/joelanford/helm-operator/pkg/annotation"
	"github.com/joelanford/helm-operator/pkg/manager"
	"github.com/joelanford/helm-operator/pkg/reconciler"
	"github.com/joelanford/helm-operator/pkg/watches"
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
	var (
		metricsAddr          string
		enableLeaderElection bool
		leaderElectionID     string

		watchesFile             string
		maxConcurrentReconciles int
		reconcilePeriod         time.Duration
	)

	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&leaderElectionID, "leader-election-id", "leader-lock",
		"Name of the configmap that is used for holding the leader lock.")

	flag.StringVar(&watchesFile, "watches-file", "./watches.yaml", "Path to watches.yaml file.")
	flag.IntVar(&maxConcurrentReconciles, "max-concurrent-reconciles", 1, "Default maximum number of concurrent reconciles for controllers.")
	flag.DurationVar(&reconcilePeriod, "reconcile-period", time.Minute, "Default reconcile period for controllers.")

	klog.InitFlags(flag.CommandLine)
	flag.Parse()

	logLvl := zap.NewAtomicLevelAt(zap.InfoLevel)
	sttLvl := zap.NewAtomicLevelAt(zap.PanicLevel)
	ctrl.SetLogger(zapl.New(
		zapl.UseDevMode(false),
		zapl.Level(&logLvl),
		zapl.StacktraceLevel(&sttLvl),
	))

	options := ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   leaderElectionID,
		Port:               9443,
		NewClient:          manager.NewDelegatingClientFunc(),
	}
	manager.ConfigureWatchNamespaces(&options, setupLog)
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), options)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	ws, err := watches.Load(watchesFile)
	if err != nil {
		setupLog.Error(err, "unable to load watches.yaml", "path", watchesFile)
		os.Exit(1)
	}

	for _, w := range ws {
		if w.ReconcilePeriod != nil {
			reconcilePeriod = *w.ReconcilePeriod
		}
		if w.MaxConcurrentReconciles != nil {
			maxConcurrentReconciles = *w.MaxConcurrentReconciles
		}

		r, err := reconciler.New(
			reconciler.WithChart(*w.Chart),
			reconciler.WithGroupVersionKind(w.GroupVersionKind),
			reconciler.WithOverrideValues(w.OverrideValues),
			reconciler.SkipDependentWatches(w.WatchDependentResources != nil && !*w.WatchDependentResources),
			reconciler.WithMaxConcurrentReconciles(maxConcurrentReconciles),
			reconciler.WithReconcilePeriod(reconcilePeriod),
			reconciler.WithInstallAnnotation(&annotation.InstallDisableHooks{}),
			reconciler.WithUpgradeAnnotation(&annotation.UpgradeDisableHooks{}),
			reconciler.WithUpgradeAnnotation(&annotation.UpgradeForce{}),
			reconciler.WithUninstallAnnotation(&annotation.UninstallDisableHooks{}),
		)
		if err != nil {
			setupLog.Error(err, "unable to create helm reconciler", "controller", "Helm")
			os.Exit(1)
		}

		if err := r.SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Helm")
			os.Exit(1)
		}
		setupLog.Info("configured watch", "gvk", w.GroupVersionKind, "chartPath", w.ChartPath, "maxConcurrentReconciles", maxConcurrentReconciles, "reconcilePeriod", reconcilePeriod)
	}

	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
