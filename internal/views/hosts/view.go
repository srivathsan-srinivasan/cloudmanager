package hosts

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"cloudmanager/internal/access"
	"cloudmanager/internal/config"
	"cloudmanager/internal/core"
	hostinventory "cloudmanager/internal/hosts"
	"cloudmanager/internal/ui"
)

type sshCompleteMsg struct{ err error }
type clipboardCompleteMsg struct{ err error }

type HostsView struct {
	table          table.Model
	cfg            *config.AppConfig
	hosts          []core.ManualHost
	visibleHosts   []core.ManualHost
	searchQuery    string
	activeCtx      core.CloudContext
	width, height  int
	statusMsg      string
	confirmRemove  bool
	pendingRemove  core.ManualHost
	canScrollLeft  bool
	canScrollRight bool
}

func New(cfg *config.AppConfig) *HostsView {
	return &HostsView{cfg: cfg}
}

func (v *HostsView) Title() string { return "Hosts" }
func (v *HostsView) ShortHelp() string {
	return "↑↓: Navigate • s/Enter: SSH • c: Copy SSH • d: Remove • r: Reload"
}
func (v *HostsView) IsInputActive() bool { return false }
func (v *HostsView) SetSearchQuery(query string) {
	v.searchQuery = strings.TrimSpace(query)
	v.refreshTable()
}

func (v *HostsView) Init(ctx core.CloudContext, width, height int, showSidebar bool) tea.Cmd {
	v.activeCtx = ctx
	v.width = width
	v.height = height
	v.hosts = hostinventory.FromConfig(*v.cfg)
	v.statusMsg = fmt.Sprintf("Loaded %d manual hosts.", len(v.hosts))
	v.refreshTable()
	return nil
}

func (v *HostsView) Resize(width, height int, showSidebar bool) {
	v.width = width
	v.height = height
	v.refreshTable()
}

func (v *HostsView) Update(msg tea.Msg) (ui.View, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if v.confirmRemove {
			return v.handleConfirmRemoveKeys(msg)
		}
		switch msg.String() {
		case "r":
			v.hosts = hostinventory.FromConfig(*v.cfg)
			v.statusMsg = fmt.Sprintf("Reloaded %d manual hosts.", len(v.hosts))
			v.refreshTable()
			return v, nil
		case "s", "enter":
			host, ok := v.selectedHost()
			if !ok {
				return v, nil
			}
			method, ok := firstRunnableMethod(host)
			if !ok {
				v.statusMsg = "No runnable SSH method for selected host."
				return v, nil
			}
			cmdExec, err := access.ExecCommand(context.Background(), method)
			if err != nil {
				v.statusMsg = fmt.Sprintf("SSH failed: %v", err)
				return v, nil
			}
			v.statusMsg = fmt.Sprintf("Starting %s...", method.Label)
			return v, tea.ExecProcess(cmdExec, func(err error) tea.Msg {
				return sshCompleteMsg{err: err}
			})
		case "c":
			host, ok := v.selectedHost()
			if !ok {
				return v, nil
			}
			method, ok := firstRunnableMethod(host)
			if !ok || strings.TrimSpace(method.CopyText) == "" {
				v.statusMsg = "Nothing to copy."
				return v, nil
			}
			return v, copyToClipboardCmd(method.CopyText)
		case "d":
			host, ok := v.selectedHost()
			if !ok {
				return v, nil
			}
			v.pendingRemove = host
			v.confirmRemove = true
			v.statusMsg = fmt.Sprintf("Confirm remove %s: Enter yes, Esc no.", host.ID())
			return v, nil
		}
		v.table, cmd = v.table.Update(msg)
		return v, cmd
	case sshCompleteMsg:
		if msg.err != nil {
			v.statusMsg = fmt.Sprintf("SSH failed: %v", msg.err)
		} else {
			v.statusMsg = "SSH session closed."
		}
	case clipboardCompleteMsg:
		if msg.err != nil {
			v.statusMsg = fmt.Sprintf("Copy failed: %v", msg.err)
		} else {
			v.statusMsg = "Copied SSH command."
		}
	}
	return v, nil
}

func (v *HostsView) handleConfirmRemoveKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	switch msg.String() {
	case "esc", "n", "N":
		v.confirmRemove = false
		v.pendingRemove = core.ManualHost{}
		v.statusMsg = "Remove canceled."
		return v, nil
	case "enter", "ctrl+j", "ctrl+m", "y", "Y":
		host := v.pendingRemove
		v.confirmRemove = false
		v.pendingRemove = core.ManualHost{}
		return v.removeHost(host)
	}
	return v, nil
}

func (v *HostsView) removeHost(host core.ManualHost) (ui.View, tea.Cmd) {
	if _, err := config.BackupConfig(); err != nil {
		v.statusMsg = fmt.Sprintf("Remove canceled; backup failed: %v", err)
		return v, nil
	}
	next := make([]config.ManualHost, 0, len(v.cfg.ManualHosts))
	removed := false
	for _, raw := range v.cfg.ManualHosts {
		if !removed && manualHostMatches(raw, host) {
			removed = true
			continue
		}
		next = append(next, raw)
	}
	if !removed {
		v.statusMsg = "Selected manual host was not found in config."
		return v, nil
	}
	v.cfg.ManualHosts = next
	if err := config.Save(*v.cfg); err != nil {
		v.statusMsg = fmt.Sprintf("Remove failed: %v", err)
		return v, nil
	}
	v.hosts = hostinventory.FromConfig(*v.cfg)
	v.statusMsg = fmt.Sprintf("Removed manual host %s.", host.ID())
	v.refreshTable()
	return v, func() tea.Msg {
		return ui.ManualHostsChangedMsg{Msg: v.statusMsg}
	}
}

func (v *HostsView) Render() string {
	header := ui.BreadcrumbStyle.Render("Manual Hosts")
	content := v.table.View()
	if len(v.visibleHosts) == 0 {
		content = lipgloss.NewStyle().Padding(2).Foreground(ui.Subtle).Render("No manual hosts. Use :add-host to add one.")
	}
	if v.confirmRemove {
		content = lipgloss.JoinVertical(lipgloss.Left,
			content,
			"",
			renderRemoveConfirm(v.pendingRemove),
		)
	}
	return ui.ClampToWindow(lipgloss.JoinVertical(lipgloss.Left, header, "", content), v.width, v.height)
}

func renderRemoveConfirm(host core.ManualHost) string {
	body := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Foreground(ui.Alert).Bold(true).Render("Remove manual host?"),
		fmt.Sprintf("%s  %s@%s", host.ID(), host.Username, host.Host),
		lipgloss.NewStyle().Foreground(ui.Subtle).Render("Enter/y: Remove • Esc/n: Cancel"),
	)
	return ui.OverlayStyle.Copy().BorderForeground(ui.Alert).Render(body)
}

func (v *HostsView) refreshTable() {
	if v.width == 0 {
		return
	}
	cols := []table.Column{
		{Title: "Name", Width: 24},
		{Title: "Host", Width: 20},
		{Title: "User", Width: 12},
		{Title: "Connection", Width: 10},
		{Title: "Provider", Width: 12},
		{Title: "Auth", Width: 18},
		{Title: "Tags", Width: 24},
	}
	tbl := table.New(
		table.WithColumns(cols),
		table.WithFocused(true),
		table.WithHeight(ui.TableHeight(v.height)),
		table.WithWidth(ui.TableViewportWidth(v.width)),
	)
	tbl.SetStyles(ui.DefaultTableStyles())
	v.visibleHosts = filterHosts(v.hosts, v.searchQuery)
	rows := make([]table.Row, 0, len(v.visibleHosts))
	for _, host := range v.visibleHosts {
		rows = append(rows, table.Row{
			host.Name,
			host.Host,
			host.Username,
			host.Connection,
			host.Provider,
			host.AuthSummary(),
			strings.Join(host.Tags, ","),
		})
	}
	tbl.SetRows(rows)
	v.table = tbl
}

func (v *HostsView) selectedHost() (core.ManualHost, bool) {
	idx := v.table.Cursor()
	if idx < 0 || idx >= len(v.visibleHosts) {
		return core.ManualHost{}, false
	}
	return v.visibleHosts[idx], true
}

func filterHosts(hosts []core.ManualHost, query string) []core.ManualHost {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return hosts
	}
	filtered := make([]core.ManualHost, 0, len(hosts))
	for _, host := range hosts {
		haystack := strings.ToLower(strings.Join([]string{
			host.Name,
			host.ID(),
			host.Host,
			host.Username,
			host.Provider,
			host.Connection,
			strings.Join(host.Tags, " "),
		}, " "))
		if strings.Contains(haystack, query) {
			filtered = append(filtered, host)
		}
	}
	return filtered
}

func manualHostMatches(raw config.ManualHost, host core.ManualHost) bool {
	raw = config.SanitizeManualHost(raw)
	return raw.Name == host.Name &&
		raw.Provider == host.Provider &&
		raw.Host == host.Host &&
		raw.Username == host.Username &&
		raw.Connection == host.Connection &&
		raw.SSHConfigHost == host.SSHConfigHost &&
		raw.KeyPath == host.KeyPath
}

func firstRunnableMethod(host core.ManualHost) (core.AccessMethod, bool) {
	ctx := core.CloudContext{Provider: "Manual", AccountID: "manual-hosts", AccountName: "Manual Hosts", Region: "global"}
	methods := access.Resolve(context.Background(), access.Request{Context: ctx, VM: host.ToVM(), ManualHost: &host})
	for _, method := range methods {
		if method.Available && len(method.Command) > 0 {
			return method, true
		}
	}
	return core.AccessMethod{}, false
}

func copyToClipboardCmd(text string) tea.Cmd {
	return func() tea.Msg {
		args, ok := clipboardCommand()
		if !ok {
			return clipboardCompleteMsg{err: fmt.Errorf("clipboard command not found")}
		}
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err != nil {
			return clipboardCompleteMsg{err: err}
		}
		return clipboardCompleteMsg{}
	}
}

func clipboardCommand() ([]string, bool) {
	candidates := [][]string{{"pbcopy"}, {"wl-copy"}, {"xclip", "-selection", "clipboard"}, {"xsel", "--clipboard", "--input"}}
	for _, candidate := range candidates {
		if _, err := exec.LookPath(candidate[0]); err == nil {
			return candidate, true
		}
	}
	return nil, false
}
