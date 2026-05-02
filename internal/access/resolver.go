package access

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"cloudmanager/internal/core"
)

type Request struct {
	Context      core.CloudContext
	VM           core.VM
	ManualHost   *core.ManualHost
	NativeMethod *core.AccessMethod
}

func Resolve(ctx context.Context, req Request) []core.AccessMethod {
	_ = ctx
	var methods []core.AccessMethod
	if req.NativeMethod != nil {
		methods = append(methods, *req.NativeMethod)
	}
	methods = append(methods, manualHostMethods(req)...)
	if req.ManualHost != nil {
		sortAccessMethods(methods)
		return methods
	}
	methods = append(methods, sshConfigMethods(req)...)
	methods = append(methods, directSSHMethods(req)...)
	methods = append(methods, remediationMethod(req))
	sortAccessMethods(methods)
	return methods
}

func sortAccessMethods(methods []core.AccessMethod) {
	sort.SliceStable(methods, func(i, j int) bool {
		if methods[i].Available != methods[j].Available {
			return methods[i].Available
		}
		return methods[i].Priority < methods[j].Priority
	})
}

func manualHostMethods(req Request) []core.AccessMethod {
	if req.ManualHost == nil {
		return nil
	}
	host := *req.ManualHost
	if strings.ToLower(strings.TrimSpace(host.Connection)) != "ssh" {
		return []core.AccessMethod{{
			ID:        "manual-unsupported",
			Kind:      "manual",
			Label:     "Manual " + strings.ToUpper(strings.TrimSpace(host.Connection)) + " access",
			Priority:  30,
			Available: false,
			Reason:    "Only SSH manual hosts are runnable today.",
		}}
	}
	if alias := strings.TrimSpace(host.SSHConfigHost); alias != "" {
		args := []string{"ssh", alias}
		return []core.AccessMethod{{
			ID:        "manual-ssh-config",
			Kind:      "ssh_config",
			Label:     "SSH config: " + alias,
			Priority:  15,
			Command:   args,
			CopyText:  FormatCommand(args),
			Available: true,
		}}
	}
	target := manualSSHTarget(host)
	if target == "" {
		return nil
	}
	args := []string{"ssh"}
	if keyPath := strings.TrimSpace(host.KeyPath); keyPath != "" {
		args = append(args, "-i", expandPath(keyPath))
	}
	args = append(args, target)
	return []core.AccessMethod{{
		ID:        "manual-ssh",
		Kind:      "ssh",
		Label:     "Manual SSH",
		Priority:  20,
		Command:   args,
		CopyText:  FormatCommand(args),
		Available: true,
	}}
}

func manualSSHTarget(host core.ManualHost) string {
	address := strings.TrimSpace(host.Host)
	if address == "" {
		return ""
	}
	if user := strings.TrimSpace(host.Username); user != "" {
		return user + "@" + address
	}
	return address
}

func sshConfigMethods(req Request) []core.AccessMethod {
	entries, err := ParseSSHConfig(defaultSSHConfigPath())
	if err != nil {
		return nil
	}
	var out []core.AccessMethod
	seen := map[string]bool{}
	for _, entry := range entries {
		if !entryMatchesVM(entry, req.VM) {
			continue
		}
		alias := entry.PrimaryAlias()
		if alias == "" || seen[alias] {
			continue
		}
		seen[alias] = true
		args := []string{"ssh", alias}
		out = append(out, core.AccessMethod{
			ID:        "ssh-config-" + alias,
			Kind:      "ssh_config",
			Label:     "SSH config: " + alias,
			Priority:  20,
			Command:   args,
			CopyText:  FormatCommand(args),
			Available: true,
		})
	}
	return out
}

func directSSHMethods(req Request) []core.AccessMethod {
	target := firstUsableIP(req.VM.PublicIP, req.VM.PrivateIP)
	if target == "" {
		return nil
	}
	args := []string{"ssh", target}
	return []core.AccessMethod{{
		ID:        "direct-ssh",
		Kind:      "ssh",
		Label:     "Direct SSH with default identity",
		Priority:  40,
		Command:   args,
		CopyText:  FormatCommand(args),
		Available: true,
	}}
}

func remediationMethod(req Request) core.AccessMethod {
	return core.AccessMethod{
		ID:        "remediation",
		Kind:      "remediation",
		Label:     "Access remediation guide",
		Priority:  90,
		CopyText:  "No direct command is available. Use provider metadata or authorized_keys to add a reachable SSH key.",
		Available: true,
	}
}

func ExecCommand(ctx context.Context, method core.AccessMethod) (*exec.Cmd, error) {
	if len(method.Command) == 0 {
		return nil, fmt.Errorf("access method has no command")
	}
	return exec.CommandContext(ctx, method.Command[0], method.Command[1:]...), nil
}

func FormatCommand(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, shellQuote(arg))
	}
	return strings.Join(quoted, " ")
}

func shellQuote(arg string) string {
	if arg == "" {
		return "''"
	}
	if !strings.ContainsAny(arg, " \t\n'\"\\$&;|<>`(){}[]*?!") {
		return arg
	}
	return "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
}

func firstUsableIP(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || value == "-" {
			continue
		}
		return value
	}
	return ""
}

func defaultSSHConfigPath() string {
	if path := strings.TrimSpace(os.Getenv("CLOUDMANAGER_SSH_CONFIG")); path != "" {
		return expandPath(path)
	}
	return expandPath("~/.ssh/config")
}

func expandPath(path string) string {
	path = os.ExpandEnv(strings.TrimSpace(path))
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}
