package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/x7ssss/tf-unlock/pkg/detector"
	"github.com/x7ssss/tf-unlock/pkg/safety"
	"github.com/x7ssss/tf-unlock/pkg/ui"
)

var (
	inspectStaleAfter time.Duration
)

var inspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Inspect remote state lock without modifying remote state",
	Long:  "Inspect checks the remote backend configured in .terraform/terraform.tfstate for any active or stale locks and displays diagnostic metadata in a brutalist table.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()

		mgr, _, err := detector.DetectAndLoad(statePathFlag)
		if err != nil {
			return fmt.Errorf("backend detection failed: %w", err)
		}

		lock, err := mgr.Inspect(ctx)
		if err != nil {
			return fmt.Errorf("lock inspection failed for %s: %w", mgr.Target(), err)
		}

		if lock == nil {
			ui.RenderClean(os.Stdout, mgr.Type(), mgr.Target())
			return nil
		}

		ui.RenderLockInfo(os.Stdout, lock, inspectStaleAfter)
		return nil
	},
}

func init() {
	inspectCmd.Flags().DurationVar(&inspectStaleAfter, "stale-after", safety.DefaultStaleThreshold, "duration threshold after which a lock is considered stale (e.g. 30m, 1h)")
	RootCmd.AddCommand(inspectCmd)
}
