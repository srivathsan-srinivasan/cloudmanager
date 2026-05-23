package hosts

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vyoogam/cloudmanager/internal/config"
	"github.com/vyoogam/cloudmanager/internal/core"
	"github.com/vyoogam/cloudmanager/internal/ui"
)

func TestFilterHostsMatchesNameIPUserAndTags(t *testing.T) {
	hosts := []core.ManualHost{
		{Name: "hetzner-web", Host: "203.0.113.10", Username: "root", Tags: []string{"prod"}},
		{Name: "lab-win", Host: "198.51.100.20", Username: "administrator", Tags: []string{"rdp"}},
	}

	for _, query := range []string{"hetzner", "203.0.113.10", "root", "prod"} {
		filtered := filterHosts(hosts, query)
		if len(filtered) != 1 || filtered[0].Name != "hetzner-web" {
			t.Fatalf("expected query %q to match hetzner-web, got %+v", query, filtered)
		}
	}
}

func TestHostsViewSearchQueryNarrowsRows(t *testing.T) {
	cfg := config.AppConfig{
		ManualHosts: []config.ManualHost{
			{Name: "hetzner-web", Host: "203.0.113.10", Username: "root"},
			{Name: "lab-win", Host: "198.51.100.20", Username: "administrator"},
		},
	}
	view := New(&cfg)
	view.Init(core.CloudContext{Provider: "Manual"}, 120, 30, false)
	view.SetSearchQuery("lab-win")

	rendered := view.Render()
	if !strings.Contains(rendered, "lab-win") {
		t.Fatalf("expected filtered host in render, got:\n%s", rendered)
	}
	if strings.Contains(rendered, "hetzner-web") {
		t.Fatalf("did not expect unfiltered host in render, got:\n%s", rendered)
	}
}

func TestHostsViewShowsReachabilityStatus(t *testing.T) {
	cfg := config.AppConfig{
		ManualHosts: []config.ManualHost{
			{Name: "hetzner-web", Host: "203.0.113.10", Username: "root"},
		},
	}
	view := New(&cfg)
	_ = view.Init(core.CloudContext{Provider: "Manual"}, 120, 30, false)
	if !strings.Contains(view.Render(), "checking") {
		t.Fatalf("expected initial reachability status, got:\n%s", view.Render())
	}

	updated, _ := view.Update(hostReachabilityMsg{key: manualHostKey(view.hosts[0]), status: "ssh-ok"})
	view = updated.(*HostsView)
	if !strings.Contains(view.Render(), "ssh-ok") {
		t.Fatalf("expected ssh reachability status, got:\n%s", view.Render())
	}
}

func TestHostsViewRemoveSelectedHostRequiresConfirmation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg := config.AppConfig{
		ManualHosts: []config.ManualHost{
			{Name: "hetzner-web", Provider: "Hetzner", Host: "203.0.113.10", Username: "root"},
			{Name: "lab-win", Provider: "Manual", Host: "198.51.100.20", Username: "administrator"},
		},
	}
	if err := config.Save(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	view := New(&cfg)
	view.Init(core.CloudContext{Provider: "Manual"}, 120, 30, false)

	updated, cmd := view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	view = updated.(*HostsView)
	if cmd != nil {
		t.Fatal("expected remove key to only open confirmation")
	}
	if !view.confirmRemove {
		t.Fatal("expected remove confirmation")
	}
	if len(cfg.ManualHosts) != 2 {
		t.Fatalf("did not expect host removal before confirmation, got %+v", cfg.ManualHosts)
	}
	if !strings.Contains(view.Render(), "Remove manual host?") {
		t.Fatalf("expected confirmation prompt in render, got:\n%s", view.Render())
	}

	updated, cmd = view.Update(tea.KeyMsg{Type: tea.KeyEnter})
	view = updated.(*HostsView)
	if cmd == nil {
		t.Fatal("expected manual host changed command")
	}
	msg := cmd()
	if _, ok := msg.(ui.ManualHostsChangedMsg); !ok {
		t.Fatalf("expected ManualHostsChangedMsg, got %T", msg)
	}
	if len(cfg.ManualHosts) != 1 || cfg.ManualHosts[0].Name != "lab-win" {
		t.Fatalf("expected selected host removed, got %+v", cfg.ManualHosts)
	}
	if strings.Contains(view.Render(), "hetzner-web") {
		t.Fatalf("did not expect removed host in render:\n%s", view.Render())
	}
}

func TestHostsViewRemoveCancelKeepsHost(t *testing.T) {
	cfg := config.AppConfig{
		ManualHosts: []config.ManualHost{
			{Name: "hetzner-web", Provider: "Hetzner", Host: "203.0.113.10", Username: "root"},
		},
	}
	view := New(&cfg)
	view.Init(core.CloudContext{Provider: "Manual"}, 120, 30, false)

	updated, _ := view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	view = updated.(*HostsView)
	updated, cmd := view.Update(tea.KeyMsg{Type: tea.KeyEsc})
	view = updated.(*HostsView)

	if cmd != nil {
		t.Fatal("did not expect command on cancel")
	}
	if view.confirmRemove {
		t.Fatal("expected confirmation to close")
	}
	if len(cfg.ManualHosts) != 1 {
		t.Fatalf("expected host to remain after cancel, got %+v", cfg.ManualHosts)
	}
}
