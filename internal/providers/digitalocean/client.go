package digitalocean

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/vyoogam/cloudmanager/internal/core"
)

func FetchVMsCLI(ctx context.Context, cloudCtx core.CloudContext) ([]core.VM, error) {
	args := []string{
		"compute", "droplet", "list",
		"--format", "ID,Name,PublicIPv4,PrivateIPv4,Region,Status,Memory,VCPUs,Image",
		"--no-header",
	}
	if strings.TrimSpace(cloudCtx.Region) != "" && !strings.EqualFold(cloudCtx.Region, "global") {
		args = append(args, "--region", cloudCtx.Region)
	}

	cmd := exec.CommandContext(ctx, "doctl", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("doctl droplet list failed: %w\n%s", err, strings.TrimSpace(string(output)))
	}

	var vms []core.VM
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 8 {
			continue
		}

		publicIP := "-"
		privateIP := "-"
		if len(fields) > 2 && fields[2] != "" {
			publicIP = fields[2]
		}
		if len(fields) > 3 && fields[3] != "" {
			privateIP = fields[3]
		}

		vms = append(vms, core.VM{
			ID:        fields[0],
			Name:      fields[1],
			PublicIP:  publicIP,
			PrivateIP: privateIP,
			Zone:      fields[4],
			State:     normalizeState(fields[5]),
			Type:      dropletSize(fields),
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to parse droplet list: %w", err)
	}
	return vms, nil
}

func ExecuteActionCLI(ctx context.Context, action string, vm core.VM, cloudCtx core.CloudContext) (string, error) {
	var cmd *exec.Cmd
	switch action {
	case "Start":
		cmd = exec.CommandContext(ctx, "doctl", "compute", "droplet-action", "power-on", vm.ID)
	case "Stop":
		cmd = exec.CommandContext(ctx, "doctl", "compute", "droplet-action", "shutdown", vm.ID)
	case "Restart":
		cmd = exec.CommandContext(ctx, "doctl", "compute", "droplet-action", "reboot", vm.ID)
	case "Terminate":
		cmd = exec.CommandContext(ctx, "doctl", "compute", "droplet", "delete", vm.ID, "--force")
	case "Describe":
		cmd = exec.CommandContext(ctx, "doctl", "compute", "droplet", "get", vm.ID)
	default:
		return "", fmt.Errorf("action %s not supported for DigitalOcean", action)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to %s droplet: %w\n%s", action, err, strings.TrimSpace(string(output)))
	}
	if action == "Describe" {
		return string(output), nil
	}
	return fmt.Sprintf("Successfully executed '%s' on %s", action, vm.Name), nil
}

func GetSSHCmdCLI(ctx context.Context, vm core.VM, cloudCtx core.CloudContext) (*exec.Cmd, error) {
	target := strings.TrimSpace(vm.PublicIP)
	if target == "" || target == "-" {
		target = strings.TrimSpace(vm.PrivateIP)
	}
	if target == "" || target == "-" {
		return nil, fmt.Errorf("no reachable IP found for %s", vm.Name)
	}
	return exec.CommandContext(ctx, "ssh", target), nil
}

func normalizeState(state string) string {
	state = strings.TrimSpace(strings.ToLower(state))
	switch state {
	case "active":
		return "running"
	case "":
		return "-"
	default:
		return state
	}
}

func dropletSize(fields []string) string {
	if len(fields) < 8 {
		return "-"
	}
	vcpus := fields[7]
	memory := "-"
	if len(fields) > 6 {
		memory = fields[6]
	}
	if memory == "" {
		memory = "-"
	}
	if vcpus == "" {
		vcpus = "-"
	}
	return fmt.Sprintf("%svCPU/%sMB", vcpus, memory)
}

// --- Port Forwarding & SCP ---

// GetPortForwardCmdCLI returns a standard SSH command with -L flags for port forwarding.
// DigitalOcean uses standard SSH (no special tunneling).
func GetPortForwardCmdCLI(ctx context.Context, vm core.VM, cloudCtx core.CloudContext, specs []core.PortForwardSpec) (*exec.Cmd, error) {
	if len(specs) == 0 {
		return nil, fmt.Errorf("no port forward specs provided")
	}
	target := strings.TrimSpace(vm.PublicIP)
	if target == "" || target == "-" {
		target = strings.TrimSpace(vm.PrivateIP)
	}
	if target == "" || target == "-" {
		return nil, fmt.Errorf("no reachable IP found for %s", vm.Name)
	}
	args := []string{"ssh"}
	for _, s := range specs {
		localHost := s.LocalHost
		if localHost == "" {
			localHost = "localhost"
		}
		remoteHost := s.RemoteHost
		if remoteHost == "" {
			remoteHost = "localhost"
		}
		args = append(args, "-L", fmt.Sprintf("%s:%d:%s:%d", localHost, s.LocalPort, remoteHost, s.RemotePort))
	}
	args = append(args, target)
	return exec.CommandContext(ctx, args[0], args[1:]...), nil
}

// GetPortForwardCmdSDK delegates to CLI for port forwarding.
func GetPortForwardCmdSDK(ctx context.Context, vm core.VM, cloudCtx core.CloudContext, specs []core.PortForwardSpec) (*exec.Cmd, error) {
	return GetPortForwardCmdCLI(ctx, vm, cloudCtx, specs)
}

// GetSCPCmdCLI returns a standard SCP command for file transfer.
func GetSCPCmdCLI(ctx context.Context, vm core.VM, cloudCtx core.CloudContext, transfer core.SCPTransfer) (*exec.Cmd, error) {
	target := strings.TrimSpace(vm.PublicIP)
	if target == "" || target == "-" {
		target = strings.TrimSpace(vm.PrivateIP)
	}
	if target == "" || target == "-" {
		return nil, fmt.Errorf("no reachable IP found for %s", vm.Name)
	}
	var args []string
	if transfer.Direction == "pull" {
		// remote -> local
		args = []string{"scp"}
		if transfer.Recursive {
			args = append(args, "-r")
		}
		args = append(args, fmt.Sprintf("%s:%s", target, transfer.Source), transfer.Destination)
	} else {
		// local -> remote
		args = []string{"scp"}
		if transfer.Recursive {
			args = append(args, "-r")
		}
		args = append(args, transfer.Source, fmt.Sprintf("%s:%s", target, transfer.Destination))
	}
	return exec.CommandContext(ctx, args[0], args[1:]...), nil
}

// GetSCPCmdSDK delegates to CLI for SCP.
func GetSCPCmdSDK(ctx context.Context, vm core.VM, cloudCtx core.CloudContext, transfer core.SCPTransfer) (*exec.Cmd, error) {
	return GetSCPCmdCLI(ctx, vm, cloudCtx, transfer)
}
