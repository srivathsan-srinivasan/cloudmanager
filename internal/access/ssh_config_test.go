package access

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/srivathsan-srinivasan/cloudmanager/internal/core"
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

func TestResolveSkipsUnrelatedSSHConfigAliases(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte(`
Host github-work
  HostName github.com

Host github-personal
  HostName github.com
`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLOUDMANAGER_SSH_CONFIG", path)
	t.Setenv("CLOUDMANAGER_SSH_KEY_DIRS", filepath.Join(dir, "keys"))

	methods := Resolve(context.Background(), Request{
		Context: core.CloudContext{Provider: "AWS"},
		VM:      core.VM{Name: "AWS-ME-MUM-Vfweb-server", ID: "i-123", PrivateIP: "10.10.122.105"},
	})

	for _, method := range methods {
		if strings.Contains(method.CopyText, "github-") || strings.Contains(method.Label, "github-") {
			t.Fatalf("unexpected unrelated ssh config method: %+v in %+v", method, methods)
		}
	}
}

func TestResolveIncludesPrivateKeyPickerAndBuildsKeyChoices(t *testing.T) {
	home := t.TempDir()
	sshDir := filepath.Join(home, ".ssh")
	keyDir := filepath.Join(home, "sshkeys")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(sshDir, "id_ed25519"),
		filepath.Join(keyDir, "prod.pem"),
		filepath.Join(keyDir, "prod.pem.pub"),
		filepath.Join(sshDir, "known_hosts"),
	} {
		if err := os.WriteFile(path, []byte("test"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("CLOUDMANAGER_SSH_CONFIG", filepath.Join(home, "missing-config"))

	methods := Resolve(context.Background(), Request{
		Context: core.CloudContext{Provider: "AWS"},
		VM:      core.VM{Name: "web", PrivateIP: "10.0.0.5"},
	})

	foundPicker := false
	for _, method := range methods {
		if method.Kind == "private_key_picker" {
			foundPicker = true
		}
		if strings.Contains(method.CopyText, "id_ed25519") || strings.Contains(method.CopyText, "prod.pem") {
			t.Fatalf("did not expect individual keys in top-level access list, got %+v", methods)
		}
	}
	if !foundPicker {
		t.Fatalf("expected private key picker, got %+v", methods)
	}

	keyMethods := PrivateKeySSHMethods(core.VM{Name: "web", PrivateIP: "10.0.0.5"}, "deploy")
	var foundDefaultKey, foundPem, foundPub bool
	for _, method := range keyMethods {
		switch {
		case strings.Contains(method.CopyText, "id_ed25519"):
			foundDefaultKey = strings.Contains(method.CopyText, "ssh -i ") && strings.Contains(method.CopyText, "deploy@10.0.0.5")
		case strings.Contains(method.CopyText, "prod.pem.pub"):
			foundPub = true
		case strings.Contains(method.CopyText, "prod.pem"):
			foundPem = strings.Contains(method.Label, "prod.pem via private IP")
		}
	}
	if !foundDefaultKey || !foundPem {
		t.Fatalf("expected discovered key methods from ~/.ssh and ~/sshkeys, got %+v", keyMethods)
	}
	if foundPub {
		t.Fatalf("did not expect public key file as an SSH identity, got %+v", keyMethods)
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
