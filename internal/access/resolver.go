package access

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/srivathsan-srinivasan/cloudmanager/internal/core"
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
	var methods []core.AccessMethod
	keyCount := len(discoverSSHKeyFiles())
	if keyCount > 0 {
		methods = append(methods, core.AccessMethod{
			ID:        "private-key-ssh",
			Kind:      "private_key_picker",
			Label:     fmt.Sprintf("Private key SSH (%d keys)", keyCount),
			Priority:  35,
			CopyText:  "Choose key, username, and public/private IP.",
			Available: true,
		})
	}
	args := []string{"ssh", target}
	methods = append(methods, core.AccessMethod{
		ID:        "direct-ssh",
		Kind:      "ssh",
		Label:     "Direct SSH with default identity",
		Priority:  60,
		Command:   args,
		CopyText:  FormatCommand(args),
		Available: true,
	})
	return methods
}

func PrivateKeySSHMethods(vm core.VM, username string) []core.AccessMethod {
	username = strings.TrimSpace(username)
	if username == "" {
		username = "ubuntu"
	}
	targets := sshTargets(vm, username)
	if len(targets) == 0 {
		return nil
	}
	var methods []core.AccessMethod
	for _, keyPath := range discoverSSHKeyFiles() {
		for _, target := range targets {
			args := []string{"ssh", "-i", keyPath, target.target}
			methods = append(methods, core.AccessMethod{
				ID:        "direct-ssh-key-" + filepath.Base(keyPath) + "-" + target.kind,
				Kind:      "ssh",
				Label:     fmt.Sprintf("%s via %s IP", filepath.Base(keyPath), target.kind),
				Priority:  35,
				Command:   args,
				CopyText:  FormatCommand(args),
				Available: true,
			})
		}
	}
	return methods
}

func SSHKeyFiles() []string {
	return discoverSSHKeyFiles()
}

func PrivateKeySSHMethod(vm core.VM, username, keyPath, ipKind string) (core.AccessMethod, bool) {
	keyPath = strings.TrimSpace(keyPath)
	if keyPath == "" {
		return core.AccessMethod{}, false
	}
	username = strings.TrimSpace(username)
	if username == "" {
		username = "ubuntu"
	}
	var ip string
	switch strings.ToLower(strings.TrimSpace(ipKind)) {
	case "private":
		ip = strings.TrimSpace(vm.PrivateIP)
	default:
		ip = strings.TrimSpace(vm.PublicIP)
		ipKind = "public"
	}
	if ip == "" || ip == "-" {
		return core.AccessMethod{}, false
	}
	target := ip
	if username != "" {
		target = username + "@" + ip
	}
	args := []string{"ssh", "-i", keyPath, target}
	return core.AccessMethod{
		ID:        "direct-ssh-key-" + filepath.Base(keyPath) + "-" + ipKind,
		Kind:      "ssh",
		Label:     fmt.Sprintf("%s via %s IP", filepath.Base(keyPath), ipKind),
		Priority:  35,
		Command:   args,
		CopyText:  FormatCommand(args),
		Available: true,
	}, true
}

type sshTarget struct {
	kind   string
	target string
}

func sshTargets(vm core.VM, username string) []sshTarget {
	var targets []sshTarget
	seen := map[string]bool{}
	add := func(kind, ip string) {
		ip = strings.TrimSpace(ip)
		if ip == "" || ip == "-" || seen[ip] {
			return
		}
		seen[ip] = true
		target := ip
		if username != "" {
			target = username + "@" + ip
		}
		targets = append(targets, sshTarget{kind: kind, target: target})
	}
	add("public", vm.PublicIP)
	add("private", vm.PrivateIP)
	return targets
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

func discoverSSHKeyFiles() []string {
	var dirs []string
	if configured := strings.TrimSpace(os.Getenv("CLOUDMANAGER_SSH_KEY_DIRS")); configured != "" {
		dirs = filepath.SplitList(configured)
	} else {
		dirs = []string{"~/.ssh", "~/sshkeys"}
	}

	seen := map[string]bool{}
	var keys []string
	for _, dir := range dirs {
		dir = expandPath(dir)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !looksLikePrivateKey(name) {
				continue
			}
			path := filepath.Join(dir, name)
			if seen[path] {
				continue
			}
			seen[path] = true
			keys = append(keys, path)
		}
	}
	sort.Strings(keys)
	return keys
}

func looksLikePrivateKey(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	if lower == "" || strings.HasSuffix(lower, ".pub") {
		return false
	}
	switch lower {
	case "config", "known_hosts", "known_hosts.old", "authorized_keys":
		return false
	}
	return lower == "id_rsa" ||
		lower == "id_ecdsa" ||
		lower == "id_ed25519" ||
		strings.HasSuffix(lower, ".pem") ||
		strings.HasSuffix(lower, ".key")
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
