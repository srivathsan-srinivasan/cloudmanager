package core

import (
	"strings"
	"testing"
)

func TestVMConsoleURLAWS(t *testing.T) {
	got := VMConsoleURL(CloudContext{Provider: "AWS", Region: "us-east-2"}, VM{ID: "i-123"})
	if !strings.Contains(got, "ec2/home?region=us-east-2#InstanceDetails:instanceId=i-123") {
		t.Fatalf("unexpected AWS VM console URL: %s", got)
	}
}

func TestVMConsoleURLGCPNormalizesZone(t *testing.T) {
	got := VMConsoleURL(
		CloudContext{Provider: "GCP", AccountID: "project-1"},
		VM{Name: "web-1", Zone: "https://www.googleapis.com/compute/v1/projects/project-1/zones/us-central1-a"},
	)
	if !strings.Contains(got, "/zones/us-central1-a/instances/web-1?project=project-1") {
		t.Fatalf("unexpected GCP VM console URL: %s", got)
	}
}

func TestAzureConsoleURLUsesResourceID(t *testing.T) {
	resourceID := "/subscriptions/sub-1/resourceGroups/rg-main/providers/Microsoft.Compute/virtualMachines/web-1"
	got := VMConsoleURL(CloudContext{Provider: "Azure", Tenant: "example.com"}, VM{ID: resourceID})
	if !strings.Contains(got, "portal.azure.com/#@example.com/resource/subscriptions/sub-1/resourceGroups/rg-main/providers/Microsoft.Compute/virtualMachines/web-1") {
		t.Fatalf("unexpected Azure VM console URL: %s", got)
	}
}

func TestDetailWithConsoleURLPrependsLink(t *testing.T) {
	got := DetailWithConsoleURL("Name: web-1\n", "https://example.com/resource")
	if !strings.HasPrefix(got, "Console:        https://example.com/resource\n\nName: web-1") {
		t.Fatalf("unexpected detail text: %q", got)
	}
}
