package cmd

import (
	"context"

	"github.com/operator-framework/helm-operator-plugins/internal/cmd/helm-operator/run"
	"github.com/operator-framework/helm-operator-plugins/internal/version"

	"github.com/spf13/cobra"
)

func rootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "helm-operator-plugins",
		Short:   "Helm Operator Plugins cli tool",
		Long:    "A utility that enables the ability to run a helm-based operator",
		Version: version.Version.String(),
		Args:    cobra.MinimumNArgs(1),
	}

	rootCmd.AddCommand(run.NewCmd())

	return rootCmd
}

func Execute() error {
	return rootCmd().ExecuteContext(context.Background())
}
