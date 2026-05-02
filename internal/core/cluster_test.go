package core

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestEnsureKubeContextUsesExistingContextAndReturnsK9s(t *testing.T) {
	origExec := clusterExecCommand
	origExecContext := clusterExecCommandContext
	clusterExecCommand = fakeClusterExecCommand
	clusterExecCommandContext = fakeClusterExecCommandContext
	defer func() {
		clusterExecCommand = origExec
		clusterExecCommandContext = origExecContext
	}()

	t.Setenv("GO_WANT_CLUSTER_HELPER_PROCESS", "0")
	t.Setenv("CLUSTER_HELPER_CONTEXTS", "prod\nstaging\n")

	cmd, err := EnsureKubeContext(context.Background(), "staging", "aws", "eks", "update-kubeconfig")
	if err != nil {
		t.Fatalf("expected existing kube context to succeed, got %v", err)
	}
	if cmd == nil {
		t.Fatal("expected k9s command")
	}
	if got := cmd.Args[len(cmd.Args)-1]; got != "k9s" {
		t.Fatalf("expected helper to launch k9s, got args %#v", cmd.Args)
	}
}

func TestEnsureKubeContextMatchesExistingRenamedContext(t *testing.T) {
	origExec := clusterExecCommand
	origExecContext := clusterExecCommandContext
	clusterExecCommand = fakeClusterExecCommand
	clusterExecCommandContext = fakeClusterExecCommandContext
	defer func() {
		clusterExecCommand = origExec
		clusterExecCommandContext = origExecContext
	}()

	t.Setenv("GO_WANT_CLUSTER_HELPER_PROCESS", "0")
	t.Setenv("CLUSTER_HELPER_CONTEXTS", "prod-admin\nteam-prod-us-east-1\n")
	t.Setenv("CLUSTER_HELPER_FAIL_FETCH", "1")

	cmd, err := EnsureKubeContextForTarget(context.Background(), KubeContextTarget{
		ExpectedContext: "arn:aws:eks:us-east-1:111122223333:cluster/prod",
		ClusterName:     "prod",
		AccountID:       "111122223333",
		Location:        "us-east-1",
	}, "aws", "eks", "update-kubeconfig")
	if err != nil {
		t.Fatalf("expected existing fuzzy kube context to succeed, got %v", err)
	}
	if cmd == nil {
		t.Fatal("expected k9s command")
	}
}

func TestEnsureKubeContextReturnsFetchFailure(t *testing.T) {
	origExec := clusterExecCommand
	origExecContext := clusterExecCommandContext
	clusterExecCommand = fakeClusterExecCommand
	clusterExecCommandContext = fakeClusterExecCommandContext
	defer func() {
		clusterExecCommand = origExec
		clusterExecCommandContext = origExecContext
	}()

	t.Setenv("GO_WANT_CLUSTER_HELPER_PROCESS", "0")
	t.Setenv("CLUSTER_HELPER_CONTEXTS", "")
	t.Setenv("CLUSTER_HELPER_FAIL_FETCH", "1")
	t.Setenv("CLUSTER_HELPER_FETCH_STDERR", "missing credentials")

	_, err := EnsureKubeContext(context.Background(), "staging", "aws", "eks", "update-kubeconfig")
	if err == nil {
		t.Fatal("expected fetch failure")
	}
	if !strings.Contains(err.Error(), "missing credentials") {
		t.Fatalf("expected fetch stderr in error, got %v", err)
	}
}

func fakeClusterExecCommand(name string, args ...string) *exec.Cmd {
	testArgs := []string{"-test.run=TestClusterHelperProcess", "--", name}
	testArgs = append(testArgs, args...)
	cmd := exec.Command(os.Args[0], testArgs...)
	cmd.Env = append(os.Environ(), "GO_WANT_CLUSTER_HELPER_PROCESS=1")
	return cmd
}

func fakeClusterExecCommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	testArgs := []string{"-test.run=TestClusterHelperProcess", "--", name}
	testArgs = append(testArgs, args...)
	cmd := exec.CommandContext(ctx, os.Args[0], testArgs...)
	cmd.Env = append(os.Environ(), "GO_WANT_CLUSTER_HELPER_PROCESS=1")
	return cmd
}

func TestClusterHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_CLUSTER_HELPER_PROCESS") != "1" {
		return
	}

	args := os.Args
	dash := 0
	for i, arg := range args {
		if arg == "--" {
			dash = i
			break
		}
	}
	if dash == 0 || dash+1 >= len(args) {
		os.Exit(2)
	}

	name := args[dash+1]
	cmdArgs := args[dash+2:]

	switch {
	case name == "kubectl" && len(cmdArgs) >= 3 && cmdArgs[0] == "config" && cmdArgs[1] == "get-contexts":
		_, _ = os.Stdout.WriteString(os.Getenv("CLUSTER_HELPER_CONTEXTS"))
		os.Exit(0)
	case name == "kubectl" && len(cmdArgs) >= 3 && cmdArgs[0] == "config" && cmdArgs[1] == "use-context":
		os.Exit(0)
	case name == "k9s":
		os.Exit(0)
	default:
		if os.Getenv("CLUSTER_HELPER_FAIL_FETCH") == "1" {
			_, _ = os.Stderr.WriteString(os.Getenv("CLUSTER_HELPER_FETCH_STDERR"))
			os.Exit(1)
		}
		os.Exit(0)
	}
}
