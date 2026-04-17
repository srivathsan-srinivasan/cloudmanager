package gcp

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/api/compute/v1"

	"cloudmanager/internal/core"
	applog "cloudmanager/internal/logging"
)

func ExecuteFirewallActionSDK(ctx context.Context, action string, rule core.FirewallRule, cloudCtx core.CloudContext) (string, error) {
	service, err := compute.NewService(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to create gcp compute service: %w", err)
	}

	project := cloudCtx.AccountID

	fwName := rule.Name
	if fwName == "" || fwName == "-" {
		fwName = rule.ID
	}
	
	// Strip "-allow-x" or "-deny-x" suffixes that might be added by the UI/parser mapping
	if idx := strings.LastIndex(fwName, "-allow-"); idx != -1 {
		fwName = fwName[:idx]
	} else if idx := strings.LastIndex(fwName, "-deny-"); idx != -1 {
		fwName = fwName[:idx]
	}

	switch action {
	case "Delete":
		_, err := service.Firewalls.Delete(project, fwName).Context(ctx).Do()
		if err != nil {
			return "", fmt.Errorf("failed to delete firewall rule %s: %w", fwName, err)
		}
		applog.Infof("AUDIT: user modified firewall rule: deleted rule %s in project %s", fwName, project)
		return fmt.Sprintf("Successfully deleted rule %s", fwName), nil

	case "Enable", "Disable":
		disabled := action == "Disable"
		patch := &compute.Firewall{
			Disabled: disabled,
			ForceSendFields: []string{"Disabled"},
		}
		_, err := service.Firewalls.Patch(project, fwName, patch).Context(ctx).Do()
		if err != nil {
			return "", fmt.Errorf("failed to %s firewall rule %s: %w", action, fwName, err)
		}
		applog.Infof("AUDIT: user modified firewall rule: %sd rule %s in project %s", strings.ToLower(action), fwName, project)
		return fmt.Sprintf("Successfully %sd rule %s", strings.ToLower(action), fwName), nil

	case "Edit":
		existingFw, err := service.Firewalls.Get(project, fwName).Context(ctx).Do()
		if err != nil {
			return "", fmt.Errorf("failed to get firewall rule %s for editing: %w", fwName, err)
		}

		patch := &compute.Firewall{}
		
		if rule.Direction == "Inbound" {
			patch.Direction = "INGRESS"
			if rule.Source != "" && rule.Source != "-" {
				parts := strings.Split(rule.Source, ",")
				for i := range parts {
					parts[i] = strings.TrimSpace(parts[i])
				}
				patch.SourceRanges = parts
				patch.ForceSendFields = append(patch.ForceSendFields, "SourceRanges")
			}
		} else {
			patch.Direction = "EGRESS"
			if rule.Destination != "" && rule.Destination != "-" {
				parts := strings.Split(rule.Destination, ",")
				for i := range parts {
					parts[i] = strings.TrimSpace(parts[i])
				}
				patch.DestinationRanges = parts
				patch.ForceSendFields = append(patch.ForceSendFields, "DestinationRanges")
			}
		}

		if rule.Priority > 0 {
			patch.Priority = int64(rule.Priority)
			patch.ForceSendFields = append(patch.ForceSendFields, "Priority")
		}

		if rule.Action == "Allow" {
			patch.Allowed = []*compute.FirewallAllowed{{
				IPProtocol: rule.Protocol,
			}}
			if rule.PortRange != "" && !strings.EqualFold(rule.PortRange, "all") && rule.PortRange != "-" {
				patch.Allowed[0].Ports = []string{rule.PortRange}
			}
			patch.Denied = []*compute.FirewallDenied{}
			patch.ForceSendFields = append(patch.ForceSendFields, "Allowed")
			if len(existingFw.Denied) > 0 {
				patch.ForceSendFields = append(patch.ForceSendFields, "Denied")
			}
		} else if rule.Action == "Deny" {
			patch.Denied = []*compute.FirewallDenied{{
				IPProtocol: rule.Protocol,
			}}
			if rule.PortRange != "" && !strings.EqualFold(rule.PortRange, "all") && rule.PortRange != "-" {
				patch.Denied[0].Ports = []string{rule.PortRange}
			}
			patch.Allowed = []*compute.FirewallAllowed{}
			patch.ForceSendFields = append(patch.ForceSendFields, "Denied")
			if len(existingFw.Allowed) > 0 {
				patch.ForceSendFields = append(patch.ForceSendFields, "Allowed")
			}
		}

		_, err = service.Firewalls.Patch(project, fwName, patch).Context(ctx).Do()
		if err != nil {
			return "", fmt.Errorf("failed to edit firewall rule %s: %w", fwName, err)
		}

		applog.Infof("AUDIT: user modified firewall rule: edited rule %s in project %s", fwName, project)
		return fmt.Sprintf("Successfully updated rule %s", fwName), nil

	default:
		return "", fmt.Errorf("action %s not supported for GCP Firewalls via CloudManager SDK", action)
	}
}
