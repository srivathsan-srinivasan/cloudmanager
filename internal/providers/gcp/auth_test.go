package gcp

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestGCPAccessTokenFromCLITrimsOutput(t *testing.T) {
	origExec := gcpExecCommandContext
	gcpExecCommandContext = fakeGCloudExecCommandContext
	defer func() {
		gcpExecCommandContext = origExec
	}()

	t.Setenv("GO_WANT_GCLOUD_HELPER_PROCESS", "0")
	t.Setenv("GCLOUD_HELPER_MODE", "token")
	t.Setenv("GCLOUD_HELPER_STDOUT", "fake-access-token\n")

	token, err := gcpAccessTokenFromCLI(context.Background())
	if err != nil {
		t.Fatalf("expected token fallback to succeed, got error: %v", err)
	}
	if token != "fake-access-token" {
		t.Fatalf("expected trimmed token, got %q", token)
	}
}

func TestGCPAccessTokenFromCLIReturnsCommandOutputOnFailure(t *testing.T) {
	origExec := gcpExecCommandContext
	gcpExecCommandContext = fakeGCloudExecCommandContext
	defer func() {
		gcpExecCommandContext = origExec
	}()

	t.Setenv("GO_WANT_GCLOUD_HELPER_PROCESS", "0")
	t.Setenv("GCLOUD_HELPER_MODE", "fail")
	t.Setenv("GCLOUD_HELPER_STDERR", "not logged in")

	_, err := gcpAccessTokenFromCLI(context.Background())
	if err == nil {
		t.Fatal("expected token fallback to fail")
	}
	if !strings.Contains(err.Error(), "not logged in") {
		t.Fatalf("expected command stderr in error, got %v", err)
	}
}

func fakeGCloudExecCommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	testArgs := []string{"-test.run=TestGCloudHelperProcess", "--", name}
	testArgs = append(testArgs, args...)
	cmd := exec.CommandContext(ctx, os.Args[0], testArgs...)
	cmd.Env = append(os.Environ(), "GO_WANT_GCLOUD_HELPER_PROCESS=1")
	return cmd
}

func TestGCloudHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_GCLOUD_HELPER_PROCESS") != "1" {
		return
	}

	mode := os.Getenv("GCLOUD_HELPER_MODE")
	switch mode {
	case "token":
		_, _ = os.Stdout.WriteString(os.Getenv("GCLOUD_HELPER_STDOUT"))
		os.Exit(0)
	case "fail":
		_, _ = os.Stderr.WriteString(os.Getenv("GCLOUD_HELPER_STDERR"))
		os.Exit(1)
	default:
		os.Exit(2)
	}
}
