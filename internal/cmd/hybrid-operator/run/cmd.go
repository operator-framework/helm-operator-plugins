// Copyright 2021 The Operator-SDK Authors
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

package run

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/operator-framework/helm-operator-plugins/internal/flags"
	"github.com/operator-framework/helm-operator-plugins/internal/metrics"
	"github.com/operator-framework/helm-operator-plugins/internal/version"
	"github.com/operator-framework/helm-operator-plugins/pkg/annotation"
	helmmgr "github.com/operator-framework/helm-operator-plugins/pkg/manager"
	"github.com/operator-framework/helm-operator-plugins/pkg/reconciler"
	"github.com/operator-framework/helm-operator-plugins/pkg/watches"
	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	zapf "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var log = logf.Log.WithName("cmd")

// TODO: Print the helm-operator plugin version. Earlier this was
// tied to operator-sdk version
func printVersion() {
	log.Info("Version",
		"Go Version", runtime.Version(),
		"GOOS", runtime.GOOS,
		"GOARCH", runtime.GOARCH,
		"helm-operator", version.GitVersion,
		"commit", version.GitCommit)
}

func NewCmd() *cobra.Command {
	f := &flags.Flags{}
	zapfs := flag.NewFlagSet("zap", flag.ExitOnError)
	opts := &zapf.Options{}
	opts.BindFlags(zapfs)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the operator",
		Run: func(cmd *cobra.Command, _ []string) {
			logf.SetLogger(zapf.New(zapf.UseFlagOptions(opts)))
			run(cmd, f)
		},
	}

	f.AddTo(cmd.Flags())
	cmd.Flags().AddGoFlagSet(zapfs)
	return cmd
}

func run(cmd *cobra.Command, f *flags.Flags) {
	printVersion()
	metrics.RegisterBuildInfo(crmetrics.Registry)

	// Load config options from the config at f.ManagerConfigPath.
	// These options will not override those set by flags.
	var (
		options manager.Options
		err     error
	)

	if f.ManagerConfigPath != "" {
		cfgLoader := ctrl.ConfigFile().AtPath(f.ManagerConfigPath)
		if options, err = options.AndFrom(cfgLoader); err != nil {
			log.Error(err, "Unable to load the manager config file")
			os.Exit(1)
		}
	}
	exitIfUnsupported(options)

	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "Failed to get config.")
		os.Exit(1)
	}

	// TODO(2.0.0): remove
	// Deprecated: OPERATOR_NAME environment variable is an artifact of the
	// legacy operator-sdk project scaffolding. Flag `--leader-election-id`
	// should be used instead.
	if operatorName, found := os.LookupEnv("OPERATOR_NAME"); found {
		log.Info("Environment variable OPERATOR_NAME has been deprecated, use --leader-election-id instead.")
		if cmd.Flags().Changed("leader-election-id") {
			log.Info("Ignoring OPERATOR_NAME environment variable since --leader-election-id is set")
		} else if options.LeaderElectionID == "" {
			// Only set leader election ID using OPERATOR_NAME if unset everywhere else,
			// since this env var is deprecated.
			options.LeaderElectionID = operatorName
		}
	}

	//TODO(2.0.0): remove the following checks. they are required just because of the flags deprecation
	if cmd.Flags().Changed("leader-elect") && cmd.Flags().Changed("enable-leader-election") {
		log.Error(errors.New("only one of --leader-elect and --enable-leader-election may be set"), "invalid flags usage")
		os.Exit(1)
	}

	if cmd.Flags().Changed("metrics-addr") && cmd.Flags().Changed("metrics-bind-address") {
		log.Error(errors.New("only one of --metrics-addr and --metrics-bind-address may be set"), "invalid flags usage")
		os.Exit(1)
	}

	// Set default manager options
	options = f.ToManagerOptions(options)

	// Log manager option flags
	optionsLog := map[string]interface{}{
		"MetricsBindAddress": options.MetricsBindAddress,
		"HealthProbeAddress": options.HealthProbeBindAddress,
		"LeaderElection":     options.LeaderElection,
	}
	if options.LeaderElectionID != "" {
		optionsLog["LeaderElectionId"] = options.LeaderElectionID
	}
	if options.LeaderElectionNamespace != "" {
		optionsLog["LeaderElectionNamespace"] = options.LeaderElectionNamespace
	}
	log.Info("Setting manager options", "Options", optionsLog)

	if options.NewClient == nil {
		options.NewClient = helmmgr.NewCachingClientFunc()
	}
	helmmgr.ConfigureWatchNamespaces(&options, log)

	mgr, err := manager.New(cfg, options)
	if err != nil {
		log.Error(err, "Failed to create a new manager")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Error(err, "Unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Error(err, "Unable to set up ready check")
		os.Exit(1)
	}

	ws, err := watches.Load(f.WatchesFile)
	if err != nil {
		log.Error(err, "unable to load watches.yaml", "path", f.WatchesFile)
		os.Exit(1)
	}

	for _, w := range ws {
		reconcilePeriod := f.ReconcilePeriod
		if w.ReconcilePeriod != nil {
			reconcilePeriod = w.ReconcilePeriod.Duration
		}

		maxConcurrentReconciles := f.MaxConcurrentReconciles
		if w.MaxConcurrentReconciles != nil {
			maxConcurrentReconciles = *w.MaxConcurrentReconciles
		}

		r, err := reconciler.New(
			reconciler.WithChart(*w.Chart),
			reconciler.WithGroupVersionKind(w.GroupVersionKind),
			reconciler.WithOverrideValues(w.OverrideValues),
			reconciler.WithSelector(*w.Selector),
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
		log.Info("configured watch", "gvk", w.GroupVersionKind, "chartPath", w.ChartPath, "maxConcurrentReconciles", f.MaxConcurrentReconciles, "reconcilePeriod", f.ReconcilePeriod)
	}

	log.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error(err, "problem running manager")
		os.Exit(1)
	}
}

// exitIfUnsupported prints an error containing unsupported field names and exits
// if any of those fields are not their default values.
func exitIfUnsupported(options manager.Options) {
	var keys []string
	// The below options are webhook-specific, which is not supported by helm.
	if options.CertDir != "" {
		keys = append(keys, "certDir")
	}
	if options.Host != "" {
		keys = append(keys, "host")
	}
	if options.Port != 0 {
		keys = append(keys, "port")
	}

	if len(keys) > 0 {
		log.Error(fmt.Errorf("%s set in manager options", strings.Join(keys, ", ")), "unsupported fields")
		os.Exit(1)
	}
}
