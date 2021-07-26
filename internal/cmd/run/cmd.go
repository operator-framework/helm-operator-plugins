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

package run

import (
	"flag"
	"os"
	"runtime"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	zapl "sigs.k8s.io/controller-runtime/pkg/log/zap"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/operator-framework/helm-operator-plugins/internal/metrics"
	"github.com/operator-framework/helm-operator-plugins/internal/version"
	"github.com/operator-framework/helm-operator-plugins/pkg/annotation"
	"github.com/operator-framework/helm-operator-plugins/pkg/manager"
	"github.com/operator-framework/helm-operator-plugins/pkg/reconciler"
	"github.com/operator-framework/helm-operator-plugins/pkg/watches"
)

func NewCmd() *cobra.Command {
	r := run{}
	zapfs := flag.NewFlagSet("zap", flag.ExitOnError)
	opts := &zapl.Options{}
	opts.BindFlags(zapfs)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the helm operator controller",
		Run: func(cmd *cobra.Command, _ []string) {
			logf.SetLogger(zapl.New(zapl.UseFlagOptions(opts)))
			r.run(cmd)
		},
	}
	r.bindFlags(cmd.Flags())
	cmd.Flags().AddGoFlagSet(zapfs)
	return cmd
}

type run struct {
	metricsAddr             string
	probeAddr               string
	enableLeaderElection    bool
	leaderElectionID        string
	leaderElectionNamespace string

	watchesFile                    string
	defaultMaxConcurrentReconciles int
	defaultReconcilePeriod         time.Duration
}

func (r *run) bindFlags(fs *pflag.FlagSet) {
	fs.StringVar(&r.metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	fs.StringVar(&r.probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	fs.BoolVar(&r.enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	fs.StringVar(&r.leaderElectionID, "leader-election-id", "",
		"Name of the configmap that is used for holding the leader lock.")
	fs.StringVar(&r.leaderElectionNamespace, "leader-election-namespace", "",
		"Namespace in which to create the leader election configmap for holding the leader lock (required if running locally with leader election enabled).")

	fs.StringVar(&r.watchesFile, "watches-file", "./watches.yaml", "Path to watches.yaml file.")
	fs.DurationVar(&r.defaultReconcilePeriod, "reconcile-period", time.Minute, "Default reconcile period for controllers (use 0 to disable periodic reconciliation)")
	fs.IntVar(&r.defaultMaxConcurrentReconciles, "max-concurrent-reconciles", runtime.NumCPU(), "Default maximum number of concurrent reconciles for controllers.")
}

var log = logf.Log.WithName("cmd")

func printVersion() {
	log.Info("Version",
		"Go Version", runtime.Version(),
		"GOOS", runtime.GOOS,
		"GOARCH", runtime.GOARCH,
		"helm-operator", version.GitVersion)
}

func (r *run) run(cmd *cobra.Command) {
	printVersion()

	metrics.RegisterBuildInfo(crmetrics.Registry)

	// Deprecated: OPERATOR_NAME environment variable is an artifact of the legacy operator-sdk project scaffolding.
	//   Flag `--leader-election-id` should be used instead.
	if operatorName, found := os.LookupEnv("OPERATOR_NAME"); found {
		log.Info("environment variable OPERATOR_NAME has been deprecated, use --leader-election-id instead.")
		if cmd.Flags().Lookup("leader-election-id").Changed {
			log.Info("ignoring OPERATOR_NAME environment variable since --leader-election-id is set")
		} else {
			r.leaderElectionID = operatorName
		}
	}

	options := ctrl.Options{
		MetricsBindAddress:         r.metricsAddr,
		HealthProbeBindAddress:     r.probeAddr,
		LeaderElection:             r.enableLeaderElection,
		LeaderElectionID:           r.leaderElectionID,
		LeaderElectionNamespace:    r.leaderElectionNamespace,
		LeaderElectionResourceLock: resourcelock.ConfigMapsResourceLock,
		NewClient: func(cache cache.Cache, config *rest.Config, options client.Options, uncachedObjects ...client.Object) (client.Client, error) {
			// Create the client for Write operation
			c, err := client.New(config, options)
			if err != nil {
				return nil, err
			}
			return client.NewDelegatingClient(client.NewDelegatingClientInput{
				CacheReader:       cache,
				Client:            c,
				UncachedObjects:   uncachedObjects,
				CacheUnstructured: true,
			})
		},
	}
	manager.ConfigureWatchNamespaces(&options, log)
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), options)
	if err != nil {
		log.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Error(err, "unable to setup health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Error(err, "unable to setup readiness check")
		os.Exit(1)
	}

	ws, err := watches.Load(r.watchesFile)
	if err != nil {
		log.Error(err, "unable to load watches.yaml", "path", r.watchesFile)
		os.Exit(1)
	}

	for _, w := range ws {
		reconcilePeriod := r.defaultReconcilePeriod
		if w.ReconcilePeriod != nil {
			reconcilePeriod = w.ReconcilePeriod.Duration
		}

		maxConcurrentReconciles := r.defaultMaxConcurrentReconciles
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
			log.Error(err, "unable to create helm reconciler", "controller", "Helm")
			os.Exit(1)
		}

		if err := r.SetupWithManager(mgr); err != nil {
			log.Error(err, "unable to create controller", "controller", "Helm")
			os.Exit(1)
		}
		log.Info("configured watch", "gvk", w.GroupVersionKind, "chartPath", w.ChartPath, "maxConcurrentReconciles", maxConcurrentReconciles, "reconcilePeriod", reconcilePeriod)
	}

	log.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error(err, "problem running manager")
		os.Exit(1)
	}
}
