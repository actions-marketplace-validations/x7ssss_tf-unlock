package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCLI_Inspect_NoLock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "terraform.tfstate")
	content := `{
		"version": 3,
		"backend": {
			"type": "s3",
			"config": {
				"bucket": "my-bucket",
				"key": "test.tfstate",
				"region": "us-east-1",
				"endpoint": "` + server.URL + `",
				"use_lockfile": true
			}
		}
	}`
	if err := os.WriteFile(stateFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test state file: %v", err)
	}

	os.Setenv("AWS_ACCESS_KEY_ID", "TEST_KEY")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "TEST_SECRET")
	defer os.Unsetenv("AWS_ACCESS_KEY_ID")
	defer os.Unsetenv("AWS_SECRET_ACCESS_KEY")

	cmd := RootCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"inspect", "--state", stateFile})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("inspect command failed: %v", err)
	}
}

func TestCLI_Break_StalenessGate(t *testing.T) {
	// Mock an active lock created just 2 minutes ago
	createdRecent := time.Now().Add(-2 * time.Minute).Format(time.RFC3339)
	mockLockJSON := `{"ID":"active-lock-123","Operation":"OperationTypeApply","Info":"in progress","Who":"user@host","Version":"1.10.0","Created":"` + createdRecent + `","Path":"test.tfstate"}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"etag123"`)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockLockJSON))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "terraform.tfstate")
	content := `{
		"version": 3,
		"backend": {
			"type": "s3",
			"config": {
				"bucket": "my-bucket",
				"key": "test.tfstate",
				"region": "us-east-1",
				"endpoint": "` + server.URL + `",
				"use_lockfile": true
			}
		}
	}`
	if err := os.WriteFile(stateFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test state file: %v", err)
	}

	os.Setenv("AWS_ACCESS_KEY_ID", "TEST_KEY")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "TEST_SECRET")
	defer os.Unsetenv("AWS_ACCESS_KEY_ID")
	defer os.Unsetenv("AWS_SECRET_ACCESS_KEY")

	cmd := RootCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	// Without --force, break should be refused because age (2m) < 30m threshold
	cmd.SetArgs([]string{"break", "--state", stateFile, "--stale-after", "30m"})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected staleness gate error, got nil")
	}
	if !strings.Contains(err.Error(), "refusing to break active lock") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCLI_Break_Force(t *testing.T) {
	createdRecent := time.Now().Add(-2 * time.Minute).Format(time.RFC3339)
	mockLockJSON := `{"ID":"active-lock-123","Operation":"OperationTypeApply","Info":"in progress","Who":"user@host","Version":"1.10.0","Created":"` + createdRecent + `","Path":"test.tfstate"}`

	deleted := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.Header().Set("ETag", `"etag123"`)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(mockLockJSON))
		} else if r.Method == "DELETE" {
			deleted = true
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()

	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "terraform.tfstate")
	content := `{
		"version": 3,
		"backend": {
			"type": "s3",
			"config": {
				"bucket": "my-bucket",
				"key": "test.tfstate",
				"region": "us-east-1",
				"endpoint": "` + server.URL + `",
				"use_lockfile": true
			}
		}
	}`
	if err := os.WriteFile(stateFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test state file: %v", err)
	}

	os.Setenv("AWS_ACCESS_KEY_ID", "TEST_KEY")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "TEST_SECRET")
	defer os.Unsetenv("AWS_ACCESS_KEY_ID")
	defer os.Unsetenv("AWS_SECRET_ACCESS_KEY")

	cmd := RootCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	// With --force, break should succeed
	cmd.SetArgs([]string{"break", "--state", stateFile, "--force"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error with --force: %v", err)
	}

	if !deleted {
		t.Errorf("expected DELETE request to be executed on server")
	}
}
