package networks

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vyoogam/cloudmanager/internal/config"
	"github.com/vyoogam/cloudmanager/internal/core"
)

func TestNetworkTableArrowKeysMoveSelectionOnce(t *testing.T) {
	view := New(&config.AppConfig{})
	view.Resize(100, 30, true)
	view.networkData = []core.Network{
		{Name: "alpha", ID: "net-1"},
		{Name: "beta", ID: "net-2"},
		{Name: "gamma", ID: "net-3"},
	}
	view.syncVisibleRows()
	view.networks.Focus()

	updated, _ := view.Update(tea.KeyMsg{Type: tea.KeyDown})
	next := updated.(*NetworksView)

	if got := next.networks.Cursor(); got != 1 {
		t.Fatalf("expected cursor to move to row 1 after down key, got %d", got)
	}
}

func TestSubnetTableArrowKeysMoveSelectionOnce(t *testing.T) {
	view := New(&config.AppConfig{})
	view.Resize(100, 30, true)
	view.activePane = paneSubnets
	view.subnetData = []core.Subnet{
		{Name: "alpha", ID: "subnet-1"},
		{Name: "beta", ID: "subnet-2"},
		{Name: "gamma", ID: "subnet-3"},
	}
	view.syncVisibleSubnets()
	view.subnets.Focus()

	updated, _ := view.Update(tea.KeyMsg{Type: tea.KeyDown})
	next := updated.(*NetworksView)

	if got := next.subnets.Cursor(); got != 1 {
		t.Fatalf("expected subnet cursor to move to row 1 after down key, got %d", got)
	}
}

func TestSetSearchQueryFiltersNetworks(t *testing.T) {
	view := New(&config.AppConfig{})
	view.Resize(100, 30, true)
	view.networkData = []core.Network{
		{Name: "alpha", ID: "net-1"},
		{Name: "beta", ID: "net-2"},
	}

	view.SetSearchQuery("net-2")

	if view.activePane != paneTable {
		t.Fatalf("expected network table pane, got %d", view.activePane)
	}
	if len(view.visibleRows) != 1 || view.visibleRows[0].ID != "net-2" {
		t.Fatalf("expected search handoff to filter to net-2, got %+v", view.visibleRows)
	}
}

func TestSetSearchQueryCanOpenSubnetResults(t *testing.T) {
	view := New(&config.AppConfig{})
	view.Resize(100, 30, true)
	view.subnetData = []core.Subnet{
		{Name: "alpha", ID: "subnet-1", NetworkID: "net-1"},
		{Name: "beta", ID: "subnet-2", NetworkID: "net-2"},
	}

	view.SetSearchQuery("subnet:subnet-2")

	if view.activePane != paneSubnets {
		t.Fatalf("expected subnet pane, got %d", view.activePane)
	}
	if len(view.visibleSubnets) != 1 || view.visibleSubnets[0].ID != "subnet-2" {
		t.Fatalf("expected search handoff to filter to subnet-2, got %+v", view.visibleSubnets)
	}
}

func TestDescribePaneCopiesNetworkDetails(t *testing.T) {
	view := New(&config.AppConfig{})
	view.activePane = paneDescribe
	view.copyableText = "network details"

	_, cmd := view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})

	if cmd == nil {
		t.Fatal("expected copy command for network describe pane")
	}
}
