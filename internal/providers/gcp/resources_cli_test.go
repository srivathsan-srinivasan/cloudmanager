package gcp

import (
	"strings"
	"testing"

	"google.golang.org/api/compute/v1"
)

func TestParseGCPDisksCLI(t *testing.T) {
	output := []byte(`[
		{
			"name": "disk-a",
			"id": "123",
			"sizeGb": "200",
			"type": "https://www.googleapis.com/compute/v1/projects/p1/zones/us-east1-b/diskTypes/pd-ssd",
			"status": "READY",
			"zone": "https://www.googleapis.com/compute/v1/projects/p1/zones/us-east1-b",
			"users": ["https://www.googleapis.com/compute/v1/projects/p1/zones/us-east1-b/instances/web-1"],
			"creationTimestamp": "2026-04-01T00:00:00.000-00:00",
			"labels": {"env": "prod", "tier": "web"}
		}
	]`)

	disks, err := parseGCPDisksCLI(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(disks) != 1 {
		t.Fatalf("expected 1 disk, got %d", len(disks))
	}
	if disks[0].Name != "disk-a" || disks[0].SizeGB != 200 || disks[0].Type != "pd-ssd" {
		t.Fatalf("unexpected disk parse result: %#v", disks[0])
	}
	if disks[0].AttachedToVM != "web-1" || disks[0].Zone != "us-east1-b" {
		t.Fatalf("unexpected disk attachment/zone parse: %#v", disks[0])
	}
	if !strings.Contains(disks[0].Labels, "env=prod") || !strings.Contains(disks[0].Labels, "tier=web") {
		t.Fatalf("expected labels to be preserved, got %#v", disks[0])
	}
}

func TestParseGCPSnapshotsCLI(t *testing.T) {
	output := []byte(`[
		{
			"name": "snap-a",
			"id": "456",
			"status": "READY",
			"diskSizeGb": "50",
			"sourceDisk": "https://www.googleapis.com/compute/v1/projects/p1/zones/us-east1-b/disks/disk-a",
			"creationTimestamp": "2026-04-01T00:00:00.000-00:00",
			"description": "nightly snapshot",
			"labels": {"env": "prod"}
		}
	]`)

	snaps, err := parseGCPSnapshotsCLI(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snaps))
	}
	if snaps[0].Name != "snap-a" || snaps[0].SizeGB != 50 || snaps[0].SourceDiskName != "disk-a" {
		t.Fatalf("unexpected snapshot parse result: %#v", snaps[0])
	}
	if snaps[0].Description != "nightly snapshot" || !strings.Contains(snaps[0].Labels, "env=prod") {
		t.Fatalf("expected description/labels to be preserved, got %#v", snaps[0])
	}
}

func TestGCPSecurityGroupsFromNetworkData(t *testing.T) {
	networks := []*compute.Network{
		{Name: "prod-vpc", SelfLink: "https://www.googleapis.com/compute/v1/projects/p1/global/networks/prod-vpc"},
	}
	firewalls := []*compute.Firewall{
		{
			Name:         "allow-ssh",
			Network:      "https://www.googleapis.com/compute/v1/projects/p1/global/networks/prod-vpc",
			Direction:    "INGRESS",
			SourceRanges: []string{"0.0.0.0/0"},
			Allowed: []*compute.FirewallAllowed{
				{IPProtocol: "tcp", Ports: []string{"22"}},
			},
		},
		{
			Name:      "allow-egress",
			Network:   "https://www.googleapis.com/compute/v1/projects/p1/global/networks/prod-vpc",
			Direction: "EGRESS",
			Allowed: []*compute.FirewallAllowed{
				{IPProtocol: "tcp", Ports: []string{"443"}},
			},
		},
	}

	groups := gcpSecurityGroupsFromNetworkData(networks, firewalls, map[string]int{"prod-vpc": 2})
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].Name != "prod-vpc" || groups[0].InboundRuleCount != 1 || groups[0].OutboundRuleCount != 1 {
		t.Fatalf("unexpected group summary: %#v", groups[0])
	}
	if groups[0].AttachedResources != 2 || !groups[0].HasOpenSSH {
		t.Fatalf("expected attachment count and open ssh audit flag, got %#v", groups[0])
	}
}
