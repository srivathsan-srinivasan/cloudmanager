package hosts

import (
	"testing"

	"cloudmanager/internal/config"
)

func TestFromConfigSkipsEmptyHostsAndSanitizes(t *testing.T) {
	hosts := FromConfig(config.AppConfig{
		ManualHosts: []config.ManualHost{
			{Name: "missing"},
			{Name: "hetzner-web", Provider: "Hetzner", Host: "203.0.113.10", Username: "root", Tags: []string{"Prod", "prod"}},
		},
	})

	if len(hosts) != 1 {
		t.Fatalf("expected one usable host, got %+v", hosts)
	}
	if hosts[0].Connection != "ssh" || len(hosts[0].Tags) != 1 || hosts[0].Tags[0] != "prod" {
		t.Fatalf("expected sanitized manual host, got %+v", hosts[0])
	}
}

func TestToVMsMapsManualHostToSearchableVMRecord(t *testing.T) {
	vms := ToVMs(config.AppConfig{
		ManualHosts: []config.ManualHost{
			{Name: "hz-web", Provider: "Hetzner", Host: "203.0.113.10", Username: "root", Tags: []string{"prod"}},
		},
	})

	if len(vms) != 1 {
		t.Fatalf("expected one VM record, got %+v", vms)
	}
	vm := vms[0]
	if vm.Name != "hz-web" || vm.ID != "hz-web" || vm.Type != "manual-host" || vm.PublicIP != "203.0.113.10" {
		t.Fatalf("unexpected VM mapping: %+v", vm)
	}
}
