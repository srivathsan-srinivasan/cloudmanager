package aws

import (
	"context"
	"reflect"
	"strings"
	"testing"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/srivathsan-srinivasan/cloudmanager/internal/core"
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

	want := []string{"aws", "--no-cli-pager", "ssm", "start-session", "--target", "i-123", "--profile", "aws-main-9431", "--region", "us-east-2"}
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

	want := []string{"aws", "--no-cli-pager", "ec2-instance-connect", "ssh", "--instance-id", "i-123", "--profile", "aws-main-9431", "--region", "us-east-2"}
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
	want := []string{"aws", "--no-cli-pager", "ssm", "start-session", "--target", "i-123", "--region", "us-east-2"}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Fatalf("unexpected command args:\nwant %#v\ngot  %#v", want, cmd.Args)
	}
}

func TestFormatAWSInstanceDetailIncludesOperationalFields(t *testing.T) {
	out := formatAWSInstanceDetail(awsInstanceDetail{
		Name:             "web-1",
		InstanceID:       "i-123",
		InstanceType:     "t3.medium",
		State:            "running",
		LaunchTime:       "2026-05-04T10:00:00Z",
		Uptime:           "2h 10m",
		KeyName:          "prod-key",
		VpcID:            "vpc-123",
		VpcCIDR:          "10.0.0.0/16",
		SubnetID:         "subnet-123",
		SubnetCIDR:       "10.0.1.0/24",
		AvailabilityZone: "us-east-1a",
		PrivateIP:        "10.0.1.10",
		PublicIP:         "203.0.113.10",
		SecurityGroups:   []string{"web-sg (sg-123)"},
		IAMProfile:       "arn:aws:iam::123:instance-profile/web",
		Tags:             []string{"Name=web-1", "app=web"},
	})

	for _, want := range []string{
		"AWS INSTANCE DETAILS",
		"SSH Key Pair: prod-key",
		"Launch Time / Last Start: 2026-05-04T10:00:00Z",
		"Uptime: 2h 10m",
		"VPC CIDR: 10.0.0.0/16",
		"Subnet CIDR: 10.0.1.0/24",
		"Security Groups: web-sg (sg-123)",
		"IAM Profile: arn:aws:iam::123:instance-profile/web",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected formatted describe to contain %q, got:\n%s", want, out)
		}
	}
}

func TestAWSInstanceDetailFromSDKHandlesMissingIAMProfile(t *testing.T) {
	detail := awsInstanceDetailFromSDK(ec2types.Instance{
		BlockDeviceMappings: []ec2types.InstanceBlockDeviceMapping{{}},
		NetworkInterfaces:   []ec2types.InstanceNetworkInterface{{}},
	})

	if detail.IAMProfile != "" {
		t.Fatalf("expected empty IAM profile for nil SDK profile, got %q", detail.IAMProfile)
	}
	if len(detail.Volumes) != 1 || len(detail.NetworkInterfaces) != 1 {
		t.Fatalf("expected nil-safe volume/network formatting, got volumes=%v network=%v", detail.Volumes, detail.NetworkInterfaces)
	}
}
