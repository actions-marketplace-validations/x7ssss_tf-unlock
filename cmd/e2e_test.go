package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func resetFlags() {
	dryRunFlag, breakForceFlag, breakYesFlag = false, false, false
	breakLockIDFlag, githubRunIDFlag, githubRepoFlag, githubTokenFlag = "", "", "", ""
	breakStaleAfter = 30 * time.Minute
	autoStaleAfter = time.Hour
}

// e2eEnv starts a mock S3 (native lockfile) holding a 2h old lock and a mock GitHub API.
func e2eEnv(t *testing.T, ghStatus, ghConclusion string) (state string, ghURL string, deletes *int32) {
	t.Helper()
	created := time.Now().Add(-2 * time.Hour).Format(time.RFC3339)
	lock := `{"ID":"lock-1","Operation":"OperationTypeApply","Info":"","Who":"ci@runner","Version":"1.10.0","Created":"` + created + `","Path":"s.tfstate"}`

	deletes = new(int32)
	s3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			w.Header().Set("ETag", `"e1"`)
			w.Write([]byte(lock))
		case "DELETE":
			atomic.AddInt32(deletes, 1)
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	t.Cleanup(s3.Close)

	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/o/r/actions/runs/42" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write([]byte(`{"status":"` + ghStatus + `","conclusion":"` + ghConclusion + `"}`))
	}))
	t.Cleanup(gh.Close)

	state = filepath.Join(t.TempDir(), "terraform.tfstate")
	content := `{"version":3,"backend":{"type":"s3","config":{"bucket":"b","key":"s.tfstate","region":"us-east-1","endpoint":"` + s3.URL + `","use_lockfile":true}}}`
	if err := os.WriteFile(state, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AWS_ACCESS_KEY_ID", "K")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "S")
	t.Cleanup(resetFlags)
	return state, gh.URL, deletes
}

func runCLI(args ...string) (string, error) {
	buf := new(bytes.Buffer)
	RootCmd.SetOut(buf)
	RootCmd.SetErr(buf)
	RootCmd.SetArgs(args)
	err := RootCmd.Execute()
	return buf.String(), err
}

func TestE2E_Break_RunnerGate(t *testing.T) {
	tests := []struct {
		name       string
		status     string
		conclusion string
		wantErr    string
		wantDelete int32
	}{
		{"cancelled run allows break", "completed", "cancelled", "", 1},
		{"timed_out run allows break", "completed", "timed_out", "", 1},
		{"failed run allows break", "completed", "failure", "", 1},
		{"in_progress run blocks break", "in_progress", "", "still active", 0},
		{"queued run blocks break", "queued", "", "still active", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetFlags()
			state, gh, deletes := e2eEnv(t, tt.status, tt.conclusion)
			_, err := runCLI("break", "--state", state, "--yes", "--lock-id", "lock-1",
				"--github-api-url", gh, "--github-repo", "o/r", "--github-run-id", "42")
			if tt.wantErr == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
			if got := atomic.LoadInt32(deletes); got != tt.wantDelete {
				t.Errorf("deletes = %d, want %d", got, tt.wantDelete)
			}
		})
	}
}

func TestE2E_Break_DryRunNeverDeletes(t *testing.T) {
	resetFlags()
	state, gh, deletes := e2eEnv(t, "completed", "cancelled")
	_, err := runCLI("break", "--state", state, "--yes", "--dry-run",
		"--github-api-url", gh, "--github-repo", "o/r", "--github-run-id", "42")
	if err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(deletes) != 0 {
		t.Errorf("dry-run must not delete; deletes=%d", atomic.LoadInt32(deletes))
	}
}

func TestE2E_Break_LockIDMismatch(t *testing.T) {
	resetFlags()
	state, _, deletes := e2eEnv(t, "completed", "cancelled")
	_, err := runCLI("break", "--state", state, "--yes", "--lock-id", "other")
	if err == nil || !strings.Contains(err.Error(), "lock ID mismatch") {
		t.Fatalf("error = %v", err)
	}
	if atomic.LoadInt32(deletes) != 0 {
		t.Error("must not delete on ID mismatch")
	}
}

func TestE2E_Auto_DryRun(t *testing.T) {
	resetFlags()
	state, _, deletes := e2eEnv(t, "completed", "cancelled")
	if _, err := runCLI("auto", "--state", state, "--stale-after", "1h", "--dry-run"); err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(deletes) != 0 {
		t.Error("auto --dry-run must not delete")
	}
}

func TestLoadManager_Flags(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "K")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "S")
	t.Cleanup(func() { backendTypeFlag, tableNameFlag, s3BucketFlag, s3KeyFlag = "", "", "", "" })

	backendTypeFlag, tableNameFlag, s3BucketFlag, s3KeyFlag = "s3-dynamodb", "tbl", "b", "k"
	mgr, err := loadManager()
	if err != nil || mgr.Type() != "s3-dynamodb" {
		t.Fatalf("dynamodb: %v %v", mgr, err)
	}
	backendTypeFlag = "s3-native"
	if mgr, err = loadManager(); err != nil || mgr.Type() != "s3-native" {
		t.Fatalf("native: %v %v", mgr, err)
	}
	tableNameFlag, backendTypeFlag = "", "s3-dynamodb"
	if _, err = loadManager(); err == nil {
		t.Error("expected missing table error")
	}
	backendTypeFlag = "nope"
	if _, err = loadManager(); err == nil {
		t.Error("expected unsupported backend error")
	}
}
