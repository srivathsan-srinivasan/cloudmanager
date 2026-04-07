package aws

import (
	"context"
	"reflect"
	"testing"

	"cloudmanager/internal/core"
)

func TestGetSSHCmdCLIUsesAccountAuthProfile(t *testing.T) {
	cmd, err := GetSSHCmdCLI(context.Background(), core.VM{ID: "i-123"}, core.CloudContext{
		Provider:          "AWS",
		AccountID:         "9431",
		AccountName:       "main",
		Region:            "us-east-2",
		CredentialProfile: "aws-main-9431",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"aws", "ssm", "start-session", "--target", "i-123", "--profile", "aws-main-9431", "--region", "us-east-2"}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Fatalf("unexpected ssm command args:\nwant %#v\ngot  %#v", want, cmd.Args)
	}
}

func TestGetSSHCmdCLIWithPublicIP(t *testing.T) {
	cmd, err := GetSSHCmdCLI(context.Background(), core.VM{ID: "i-123", PublicIP: "1.2.3.4"}, core.CloudContext{
		Provider:          "AWS",
		AccountID:         "9431",
		AccountName:       "main",
		Region:            "us-east-2",
		CredentialProfile: "aws-main-9431",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"aws", "ec2-instance-connect", "ssh", "--instance-id", "i-123", "--profile", "aws-main-9431", "--region", "us-east-2"}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Fatalf("unexpected connect command args:\nwant %#v\ngot  %#v", want, cmd.Args)
	}
}

func TestGetSSHCmdCLIOmitsProfileWhenEmpty(t *testing.T) {
	cmd, err := GetSSHCmdCLI(context.Background(), core.VM{ID: "i-123"}, core.CloudContext{
		Provider:          "AWS",
		AccountID:         "9431",
		AccountName:       "main",
		Region:            "us-east-2",
		CredentialProfile: "",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should NOT include --profile if CredentialProfile is empty
	want := []string{"aws", "ssm", "start-session", "--target", "i-123", "--region", "us-east-2"}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Fatalf("unexpected command args:\nwant %#v\ngot  %#v", want, cmd.Args)
	}
}
