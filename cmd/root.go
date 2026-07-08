package cmd

import (
	"context"
	"os"
	"os/signal"

	"github.com/spf13/cobra"
)

var RootCmd = &cobra.Command{
	Use:   "cupel",
	Short: "cupel compares Helm release revisions.",
	Long:  "cupel compares Helm release revisions.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		cmd.SilenceUsage = true
	},
}

func Execute() {
	lifetimeCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := RootCmd.ExecuteContext(lifetimeCtx); err != nil {
		os.Exit(1)
	}
}
