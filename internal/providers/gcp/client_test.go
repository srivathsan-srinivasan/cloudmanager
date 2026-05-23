package gcp

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/vyoogam/cloudmanager/internal/core"
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

func TestGetSSHCmdCLINormalizesZoneURL(t *testing.T) {
	cmd, err := GetSSHCmdCLI(context.Background(), core.VM{
		Name: "vm-1",
		Zone: "https://www.googleapis.com/compute/v1/projects/project-1/zones/us-east1-b",
	}, core.CloudContext{
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

func TestResolveGCPVMZoneUsesExistingZone(t *testing.T) {
	zone, err := resolveGCPVMZone(context.Background(), "project-1", core.VM{
		Name: "vm-1",
		Zone: "projects/project-1/zones/us-central1-a",
	}, func(context.Context, string) ([]core.VM, error) {
		t.Fatal("fetch should not be called when zone is already present")
		return nil, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if zone != "us-central1-a" {
		t.Fatalf("expected normalized zone, got %q", zone)
	}
}

func TestResolveGCPVMZoneDiscoversMissingZoneByName(t *testing.T) {
	zone, err := resolveGCPVMZone(context.Background(), "project-1", core.VM{Name: "vm-1"}, func(ctx context.Context, project string) ([]core.VM, error) {
		if project != "project-1" {
			t.Fatalf("unexpected project %q", project)
		}
		return []core.VM{{Name: "vm-1", ID: "123", Zone: "us-west1-b"}}, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if zone != "us-west1-b" {
		t.Fatalf("expected discovered zone, got %q", zone)
	}
}

func TestResolveGCPVMZoneReportsDiscoveryFailure(t *testing.T) {
	_, err := resolveGCPVMZone(context.Background(), "project-1", core.VM{Name: "vm-1"}, func(context.Context, string) ([]core.VM, error) {
		return nil, errors.New("list failed")
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGCPInstanceToVMNormalizesZone(t *testing.T) {
	vm := gcpInstanceToVM(gcpInstance{
		Name:        "vm-1",
		Id:          "123",
		MachineType: "https://www.googleapis.com/compute/v1/projects/project-1/zones/us-central1-a/machineTypes/e2-medium",
		Status:      "RUNNING",
		Zone:        "https://www.googleapis.com/compute/v1/projects/project-1/zones/us-central1-a",
	})
	if vm.Zone != "us-central1-a" {
		t.Fatalf("expected normalized zone, got %q", vm.Zone)
	}
}
