package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/x7ssss/tf-unlock/pkg/backend"
	"github.com/x7ssss/tf-unlock/pkg/detector"
	"github.com/x7ssss/tf-unlock/pkg/ghrun"
	"github.com/x7ssss/tf-unlock/pkg/safety"
)

var (
	dryRunFlag       bool
	backendTypeFlag  string
	tableNameFlag    string
	s3BucketFlag     string
	s3KeyFlag        string
	githubTokenFlag  string
	githubRepoFlag   string
	githubRunIDFlag  string
	githubAPIURLFlag string
)

func init() {
	pf := RootCmd.PersistentFlags()
	pf.BoolVar(&dryRunFlag, "dry-run", false, "run every safety check but never delete the lock")
	pf.StringVar(&backendTypeFlag, "backend-type", "", "backend type override (s3-dynamodb, s3-native); skips reading the state file")
	pf.StringVar(&tableNameFlag, "table-name", "", "DynamoDB lock table (with --backend-type s3-dynamodb)")
	pf.StringVar(&s3BucketFlag, "s3-bucket", "", "S3 bucket of the Terraform state")
	pf.StringVar(&s3KeyFlag, "s3-key", "", "S3 key of the Terraform state")
	pf.StringVar(&githubTokenFlag, "github-token", os.Getenv("GITHUB_TOKEN"), "GitHub token used to look up the lock owner's workflow run")
	pf.StringVar(&githubRepoFlag, "github-repo", os.Getenv("GITHUB_REPOSITORY"), "owner/name of the repository that owns the lock holder run")
	pf.StringVar(&githubRunIDFlag, "github-run-id", "", "workflow run ID of the lock holder; the lock is only broken if that run has terminated")
	pf.StringVar(&githubAPIURLFlag, "github-api-url", ghrun.DefaultAPIURL, "GitHub API base URL")
}

// loadManager builds a lock manager from explicit flags, falling back to the state file.
func loadManager() (backend.LockManager, error) {
	if backendTypeFlag == "" {
		mgr, _, err := detector.DetectAndLoad(statePathFlag)
		return mgr, err
	}
	cfg := &detector.BackendConfig{Type: "s3", Config: map[string]interface{}{
		"bucket": s3BucketFlag,
		"key":    s3KeyFlag,
	}}
	switch backendTypeFlag {
	case "s3-dynamodb":
		if tableNameFlag == "" {
			return nil, errors.New("--table-name is required for --backend-type s3-dynamodb")
		}
		cfg.Config["dynamodb_table"] = tableNameFlag
	case "s3-native":
		cfg.Config["use_lockfile"] = true
	default:
		return nil, fmt.Errorf("unsupported --backend-type %q: use s3-dynamodb or s3-native", backendTypeFlag)
	}
	return detector.NewLockManager(cfg)
}

// verifyRunner enforces the runner-termination gate when a run ID was supplied.
func verifyRunner(ctx context.Context, force bool) error {
	if githubRunIDFlag == "" {
		return nil
	}
	checker := &ghrun.Checker{BaseURL: githubAPIURLFlag, Token: githubTokenFlag}
	return safety.VerifyRunnerTerminated(ctx, checker, githubRepoFlag, githubRunIDFlag, force)
}
