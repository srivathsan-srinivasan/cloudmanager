package main

import (
	"fmt"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
)

type describeCompleteMsg struct {
	output string
	err    error
}

func executeActionCmd(action string, vm VM, ctx contextItem) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		var err error

		switch ctx.provider {
		case "AWS":
			switch action {
			case "Start":
				cmd = exec.Command("aws", "ec2", "start-instances", "--instance-ids", vm.ID, "--profile", ctx.accountName, "--region", ctx.region)
			case "Stop":
				cmd = exec.Command("aws", "ec2", "stop-instances", "--instance-ids", vm.ID, "--profile", ctx.accountName, "--region", ctx.region)
			case "Restart":
				cmd = exec.Command("aws", "ec2", "reboot-instances", "--instance-ids", vm.ID, "--profile", ctx.accountName, "--region", ctx.region)
			case "Terminate":
				cmd = exec.Command("aws", "ec2", "terminate-instances", "--instance-ids", vm.ID, "--profile", ctx.accountName, "--region", ctx.region)
			case "Describe":
				cmd = exec.Command("aws", "ec2", "describe-instances", "--instance-ids", vm.ID, "--profile", ctx.accountName, "--region", ctx.region)
			}
		case "GCP":
			switch action {
			case "Start":
				cmd = exec.Command("gcloud", "compute", "instances", "start", vm.Name, "--project", ctx.accountID, "--zone", vm.Zone)
			case "Stop":
				cmd = exec.Command("gcloud", "compute", "instances", "stop", vm.Name, "--project", ctx.accountID, "--zone", vm.Zone)
			case "Restart":
				cmd = exec.Command("gcloud", "compute", "instances", "reset", vm.Name, "--project", ctx.accountID, "--zone", vm.Zone)
			case "Terminate":
				cmd = exec.Command("gcloud", "compute", "instances", "delete", vm.Name, "--project", ctx.accountID, "--zone", vm.Zone, "--quiet")
			case "Describe":
				cmd = exec.Command("gcloud", "compute", "instances", "describe", vm.Name, "--project", ctx.accountID, "--zone", vm.Zone)
			}
		case "Azure":
			switch action {
			case "Start":
				cmd = exec.Command("az", "vm", "start", "--name", vm.Name, "--resource-group", vm.ResourceGroup, "--subscription", ctx.accountID)
			case "Stop":
				cmd = exec.Command("az", "vm", "stop", "--name", vm.Name, "--resource-group", vm.ResourceGroup, "--subscription", ctx.accountID)
			case "Restart":
				cmd = exec.Command("az", "vm", "restart", "--name", vm.Name, "--resource-group", vm.ResourceGroup, "--subscription", ctx.accountID)
			case "Terminate":
				cmd = exec.Command("az", "vm", "delete", "--name", vm.Name, "--resource-group", vm.ResourceGroup, "--subscription", ctx.accountID, "--yes")
			case "Describe":
				cmd = exec.Command("az", "vm", "show", "--name", vm.Name, "--resource-group", vm.ResourceGroup, "--subscription", ctx.accountID)
			}
		}

		if cmd == nil {
			return commandCompleteMsg{err: fmt.Errorf("action %s not supported for %s", action, ctx.provider)}
		}

		output, err := cmd.CombinedOutput()
		
		if action == "Describe" {
			return describeCompleteMsg{output: string(output), err: err}
		}

		if err != nil {
			return commandCompleteMsg{err: fmt.Errorf("failed to %s: %w\n%s", action, err, string(output))}
		}

		return commandCompleteMsg{output: fmt.Sprintf("Successfully executed '%s' on %s", action, vm.Name), err: nil}
	}
}

func createSSHCmd(vm VM, ctx contextItem) *exec.Cmd {
	switch ctx.provider {
	case "AWS":
		// Requires Session Manager plugin to be installed locally
		return exec.Command("aws", "ssm", "start-session", "--target", vm.ID, "--profile", ctx.accountName, "--region", ctx.region)
	case "GCP":
		return exec.Command("gcloud", "compute", "ssh", vm.Name, "--project", ctx.accountID, "--zone", vm.Zone)
	case "Azure":
		return exec.Command("az", "ssh", "vm", "--name", vm.Name, "--resource-group", vm.ResourceGroup, "--subscription", ctx.accountID)
	}
	return nil
}
