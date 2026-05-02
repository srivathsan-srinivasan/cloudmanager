package access

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"cloudmanager/internal/core"
)

func TestParseSSHConfigAndMatchVM(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	content := `
Host prod-web-01
  HostName 203.0.113.10
  User ubuntu
  IdentityFile ~/sshkeys/prod

Host *.internal
  User root
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	entries, err := ParseSSHConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected two ssh config entries, got %+v", entries)
	}
	if entries[0].PrimaryAlias() != "prod-web-01" {
		t.Fatalf("expected primary alias prod-web-01, got %q", entries[0].PrimaryAlias())
	}
	if !entryMatchesVM(entries[0], core.VM{Name: "prod-web-01", PublicIP: "203.0.113.10"}) {
		t.Fatal("expected SSH config entry to match VM by name/IP")
	}
	if entries[1].PrimaryAlias() != "" {
		t.Fatalf("expected wildcard-only host to be skipped as primary alias, got %q", entries[1].PrimaryAlias())
	}
}

func TestResolveIncludesSSHConfigFallback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte("Host api\n  HostName 198.51.100.10\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLOUDMANAGER_SSH_CONFIG", path)

	methods := Resolve(context.Background(), Request{
		Context: core.CloudContext{Provider: "Unknown"},
		VM:      core.VM{Name: "api", PublicIP: "198.51.100.10"},
	})

	found := false
	for _, method := range methods {
		if method.Kind == "ssh_config" && method.CopyText == "ssh api" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected ssh config fallback, got %+v", methods)
	}
}

func TestResolveManualHostUsesSSHConfigAlias(t *testing.T) {
	host := core.ManualHost{Name: "hetzner-web", Host: "203.0.113.10", Username: "root", Connection: "ssh", SSHConfigHost: "hz-web"}

	methods := Resolve(context.Background(), Request{ManualHost: &host, VM: host.ToVM()})

	if len(methods) != 1 {
		t.Fatalf("expected one manual method, got %+v", methods)
	}
	if methods[0].CopyText != "ssh hz-web" || !methods[0].Available {
		t.Fatalf("expected runnable ssh config command, got %+v", methods[0])
	}
}

func TestResolveManualHostBuildsSSHCommandWithKey(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	host := core.ManualHost{Name: "vultr-web", Host: "198.51.100.20", Username: "deploy", Connection: "ssh", KeyPath: "~/.ssh/vultr"}

	methods := Resolve(context.Background(), Request{ManualHost: &host, VM: host.ToVM()})

	if len(methods) != 1 {
		t.Fatalf("expected one manual method, got %+v", methods)
	}
	expected := "ssh -i " + filepath.Join(home, ".ssh", "vultr") + " deploy@198.51.100.20"
	if methods[0].CopyText != expected {
		t.Fatalf("expected %q, got %q", expected, methods[0].CopyText)
	}
}

func TestResolveManualHostDoesNotAddSSHFallbackForRDP(t *testing.T) {
	host := core.ManualHost{Name: "win", Host: "203.0.113.30", Username: "administrator", Connection: "rdp"}

	methods := Resolve(context.Background(), Request{ManualHost: &host, VM: host.ToVM()})

	if len(methods) != 1 {
		t.Fatalf("expected only unsupported manual method, got %+v", methods)
	}
	if methods[0].Available || len(methods[0].Command) != 0 {
		t.Fatalf("expected unavailable non-SSH manual method, got %+v", methods[0])
	}
}
