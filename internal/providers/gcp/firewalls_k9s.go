package gcp

import (
	"context"
	"fmt"
	"cloudmanager/internal/core"
)

func ExecuteFirewallActionSDK(ctx context.Context, action string, rule core.FirewallRule, cloudCtx core.CloudContext) (string, error) {
	if action == "Delete" {
		output, err := runGCloudJSON("compute", "firewall-rules", "delete", rule.Name, "--project", cloudCtx.AccountID, "--quiet")
		if err != nil {
			return "", fmt.Errorf("failed to delete firewall rule %s: %w", rule.Name, err)
		}
		return fmt.Sprintf("Successfully deleted rule %s\n%s", rule.Name, string(output)), nil
	}
	
	if action == "Enable" || action == "Disable" {
		disabled := "true"
		if action == "Enable" { disabled = "false" }
		
		output, err := runGCloudJSON("compute", "firewall-rules", "update", rule.Name, "--project", cloudCtx.AccountID, "--disabled="+disabled)
		if err != nil {
			return "", fmt.Errorf("failed to %s firewall rule %s: %w", action, rule.Name, err)
		}
		return fmt.Sprintf("Successfully %sd rule %s\n%s", action, rule.Name, string(output)), nil
	}

	return "", fmt.Errorf("action %s not supported for GCP Firewalls via CloudManager SDK", action)
}
