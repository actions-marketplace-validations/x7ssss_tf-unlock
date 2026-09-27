package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/x7ssss/tf-unlock/pkg/detector"
	"github.com/x7ssss/tf-unlock/pkg/safety"
	"github.com/x7ssss/tf-unlock/pkg/ui"
)

var (
	breakLockIDFlag  string
	breakForceFlag   bool
	breakStaleAfter  time.Duration
	breakYesFlag     bool
)

var breakCmd = &cobra.Command{
	Use:   "break",
	Short: "Safely break a remote state lock with staleness gates and confirmation",
	Long:  "Break verifies lock staleness, matches lock ID if provided, prompts for confirmation in interactive sessions, and releases the remote state lock.",
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
			fmt.Println("No active lock found. Nothing to break.")
			return nil
		}

		// Display current lock details
		ui.RenderLockInfo(os.Stdout, lock, breakStaleAfter)

		// 1. Verify lock ID match if specified
		if err := safety.VerifyLockID(lock, breakLockIDFlag); err != nil {
			return err
		}

		// 2. Staleness verification gate
		if err := safety.VerifyStaleness(lock, breakStaleAfter, breakForceFlag); err != nil {
			return err
		}

		// 3. Double-check confirmation
		nonInteractive := !safety.IsTerminal(os.Stdin) || breakYesFlag
		confirmed, err := safety.ConfirmBreak(os.Stdin, os.Stdout, lock.ID, breakForceFlag, nonInteractive)
		if err != nil {
			return err
		}
		if !confirmed {
			return errors.New("aborted by user: lock release cancelled")
		}

		// 4. Execute lock release
		if err := mgr.Break(ctx, lock.ID, breakForceFlag); err != nil {
			return fmt.Errorf("failed to break lock %s: %w", lock.ID, err)
		}

		ui.RenderBreakSuccess(os.Stdout, lock)
		return nil
	},
}

func init() {
	breakCmd.Flags().StringVar(&breakLockIDFlag, "lock-id", "", "expected lock ID to verify against active lock before breaking")
	breakCmd.Flags().BoolVarP(&breakForceFlag, "force", "f", false, "force break active locks without staleness or confirmation checks")
	breakCmd.Flags().DurationVar(&breakStaleAfter, "stale-after", safety.DefaultStaleThreshold, "staleness threshold (e.g. 30m, 1h); locks younger than this require --force")
	breakCmd.Flags().BoolVarP(&breakYesFlag, "yes", "y", false, "automatically answer yes to interactive confirmation prompts")
	RootCmd.AddCommand(breakCmd)
}
