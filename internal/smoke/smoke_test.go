package smoke

import (
	"reflect"
	"testing"

	"github.com/vyoogam/cloudmanager/internal/core"
)

func TestPreviewCommandsAWS(t *testing.T) {
	got := previewCommands(core.VM{ID: "i-123", PublicIP: "1.2.3.4"}, core.CloudContext{
		Provider:          "AWS",
		AccountID:         "9431",
		AccountName:       "main",
		Region:            "us-east-2",
		CredentialProfile: "aws-main-9431",
	})

	want := []string{"aws", "ec2", "start-instances", "--instance-ids", "i-123", "--dry-run", "--profile", "aws-main-9431", "--region", "us-east-2"}
	if !reflect.DeepEqual(got["Start"], want) {
		t.Fatalf("unexpected AWS start preview:\nwant %#v\ngot  %#v", want, got["Start"])
	}

	wantSSH := []string{"aws", "ec2-instance-connect", "ssh", "--instance-id", "i-123", "--profile", "aws-main-9431", "--region", "us-east-2"}
	if !reflect.DeepEqual(got["SSH"], wantSSH) {
		t.Fatalf("unexpected AWS SSH preview:\nwant %#v\ngot  %#v", wantSSH, got["SSH"])
	}
}

func TestPreviewCommandsGCP(t *testing.T) {
	got := previewCommands(core.VM{Name: "vm-1", Zone: "us-east1-b"}, core.CloudContext{
		Provider:  "GCP",
		AccountID: "project-1",
	})

	want := []string{"gcloud", "compute", "instances", "reset", "vm-1", "--project", "project-1", "--zone", "us-east1-b"}
	if !reflect.DeepEqual(got["Restart"], want) {
		t.Fatalf("unexpected GCP restart preview:\nwant %#v\ngot  %#v", want, got["Restart"])
	}
}

func TestPreviewCommandsAzure(t *testing.T) {
	got := previewCommands(core.VM{Name: "vm-1", ResourceGroup: "rg-main"}, core.CloudContext{
		Provider:  "Azure",
		AccountID: "sub-123",
	})

	want := []string{"az", "ssh", "vm", "--name", "vm-1", "--resource-group", "rg-main", "--subscription", "sub-123"}
	if !reflect.DeepEqual(got["SSH"], want) {
		t.Fatalf("unexpected Azure SSH preview:\nwant %#v\ngot  %#v", want, got["SSH"])
	}
}

func TestTargetProvidersAll(t *testing.T) {
	got, err := targetProviders("all")
	if err != nil {
		t.Fatalf("targetProviders returned error: %v", err)
	}

	want := []string{"AWS", "Azure", "DigitalOcean", "GCP"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected providers:\nwant %#v\ngot  %#v", want, got)
	}
}

func TestTargetProvidersInvalid(t *testing.T) {
	if _, err := targetProviders("randomcloud"); err == nil {
		t.Fatal("expected invalid provider error")
	}
}
