package main

import (
	"context"
	"fmt"
	"os/exec"
)

type Provider interface {
	FetchVMs(ctx context.Context, item contextItem) ([]VM, error)
	ExecuteAction(ctx context.Context, action string, vm VM, item contextItem) (string, error)
	GetSSHCmd(ctx context.Context, vm VM, item contextItem) (*exec.Cmd, error)
}

func GetProvider() Provider {
	cfg := loadAppConfig()
	if cfg.Backend == "sdk" {
		return &SDKProvider{}
	}
	return &CLIProvider{}
}

// CLIProvider uses existing os/exec logic
type CLIProvider struct{}

func (p *CLIProvider) FetchVMs(ctx context.Context, item contextItem) ([]VM, error) {
	switch item.provider {
	case "AWS":
		return fetchAWSVMs(item.accountName, item.region)
	case "GCP":
		return fetchGCPVMs(item.accountID)
	case "Azure":
		return fetchAzureVMs(item.accountID)
	}
	return nil, fmt.Errorf("provider %s not supported", item.provider)
}

func (p *CLIProvider) ExecuteAction(ctx context.Context, action string, vm VM, item contextItem) (string, error) {
	var cmd *exec.Cmd

	switch item.provider {
	case "AWS":
		switch action {
		case "Start":
			cmd = exec.CommandContext(ctx, "aws", "ec2", "start-instances", "--instance-ids", vm.ID, "--profile", item.accountName, "--region", item.region)
		case "Stop":
			cmd = exec.CommandContext(ctx, "aws", "ec2", "stop-instances", "--instance-ids", vm.ID, "--profile", item.accountName, "--region", item.region)
		case "Restart":
			cmd = exec.CommandContext(ctx, "aws", "ec2", "reboot-instances", "--instance-ids", vm.ID, "--profile", item.accountName, "--region", item.region)
		case "Terminate":
			cmd = exec.CommandContext(ctx, "aws", "ec2", "terminate-instances", "--instance-ids", vm.ID, "--profile", item.accountName, "--region", item.region)
		case "Describe":
			cmd = exec.CommandContext(ctx, "aws", "ec2", "describe-instances", "--instance-ids", vm.ID, "--profile", item.accountName, "--region", item.region)
		}
	case "GCP":
		switch action {
		case "Start":
			cmd = exec.CommandContext(ctx, "gcloud", "compute", "instances", "start", vm.Name, "--project", item.accountID, "--zone", vm.Zone)
		case "Stop":
			cmd = exec.CommandContext(ctx, "gcloud", "compute", "instances", "stop", vm.Name, "--project", item.accountID, "--zone", vm.Zone)
		case "Restart":
			cmd = exec.CommandContext(ctx, "gcloud", "compute", "instances", "reset", vm.Name, "--project", item.accountID, "--zone", vm.Zone)
		case "Terminate":
			cmd = exec.CommandContext(ctx, "gcloud", "compute", "instances", "delete", vm.Name, "--project", item.accountID, "--zone", vm.Zone, "--quiet")
		case "Describe":
			cmd = exec.CommandContext(ctx, "gcloud", "compute", "instances", "describe", vm.Name, "--project", item.accountID, "--zone", vm.Zone)
		}
	case "Azure":
		switch action {
		case "Start":
			cmd = exec.CommandContext(ctx, "az", "vm", "start", "--name", vm.Name, "--resource-group", vm.ResourceGroup, "--subscription", item.accountID)
		case "Stop":
			cmd = exec.CommandContext(ctx, "az", "vm", "stop", "--name", vm.Name, "--resource-group", vm.ResourceGroup, "--subscription", item.accountID)
		case "Restart":
			cmd = exec.CommandContext(ctx, "az", "vm", "restart", "--name", vm.Name, "--resource-group", vm.ResourceGroup, "--subscription", item.accountID)
		case "Terminate":
			cmd = exec.CommandContext(ctx, "az", "vm", "delete", "--name", vm.Name, "--resource-group", vm.ResourceGroup, "--subscription", item.accountID, "--yes")
		case "Describe":
			cmd = exec.CommandContext(ctx, "az", "vm", "show", "--name", vm.Name, "--resource-group", vm.ResourceGroup, "--subscription", item.accountID)
		}
	}

	if cmd == nil {
		return "", fmt.Errorf("action %s not supported for %s", action, item.provider)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to %s: %w\n%s", action, err, string(output))
	}

	if action == "Describe" {
		return string(output), nil
	}
	return fmt.Sprintf("Successfully executed '%s' on %s", action, vm.Name), nil
}

func (p *CLIProvider) GetSSHCmd(ctx context.Context, vm VM, item contextItem) (*exec.Cmd, error) {
	cmd := createSSHCmd(vm, item)
	if cmd == nil {
		return nil, fmt.Errorf("SSH not supported for %s", item.provider)
	}
	return cmd, nil
}

// SDKProvider is a stub for now
type SDKProvider struct{}

func (p *SDKProvider) FetchVMs(ctx context.Context, item contextItem) ([]VM, error) {
	return nil, fmt.Errorf("SDK backend not fully implemented yet for %s. Re-run with '--backend cli' or use '--configure'", item.provider)
}

func (p *SDKProvider) ExecuteAction(ctx context.Context, action string, vm VM, item contextItem) (string, error) {
	return "", fmt.Errorf("SDK backend not fully implemented yet for %s", item.provider)
}

func (p *SDKProvider) GetSSHCmd(ctx context.Context, vm VM, item contextItem) (*exec.Cmd, error) {
	return nil, fmt.Errorf("SDK backend not fully implemented yet for %s", item.provider)
}
