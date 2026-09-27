package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	statePathFlag string
	version       = "1.0.0"
)

// RootCmd represents the base command when called without any subcommands.
var RootCmd = &cobra.Command{
	Use:     "tf-unlock",
	Short:   "High-performance remote state lock inspector and breaker for Terraform and OpenTofu",
	Long:    "tf-unlock is a zero-external-dependency CLI designed to inspect, diagnose, and safely break remote state locks across AWS S3/DynamoDB, S3 Native object locks, Azure Blob Storage, and PostgreSQL backends.",
	Version: version,
}

func init() {
	RootCmd.PersistentFlags().StringVarP(&statePathFlag, "state", "s", ".terraform/terraform.tfstate", "path to .terraform/terraform.tfstate")
}

// Execute runs the root CLI command.
func Execute() error {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	return nil
}
