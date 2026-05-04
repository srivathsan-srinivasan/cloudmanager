package gcp

import (
	"context"
	"reflect"
	"testing"

	"github.com/srivathsan-srinivasan/cloudmanager/internal/core"
)

func TestGetSSHCmdCLIUsesProjectAndZone(t *testing.T) {
	cmd, err := GetSSHCmdCLI(context.Background(), core.VM{Name: "vm-1", Zone: "us-east1-b"}, core.CloudContext{
		Provider:  "GCP",
		AccountID: "project-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"gcloud", "compute", "ssh", "vm-1", "--project", "project-1", "--zone", "us-east1-b"}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Fatalf("unexpected ssh command args:\nwant %#v\ngot  %#v", want, cmd.Args)
	}
}
