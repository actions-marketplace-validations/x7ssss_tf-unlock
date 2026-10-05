package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/x7ssss/tf-unlock/pkg/ui"
)

var (
	autoStaleAfter time.Duration
)

var autoCmd = &cobra.Command{
	Use:   "auto",
	Short: "Zero-config CI state lock blocker and stale lock auto-breaker",
	Long:  "Auto is designed for CI pipelines (GitHub Actions, GitLab CI). It exits 0 if the state is clean or if a stale lock was safely cleared, and exits 1 if an active lock is currently held.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()

		mgr, err := loadManager()
		if err != nil {
			return fmt.Errorf("backend detection failed: %w", err)
		}

		lock, err := mgr.Inspect(ctx)
		if err != nil {
			return fmt.Errorf("lock inspection failed for %s: %w", mgr.Target(), err)
		}

		// 1. Clean state: no lock
		if lock == nil {
			ui.RenderClean(os.Stdout, mgr.Type(), mgr.Target())
			return nil
		}

		// 2. Lock is stale: break automatically
		if lock.IsStale(autoStaleAfter) {
			if err := verifyRunner(ctx, false); err != nil {
				return err
			}
			if dryRunFlag {
				fmt.Fprintf(os.Stdout, "[dry-run] would break stale lock %s on %s\n", lock.ID, mgr.Target())
				return nil
			}
			if err := mgr.Break(ctx, lock.ID, false); err != nil {
				return fmt.Errorf("failed to auto-break stale lock %s: %w", lock.ID, err)
			}
			ui.RenderAutoUnlocked(os.Stdout, lock, autoStaleAfter)
			return nil
		}

		// 3. Lock is active: block CI
		ui.RenderAutoBlocked(os.Stderr, lock, autoStaleAfter)
		os.Exit(1)
		return nil
	},
}

func init() {
	autoCmd.Flags().DurationVar(&autoStaleAfter, "stale-after", 1*time.Hour, "staleness threshold for auto-breaking (locks older than this are broken; younger locks block CI)")
	RootCmd.AddCommand(autoCmd)
}
