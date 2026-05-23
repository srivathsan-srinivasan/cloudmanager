package hosts

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vyoogam/cloudmanager/internal/access"
	"github.com/vyoogam/cloudmanager/internal/config"
	"github.com/vyoogam/cloudmanager/internal/core"
	hostinventory "github.com/vyoogam/cloudmanager/internal/hosts"
	"github.com/vyoogam/cloudmanager/internal/ui"
)

type sshCompleteMsg struct{ err error }
type clipboardCompleteMsg struct{ err error }
type hostReachabilityMsg struct {
	key    string
	status string
}

var hostExecCommandContext = exec.CommandContext

type HostsView struct {
	table          table.Model
	sortList       list.Model
	cfg            *config.AppConfig
	hosts          []core.ManualHost
	visibleHosts   []core.ManualHost
	reachability   map[string]string
	searchQuery    string
	activeCtx      core.CloudContext
	width, height  int
	statusMsg      string
	confirmRemove  bool
	pendingRemove  core.ManualHost
	canScrollLeft  bool
	canScrollRight bool
	showSort       bool
	sortColumn     string
	sortAsc        bool
	sortHeader     ui.HeaderSortState
}

func New(cfg *config.AppConfig) *HostsView {
	return &HostsView{
		cfg:          cfg,
		reachability: map[string]string{},
		sortList:     ui.NewSortList("Sort Hosts by (Enter to select, Esc to cancel)", hostSortColumns()),
		sortAsc:      true,
	}
}

func (v *HostsView) Title() string { return "Hosts" }
func (v *HostsView) ShortHelp() string {
	return "↑↓: Navigate • ↑ at top: Columns • Enter: Sort/SSH • c: Copy SSH • d: Remove • r: Reload + retest"
}
func (v *HostsView) IsInputActive() bool { return v.sortHeader.Active || v.showSort }
func (v *HostsView) SetSearchQuery(query string) {
	v.searchQuery = strings.TrimSpace(query)
	v.refreshTable()
}

func (v *HostsView) Init(ctx core.CloudContext, width, height int, showSidebar bool) tea.Cmd {
	v.activeCtx = ctx
	v.width = width
	v.height = height
	v.hosts = hostinventory.FromConfig(*v.cfg)
	v.markReachabilityChecking()
	v.statusMsg = fmt.Sprintf("Loaded %d manual hosts.", len(v.hosts))
	v.refreshTable()
	return v.reachabilityCmd()
}

func (v *HostsView) Resize(width, height int, showSidebar bool) {
	v.width = width
	v.height = height
	v.sortList.SetSize(width-4, height-4)
	v.refreshTable()
}

func (v *HostsView) Update(msg tea.Msg) (ui.View, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if v.confirmRemove {
			return v.handleConfirmRemoveKeys(msg)
		}
		if v.sortHeader.Active {
			return v.handleHeaderSortKeys(msg)
		}
		if v.showSort {
			return v.handleSortKeys(msg)
		}
		switch msg.String() {
		case "r":
			v.hosts = hostinventory.FromConfig(*v.cfg)
			v.markReachabilityChecking()
			v.statusMsg = fmt.Sprintf("Reloaded %d manual hosts; testing reachability.", len(v.hosts))
			v.refreshTable()
			return v, v.reachabilityCmd()
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
		case "c", "C":
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
		case "S":
			v.sortHeader.Activate(hostColumns())
			v.refreshTable()
			return v, nil
		case "up":
			if v.table.Cursor() == 0 {
				v.sortHeader.Activate(hostColumns())
				v.refreshTable()
				return v, nil
			}
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
	case hostReachabilityMsg:
		if v.reachability == nil {
			v.reachability = map[string]string{}
		}
		v.reachability[msg.key] = msg.status
		v.refreshTable()
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
	if v.showSort {
		return ui.ClampToWindow(v.sortList.View(), v.width, v.height)
	}
	content := ui.ColorizeOperationalStates(v.table.View())
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
	cursor := v.table.Cursor()
	cols := hostColumns()
	tbl := table.New(
		table.WithColumns(cols),
		table.WithFocused(true),
		table.WithHeight(ui.TableHeight(v.height)),
		table.WithWidth(ui.TableViewportWidth(v.width)),
	)
	tbl.SetStyles(ui.DefaultTableStyles())
	v.visibleHosts = filterHosts(v.hosts, v.searchQuery)
	ui.SortByColumn(v.visibleHosts, v.sortColumn, v.sortAsc, func(host core.ManualHost, column string) string {
		return v.hostSortField(host, column)
	})
	rows := make([]table.Row, 0, len(v.visibleHosts))
	for _, host := range v.visibleHosts {
		rows = append(rows, table.Row{
			host.Name,
			host.Host,
			v.hostStatus(host),
			host.Username,
			host.Connection,
			host.Provider,
			host.AuthSummary(),
			strings.Join(host.Tags, ","),
		})
	}
	tbl.SetRows(rows)
	tbl.SetColumns(ui.DecorateSortColumns(cols, v.sortHeader, v.sortColumn, v.sortAsc))
	if cursor >= len(rows) {
		cursor = len(rows) - 1
	}
	if cursor < 0 {
		cursor = 0
	}
	tbl.SetCursor(cursor)
	v.table = tbl
}

func (v *HostsView) handleHeaderSortKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	cols := hostColumns()
	switch msg.String() {
	case "esc", "down":
		v.sortHeader.Deactivate()
	case "left", "h":
		v.sortHeader.Move(-1, cols)
	case "right", "l":
		v.sortHeader.Move(1, cols)
	case "enter":
		column := v.sortHeader.SelectedColumn(cols)
		v.sortColumn, v.sortAsc = ui.ToggleSortColumn(v.sortColumn, v.sortAsc, column)
		v.statusMsg = fmt.Sprintf("Sorted by %s (%s).", v.sortColumn, ui.SortDirectionLabel(v.sortAsc))
	}
	v.refreshTable()
	return v, nil
}

func (v *HostsView) handleSortKeys(msg tea.KeyMsg) (ui.View, tea.Cmd) {
	switch msg.String() {
	case "esc":
		v.showSort = false
		return v, nil
	case "enter":
		selected, ok := v.sortList.SelectedItem().(ui.SortColumnItem)
		if !ok {
			return v, nil
		}
		v.sortColumn, v.sortAsc = ui.ToggleSortColumn(v.sortColumn, v.sortAsc, selected.Name)
		v.refreshTable()
		v.showSort = false
		v.statusMsg = fmt.Sprintf("Sorted by %s (%s).", v.sortColumn, ui.SortDirectionLabel(v.sortAsc))
		return v, nil
	default:
		var cmd tea.Cmd
		v.sortList, cmd = v.sortList.Update(msg)
		return v, cmd
	}
}

func hostSortColumns() []table.Column {
	return hostColumns()
}

func hostColumns() []table.Column {
	return []table.Column{
		{Title: "Name", Width: 24},
		{Title: "Host", Width: 20},
		{Title: "Status", Width: 12},
		{Title: "User", Width: 12},
		{Title: "Connection", Width: 10},
		{Title: "Provider", Width: 12},
		{Title: "Auth", Width: 18},
		{Title: "Tags", Width: 24},
	}
}

func hostSortItems(columns []table.Column) []list.Item {
	items := make([]list.Item, 0, len(columns))
	for _, col := range columns {
		items = append(items, ui.SortColumnItem{Name: col.Title})
	}
	return items
}

func (v *HostsView) hostSortField(host core.ManualHost, column string) string {
	switch column {
	case "Name":
		return host.Name
	case "Host":
		return host.Host
	case "Status":
		return v.hostStatus(host)
	case "User":
		return host.Username
	case "Connection":
		return host.Connection
	case "Provider":
		return host.Provider
	case "Auth":
		return host.AuthSummary()
	case "Tags":
		return strings.Join(host.Tags, ",")
	default:
		return ""
	}
}

func (v *HostsView) markReachabilityChecking() {
	v.reachability = map[string]string{}
	for _, host := range v.hosts {
		v.reachability[manualHostKey(host)] = "checking"
	}
}

func (v *HostsView) hostStatus(host core.ManualHost) string {
	if v.reachability == nil {
		return "unknown"
	}
	status := strings.TrimSpace(v.reachability[manualHostKey(host)])
	if status == "" {
		return "unknown"
	}
	return status
}

func (v *HostsView) reachabilityCmd() tea.Cmd {
	if len(v.hosts) == 0 {
		return nil
	}
	cmds := make([]tea.Cmd, 0, len(v.hosts))
	for _, host := range v.hosts {
		host := host
		cmds = append(cmds, func() tea.Msg {
			return hostReachabilityMsg{key: manualHostKey(host), status: checkManualHostReachability(host)}
		})
	}
	return tea.Batch(cmds...)
}

func manualHostKey(host core.ManualHost) string {
	return strings.Join([]string{host.Name, host.Username, host.Host, host.Connection, host.SSHConfigHost}, "\x00")
}

func checkManualHostReachability(host core.ManualHost) string {
	if strings.EqualFold(strings.TrimSpace(host.Connection), "ssh") {
		if status := checkManualHostSSH(host); status != "" {
			return status
		}
	}
	if pingManualHost(host.Host) {
		return "ping-ok"
	}
	return "unreachable"
}

func checkManualHostSSH(host core.ManualHost) string {
	target := strings.TrimSpace(host.SSHConfigHost)
	if target == "" {
		target = manualSSHTarget(host)
	}
	if target == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	args := []string{
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=3",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		target,
		"true",
	}
	out, err := hostExecCommandContext(ctx, "ssh", args...).CombinedOutput()
	if err == nil {
		return "ssh-ok"
	}
	lower := strings.ToLower(string(out))
	if strings.Contains(lower, "permission denied") || strings.Contains(lower, "authentication") {
		return "reachable"
	}
	return ""
}

func pingManualHost(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return hostExecCommandContext(ctx, "ping", "-c", "1", host).Run() == nil
}

func manualSSHTarget(host core.ManualHost) string {
	address := strings.TrimSpace(host.Host)
	if address == "" {
		return ""
	}
	user := strings.TrimSpace(host.Username)
	if user == "" {
		return address
	}
	return user + "@" + address
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
