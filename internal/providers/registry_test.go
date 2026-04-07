package providers

import "testing"

func TestRegisteredProvidersIncludeDigitalOcean(t *testing.T) {
	names := RegisteredProviderNames()
	expected := map[string]bool{
		"AWS":          false,
		"GCP":          false,
		"Azure":        false,
		"DigitalOcean": false,
	}

	for _, name := range names {
		if _, ok := expected[name]; ok {
			expected[name] = true
		}
	}

	for name, seen := range expected {
		if !seen {
			t.Fatalf("expected provider %s to be registered", name)
		}
	}
}

func TestSupportsUsesRegisteredCapabilities(t *testing.T) {
	if !Supports("DigitalOcean", CapabilityVMs) {
		t.Fatal("expected DigitalOcean to support VMs")
	}
	if Supports("DigitalOcean", CapabilityDisks) {
		t.Fatal("did not expect DigitalOcean to support disks yet")
	}
	if !Supports("AWS", CapabilitySnapshots) {
		t.Fatal("expected AWS to support snapshots")
	}
	if !Supports("AWS", CapabilityFirewalls) {
		t.Fatal("expected AWS to support firewalls")
	}
	if !Supports("GCP", CapabilityFirewalls) {
		t.Fatal("expected GCP to support firewalls")
	}
}
