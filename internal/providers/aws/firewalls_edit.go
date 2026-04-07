package aws

import (
	"context"
	"fmt"
	"strings"

	"cloudmanager/internal/core"
)

func ExecuteFirewallActionSDK(ctx context.Context, action string, rule core.FirewallRule, cloudCtx core.CloudContext) (string, error) {
	if action != "Delete" {
		return "", fmt.Errorf("action %s not supported for AWS Firewalls via CloudManager SDK", action)
	}
	
	sgID := rule.ResourceID
	if sgID == "" {
		return "", fmt.Errorf("missing Security Group ID for rule")
	}

	var args []string
	if rule.Direction == "Inbound" {
		args = append(args, "ec2", "revoke-security-group-ingress", "--group-id", sgID)
	} else {
		args = append(args, "ec2", "revoke-security-group-egress", "--group-id", sgID)
	}

	// This is a best effort mapping to the CLI parameters.
	protocol := rule.Protocol
	if protocol == "" || protocol == "-" {
		protocol = "all"
	}
	args = append(args, "--protocol", protocol)

	if rule.PortRange != "" && rule.PortRange != "-" && rule.PortRange != "all" {
		args = append(args, "--port", rule.PortRange)
	}

	if rule.Source != "" && rule.Source != "-" {
		if strings.HasPrefix(rule.Source, "sg-") {
			args = append(args, "--source-group", rule.Source)
		} else {
			args = append(args, "--cidr", rule.Source)
		}
	} else if rule.Destination != "" && rule.Destination != "-" {
		if strings.HasPrefix(rule.Destination, "sg-") {
			args = append(args, "--source-group", rule.Destination)
		} else {
			args = append(args, "--cidr", rule.Destination)
		}
	}

	cmd := awsCLICommand(ctx, cloudCtx, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to delete AWS firewall rule: %w\n%s", err, string(output))
	}

	return fmt.Sprintf("Successfully deleted rule for %s\n%s", sgID, string(output)), nil
}
