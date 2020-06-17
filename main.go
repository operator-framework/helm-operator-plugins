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
	"runtime"
	"time"

	"github.com/spf13/pflag"
	"go.uber.org/zap" // todo(camilamacedo86): why we are adding this lib? Shuld we not keep it == kube?
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	zapl "sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/joelanford/helm-operator/pkg/annotation"
	"github.com/joelanford/helm-operator/pkg/manager"
	"github.com/joelanford/helm-operator/pkg/reconciler"
	"github.com/joelanford/helm-operator/pkg/watches"
	"github.com/joelanford/helm-operator/version"
)

var (
	setupLog = ctrl.Log.WithName("setup")
)

func printVersion() {
	setupLog.Info("version information",
		"go", runtime.Version(),
		"GOOS", runtime.GOOS,
		"GOARCH", runtime.GOARCH,
		"helm-operator", version.Version) // Should not we inform the helm plugin version here? So, should it not be part of SDK impl and no lib?
}

// The implementation here is equivalent what we have in the
// /operator-sdk/pkg/helm/run.go internal/scaffold/helm/main.go and helm flags
func main() {
	var (
		metricsAddr             string
		enableLeaderElection    bool
		leaderElectionID        string
		leaderElectionNamespace string

		watchesFile                    string
		defaultMaxConcurrentReconciles int
		defaultReconcilePeriod         time.Duration

		// Deprecated: use defaultMaxConcurrentReconciles
		defaultMaxWorkers int
	)
	//todo(camilamacedo86): why the port is not 8080?
	pflag.StringVar(&metricsAddr, "metrics-addr", "0.0.0.0:8383", "The address the metric endpoint binds to.")
	pflag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")

	//todo(camilamacedo86): the following ones has not in the kube. Wy we need the leaderElectionID and leaderElectionNamespace?
	pflag.StringVar(&leaderElectionID, "leader-election-id", "",
		"Name of the configmap that is used for holding the leader lock.")
	pflag.StringVar(&leaderElectionNamespace, "leader-election-namespace", "",
		"Namespace in which to create the leader election configmap for holding the leader lock (required if running locally).")

	pflag.StringVar(&watchesFile, "watches-file", "./watches.yaml", "Path to watches.yaml file.")
	pflag.DurationVar(&defaultReconcilePeriod, "reconcile-period", 0, "Default reconcile period for controllers (use 0 to disable periodic reconciliation)")
	pflag.IntVar(&defaultMaxConcurrentReconciles, "max-concurrent-reconciles", 1, "Default maximum number of concurrent reconciles for controllers.")

	// Deprecated: --max-workers flag does not align well with the name of the option it configures on the controller
	//   (MaxConcurrentReconciles). Flag `--max-concurrent-reconciles` should be used instead.
	pflag.IntVar(&defaultMaxWorkers, "max-workers", 1, "Default maximum number of concurrent reconciles for controllers.")
	if err := pflag.CommandLine.MarkHidden("max-workers"); err != nil {
		setupLog.Error(err, "failed to hide --max-workers flag")
		os.Exit(1)
	}

	klog.InitFlags(flag.CommandLine)
	pflag.Parse()

	logLvl := zap.NewAtomicLevelAt(zap.InfoLevel)
	sttLvl := zap.NewAtomicLevelAt(zap.PanicLevel)
	ctrl.SetLogger(zapl.New(
		zapl.UseDevMode(false),
		zapl.Level(&logLvl),
		zapl.StacktraceLevel(&sttLvl),
	))

	printVersion()
	// todo: Should we not to do this deprecations now in SDK (ASAP) and then may we can just remove from here?
	// Deprecated: --max-workers flag does not align well with the name of the option it configures on the controller
	//   (MaxConcurrentReconciles). Flag `--max-concurrent-reconciles` should be used instead.
	if pflag.Lookup("max-workers").Changed {
		setupLog.Info("flag --max-workers has been deprecated, use --max-concurrent-reconciles instead")
		if pflag.Lookup("max-concurrent-reconciles").Changed {
			setupLog.Info("ignoring --max-workers since --max-concurrent-reconciles is set")
		} else {
			defaultMaxConcurrentReconciles = defaultMaxWorkers
		}
	}
	// Why we need a flag for `--leader-election-id`? What is the user case that would be required pass this value?
	// Deprecated: OPERATOR_NAME environment variable is an artifact of the legacy operator-sdk project scaffolding.
	//   Flag `--leader-election-id` should be used instead.
	if operatorName, found := os.LookupEnv("OPERATOR_NAME"); found {
		setupLog.Info("environment variable OPERATOR_NAME has been deprecated, use --leader-election-id instead.")
		if pflag.Lookup("leader-election-id").Changed {
			setupLog.Info("ignoring OPERATOR_NAME environment variable since --leader-election-id is set")
		} else {
			leaderElectionID = operatorName
		}
	}
	options := ctrl.Options{
		MetricsBindAddress: metricsAddr,
		LeaderElection:     enableLeaderElection,
		// todo: add:
		Port:                    9443, // should not it be == kubebuilder?
		LeaderElectionID:        leaderElectionID,
		LeaderElectionNamespace: leaderElectionNamespace, // todo: why we need the ns?
		NewClient:               manager.NewDelegatingClientFunc(),
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
	// Not for now. Just a thought for the future: Could we not work with markers to inject
	// the code based on the helm/ansible charts instead which would allow users
	// also add Go code. Could not it be a solution for the hibrid ones?
	for _, w := range ws {
		reconcilePeriod := defaultReconcilePeriod
		if w.ReconcilePeriod != nil {
			reconcilePeriod = *w.ReconcilePeriod
		}

		maxConcurrentReconciles := defaultMaxConcurrentReconciles
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
			reconciler.WithInstallAnnotations(annotation.DefaultInstallAnnotations...),
			reconciler.WithUpgradeAnnotations(annotation.DefaultUpgradeAnnotations...),
			reconciler.WithUninstallAnnotations(annotation.DefaultUninstallAnnotations...),
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

	// TODO(joelanford): kube-state-metrics?

	// Are we sure that the "https://godoc.org/sigs.k8s.io/controller-runtime/pkg/manager#LeaderElectionRunnable"
	// is able to do all that is done with:
	// Become the leader before proceeding
	//	err = leader.Become(ctx, operatorName+"-lock")
	//	if err != nil {
	//		log.Error(err, "Failed to become leader.")
	//		return err
	//	}
	// Should we check with we would need to keep the leader.Become or try to push something to upstream?
	// Why we still using it in the legacy projects?

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
