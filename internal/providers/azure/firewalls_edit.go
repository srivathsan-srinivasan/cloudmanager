package azure

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"cloudmanager/internal/core"
)

func ExecuteFirewallActionSDK(ctx context.Context, action string, rule core.FirewallRule, cloudCtx core.CloudContext) (string, error) {
	if action != "Delete" {
		return "", fmt.Errorf("action %s not supported for Azure Firewalls via CloudManager SDK", action)
	}

	ruleName := rule.Name
	if ruleName == "" || ruleName == "-" {
		ruleName = rule.ID
	}
	
	sgID := rule.ResourceID
	if sgID == "" || ruleName == "" {
		return "", fmt.Errorf("missing Security Group ID or Rule Name")
	}

	rg := ""
	nsgName := ""
	parts := strings.Split(sgID, "/")
	for i, p := range parts {
		if strings.EqualFold(p, "resourceGroups") && i+1 < len(parts) {
			rg = parts[i+1]
		}
		if strings.EqualFold(p, "networkSecurityGroups") && i+1 < len(parts) {
			nsgName = parts[i+1]
		}
	}

	if rg == "" || nsgName == "" {
		return "", fmt.Errorf("could not extract RG or NSG name from %s", sgID)
	}

	cmd := exec.CommandContext(ctx, "az", "network", "nsg", "rule", "delete", "--resource-group", rg, "--nsg-name", nsgName, "--name", ruleName, "--subscription", cloudCtx.AccountID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to delete Azure firewall rule %s: %w\n%s", ruleName, err, string(output))
	}

	return fmt.Sprintf("Successfully deleted rule %s\n%s", ruleName, string(output)), nil
}
