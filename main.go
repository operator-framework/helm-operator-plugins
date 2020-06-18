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
	"log"
	"os"
	"runtime"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	zapl "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/kubebuilder/pkg/cli"

	"github.com/joelanford/helm-operator/pkg/annotation"
	"github.com/joelanford/helm-operator/pkg/manager"
	pluginv1 "github.com/joelanford/helm-operator/pkg/plugin/v1"
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
		"helm-operator", version.Version)
}

func main() {
	c, err := cli.New(
		cli.WithPlugins(
			&pluginv1.Plugin{},
		),
		cli.WithDefaultPlugins(
			&pluginv1.Plugin{},
		),
		cli.WithExtraCommands(
			newRunCmd(),
		),
		cli.WithCommandName("helm-operator"),
	)
	if err != nil {
		log.Fatal(err)
	}
	if err := c.Run(); err != nil {
		log.Fatal(err)
	}
}

func newRunCmd() *cobra.Command {
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

	runCmd := cobra.Command{
		Use:   "run",
		Short: "Run the helm operator controller",
	}

	runCmd.Flags().StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	runCmd.Flags().BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	runCmd.Flags().StringVar(&leaderElectionID, "leader-election-id", "",
		"Name of the configmap that is used for holding the leader lock.")
	runCmd.Flags().StringVar(&leaderElectionNamespace, "leader-election-namespace", "",
		"Namespace in which to create the leader election configmap for holding the leader lock (required if running locally).")

	runCmd.Flags().StringVar(&watchesFile, "watches-file", "./watches.yaml", "Path to watches.yaml file.")
	runCmd.Flags().DurationVar(&defaultReconcilePeriod, "reconcile-period", 0, "Default reconcile period for controllers (use 0 to disable periodic reconciliation)")
	runCmd.Flags().IntVar(&defaultMaxConcurrentReconciles, "max-concurrent-reconciles", runtime.NumCPU(), "Default maximum number of concurrent reconciles for controllers.")

	// Deprecated: --max-workers flag does not align well with the name of the option it configures on the controller
	//   (MaxConcurrentReconciles). Flag `--max-concurrent-reconciles` should be used instead.
	runCmd.Flags().IntVar(&defaultMaxWorkers, "max-workers", runtime.NumCPU(), "Default maximum number of concurrent reconciles for controllers.")
	if err := runCmd.Flags().MarkHidden("max-workers"); err != nil {
		setupLog.Error(err, "failed to hide --max-workers flag")
		os.Exit(1)
	}

	runCmd.Run = func(cmd *cobra.Command, args []string) {
		klog.InitFlags(flag.CommandLine)

		logLvl := zap.NewAtomicLevelAt(zap.InfoLevel)
		sttLvl := zap.NewAtomicLevelAt(zap.PanicLevel)
		ctrl.SetLogger(zapl.New(
			zapl.UseDevMode(false),
			zapl.Level(&logLvl),
			zapl.StacktraceLevel(&sttLvl),
		))

		printVersion()

		// Deprecated: --max-workers flag does not align well with the name of the option it configures on the controller
		//   (MaxConcurrentReconciles). Flag `--max-concurrent-reconciles` should be used instead.
		if cmd.Flags().Lookup("max-workers").Changed {
			setupLog.Info("flag --max-workers has been deprecated, use --max-concurrent-reconciles instead")
			if cmd.Flags().Lookup("max-concurrent-reconciles").Changed {
				setupLog.Info("ignoring --max-workers since --max-concurrent-reconciles is set")
			} else {
				defaultMaxConcurrentReconciles = defaultMaxWorkers
			}
		}

		// Deprecated: OPERATOR_NAME environment variable is an artifact of the legacy operator-sdk project scaffolding.
		//   Flag `--leader-election-id` should be used instead.
		if operatorName, found := os.LookupEnv("OPERATOR_NAME"); found {
			setupLog.Info("environment variable OPERATOR_NAME has been deprecated, use --leader-election-id instead.")
			if cmd.Flags().Lookup("leader-election-id").Changed {
				setupLog.Info("ignoring OPERATOR_NAME environment variable since --leader-election-id is set")
			} else {
				leaderElectionID = operatorName
			}
		}

		options := ctrl.Options{
			MetricsBindAddress:      metricsAddr,
			LeaderElection:          enableLeaderElection,
			LeaderElectionID:        leaderElectionID,
			LeaderElectionNamespace: leaderElectionNamespace,
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

		for _, w := range ws {
			reconcilePeriod := defaultReconcilePeriod
			if w.ReconcilePeriod != nil {
				reconcilePeriod = w.ReconcilePeriod.Duration
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

		setupLog.Info("starting manager")
		if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
			setupLog.Error(err, "problem running manager")
			os.Exit(1)
		}
	}
	return &runCmd
}
