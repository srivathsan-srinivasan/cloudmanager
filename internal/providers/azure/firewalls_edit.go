package azure

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"

	"github.com/srivathsan-srinivasan/cloudmanager/internal/core"
	applog "github.com/srivathsan-srinivasan/cloudmanager/internal/logging"
)

func ExecuteFirewallActionSDK(ctx context.Context, action string, rule core.FirewallRule, cloudCtx core.CloudContext) (string, error) {
	ruleName := rule.Name
	if ruleName == "" || ruleName == "-" {
		// ID might be the full resource ID, but rule name might be the last segment
		parts := strings.Split(rule.ID, "/")
		ruleName = parts[len(parts)-1]
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

	client, err := getSecurityRulesClient(cloudCtx.AccountID)
	if err != nil {
		return "", fmt.Errorf("failed to create azure security rules client: %w", err)
	}

	switch action {
	case "Add":
		ruleName = fmt.Sprintf("rule-%d", time.Now().Unix())
		ruleObj := armnetwork.SecurityRule{
			Properties: &armnetwork.SecurityRulePropertiesFormat{
				Priority: to.Ptr(int32(1000)),
			},
		}

		if rule.Direction == "Inbound" {
			ruleObj.Properties.Direction = to.Ptr(armnetwork.SecurityRuleDirectionInbound)
		} else {
			ruleObj.Properties.Direction = to.Ptr(armnetwork.SecurityRuleDirectionOutbound)
		}

		proto := strings.ToLower(rule.Protocol)
		if proto == "tcp" {
			ruleObj.Properties.Protocol = to.Ptr(armnetwork.SecurityRuleProtocolTCP)
		} else if proto == "udp" {
			ruleObj.Properties.Protocol = to.Ptr(armnetwork.SecurityRuleProtocolUDP)
		} else if proto == "icmp" {
			ruleObj.Properties.Protocol = to.Ptr(armnetwork.SecurityRuleProtocolIcmp)
		} else {
			ruleObj.Properties.Protocol = to.Ptr(armnetwork.SecurityRuleProtocolAsterisk)
		}

		if rule.Action == "Allow" {
			ruleObj.Properties.Access = to.Ptr(armnetwork.SecurityRuleAccessAllow)
		} else {
			ruleObj.Properties.Access = to.Ptr(armnetwork.SecurityRuleAccessDeny)
		}

		if rule.Source != "" {
			ruleObj.Properties.SourceAddressPrefix = to.Ptr(rule.Source)
		} else {
			ruleObj.Properties.SourceAddressPrefix = to.Ptr("*")
		}

		if rule.Destination != "" {
			ruleObj.Properties.DestinationAddressPrefix = to.Ptr(rule.Destination)
		} else {
			ruleObj.Properties.DestinationAddressPrefix = to.Ptr("*")
		}

		if rule.PortRange != "" && rule.PortRange != "-" {
			if strings.EqualFold(rule.PortRange, "all") {
				ruleObj.Properties.DestinationPortRange = to.Ptr("*")
			} else {
				ruleObj.Properties.DestinationPortRange = to.Ptr(rule.PortRange)
			}
		} else {
			ruleObj.Properties.DestinationPortRange = to.Ptr("*")
		}

		poller, err := client.BeginCreateOrUpdate(ctx, rg, nsgName, ruleName, ruleObj, nil)
		if err != nil {
			return "", fmt.Errorf("failed to create Azure firewall rule %s: %w", ruleName, err)
		}
		_, err = poller.PollUntilDone(ctx, nil)
		if err != nil {
			return "", fmt.Errorf("failed waiting for create of Azure firewall rule %s: %w", ruleName, err)
		}
		applog.Infof("AUDIT: user modified firewall rule: added rule %s to NSG %s (RG: %s)", ruleName, nsgName, rg)
		return fmt.Sprintf("Successfully added rule %s", ruleName), nil

	case "Delete":
		poller, err := client.BeginDelete(ctx, rg, nsgName, ruleName, nil)
		if err != nil {
			return "", fmt.Errorf("failed to delete Azure firewall rule %s: %w", ruleName, err)
		}
		_, err = poller.PollUntilDone(ctx, nil)
		if err != nil {
			return "", fmt.Errorf("failed waiting for delete of Azure firewall rule %s: %w", ruleName, err)
		}
		applog.Infof("AUDIT: user modified firewall rule: deleted rule %s in NSG %s (RG: %s)", ruleName, nsgName, rg)
		return fmt.Sprintf("Successfully deleted rule %s", ruleName), nil

	case "Edit":
		// Fetch existing rule
		resp, err := client.Get(ctx, rg, nsgName, ruleName, nil)
		if err != nil {
			return "", fmt.Errorf("failed to get azure security rule %s: %w", ruleName, err)
		}

		ruleObj := resp.SecurityRule
		if ruleObj.Properties == nil {
			ruleObj.Properties = &armnetwork.SecurityRulePropertiesFormat{}
		}

		// Update properties
		if rule.Direction == "Inbound" {
			ruleObj.Properties.Direction = to.Ptr(armnetwork.SecurityRuleDirectionInbound)
		} else if rule.Direction == "Outbound" {
			ruleObj.Properties.Direction = to.Ptr(armnetwork.SecurityRuleDirectionOutbound)
		}

		proto := strings.ToLower(rule.Protocol)
		if proto == "tcp" {
			ruleObj.Properties.Protocol = to.Ptr(armnetwork.SecurityRuleProtocolTCP)
		} else if proto == "udp" {
			ruleObj.Properties.Protocol = to.Ptr(armnetwork.SecurityRuleProtocolUDP)
		} else if proto == "icmp" {
			ruleObj.Properties.Protocol = to.Ptr(armnetwork.SecurityRuleProtocolIcmp)
		} else {
			ruleObj.Properties.Protocol = to.Ptr(armnetwork.SecurityRuleProtocolAsterisk)
		}

		if rule.Action == "Allow" {
			ruleObj.Properties.Access = to.Ptr(armnetwork.SecurityRuleAccessAllow)
		} else if rule.Action == "Deny" {
			ruleObj.Properties.Access = to.Ptr(armnetwork.SecurityRuleAccessDeny)
		}

		if rule.Source != "" && rule.Source != "-" {
			ruleObj.Properties.SourceAddressPrefix = to.Ptr(rule.Source)
		}
		if rule.Destination != "" && rule.Destination != "-" {
			ruleObj.Properties.DestinationAddressPrefix = to.Ptr(rule.Destination)
		}
		if rule.PortRange != "" && rule.PortRange != "-" {
			if strings.EqualFold(rule.PortRange, "all") {
				ruleObj.Properties.DestinationPortRange = to.Ptr("*")
			} else {
				ruleObj.Properties.DestinationPortRange = to.Ptr(rule.PortRange)
			}
		}
		if rule.Priority > 0 {
			ruleObj.Properties.Priority = to.Ptr(int32(rule.Priority))
		}
		if rule.Description != "" && rule.Description != "-" {
			ruleObj.Properties.Description = to.Ptr(rule.Description)
		}

		poller, err := client.BeginCreateOrUpdate(ctx, rg, nsgName, ruleName, ruleObj, nil)
		if err != nil {
			return "", fmt.Errorf("failed to update Azure firewall rule %s: %w", ruleName, err)
		}
		_, err = poller.PollUntilDone(ctx, nil)
		if err != nil {
			return "", fmt.Errorf("failed waiting for update of Azure firewall rule %s: %w", ruleName, err)
		}
		applog.Infof("AUDIT: user modified firewall rule: edited rule %s in NSG %s (RG: %s)", ruleName, nsgName, rg)
		return fmt.Sprintf("Successfully updated rule %s", ruleName), nil

	default:
		return "", fmt.Errorf("action %s not supported for Azure Firewalls via CloudManager SDK", action)
	}
}
