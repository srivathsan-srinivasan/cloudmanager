package azure

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"

	"cloudmanager/internal/core"
)

func FetchSecurityGroupsSDK(ctx context.Context, subscriptionID string) ([]core.SecurityGroup, error) {
	client, err := getSecurityGroupsClient(subscriptionID)
	if err != nil {
		return nil, err
	}

	pager := client.NewListAllPager(nil)
	var groups []core.SecurityGroup
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list security groups: %w", err)
		}
		for _, sg := range page.Value {
			inbound, outbound := countRules(sg.Properties.SecurityRules)
			inboundDefault, outboundDefault := countRules(sg.Properties.DefaultSecurityRules)

			hasOpenSSH, hasOpenRDP := checkRiskyRules(sg.Properties.SecurityRules)

			groups = append(groups, core.SecurityGroup{
				ID:                orPtr(sg.ID),
				Name:              orPtr(sg.Name),
				Description:       "-",
				NetworkID:         "-",
				InboundRuleCount:  inbound + inboundDefault,
				OutboundRuleCount: outbound + outboundDefault,
				AttachedResources: len(sg.Properties.NetworkInterfaces),
				HasOpenSSH:        hasOpenSSH,
				HasOpenRDP:        hasOpenRDP,
				Provider:          "Azure",
				Region:            orPtr(sg.Location),
				ResourceGroup:     azureResourceGroupFromID(sg.ID),
			})
		}
	}
	return groups, nil
}

func FetchFirewallRulesSDK(ctx context.Context, subscriptionID, resourceGroup, nsgName string) ([]core.FirewallRule, error) {
	client, err := getSecurityRulesClient(subscriptionID)
	if err != nil {
		return nil, err
	}

	pager := client.NewListPager(resourceGroup, nsgName, nil)
	var rules []core.FirewallRule
	nsgID := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/networkSecurityGroups/%s", subscriptionID, resourceGroup, nsgName)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list security rules: %w", err)
		}
		for _, r := range page.Value {
			rules = append(rules, azureSecurityRuleToCore(r, nsgName, nsgID, false))
		}
	}
	return rules, nil
}

func azureSecurityRuleToCore(r *armnetwork.SecurityRule, nsgName, nsgID string, isDefault bool) core.FirewallRule {
	if r.Properties == nil {
		return core.FirewallRule{}
	}
	direction := core.Inbound
	if r.Properties.Direction != nil && *r.Properties.Direction == armnetwork.SecurityRuleDirectionOutbound {
		direction = core.Outbound
	}

	protocol := "*"
	if r.Properties.Protocol != nil {
		protocol = strings.ToLower(string(*r.Properties.Protocol))
	}

	action := "Allow"
	if r.Properties.Access != nil && *r.Properties.Access == armnetwork.SecurityRuleAccessDeny {
		action = "Deny"
	}

	return core.FirewallRule{
		Name:         orPtr(r.Name),
		ID:           orPtr(r.ID),
		Direction:    direction,
		Protocol:     protocol,
		PortRange:    orPtr(r.Properties.DestinationPortRange),
		Source:       orPtr(r.Properties.SourceAddressPrefix),
		Destination:  orPtr(r.Properties.DestinationAddressPrefix),
		Action:       action,
		Priority:     int(orPtrInt32(r.Properties.Priority)),
		Description:  orPtr(r.Properties.Description),
		ResourceID:   nsgID,
		NetworkID:    nsgID,
		ResourceName: nsgName,
		Provider:     "Azure",
	}
}

func azureRuleOpensPort(r *armnetwork.SecurityRule, port int) bool {
	if r.Properties == nil {
		return false
	}
	if r.Properties.Direction == nil || *r.Properties.Direction != armnetwork.SecurityRuleDirectionInbound {
		return false
	}
	if r.Properties.Access == nil || *r.Properties.Access != armnetwork.SecurityRuleAccessAllow {
		return false
	}

	source := orPtr(r.Properties.SourceAddressPrefix)
	if source != "*" && source != "0.0.0.0/0" && source != "Internet" {
		return false
	}

	return core.PortRangeIncludes(orPtr(r.Properties.DestinationPortRange), port)
}

func countRules(rules []*armnetwork.SecurityRule) (int, int) {
	inbound, outbound := 0, 0
	for _, r := range rules {
		if r.Properties.Direction != nil {
			if *r.Properties.Direction == armnetwork.SecurityRuleDirectionInbound {
				inbound++
			} else {
				outbound++
			}
		}
	}
	return inbound, outbound
}

func checkRiskyRules(rules []*armnetwork.SecurityRule) (bool, bool) {
	ssh, rdp := false, false
	for _, r := range rules {
		if azureRuleOpensPort(r, 22) {
			ssh = true
		}
		if azureRuleOpensPort(r, 3389) {
			rdp = true
		}
	}
	return ssh, rdp
}

func orPtrInt32(v *int32) int32 {
	if v == nil {
		return 0
	}
	return *v
}

func azureTagsToString(tags map[string]*string) string {
	var parts []string
	for k, v := range tags {
		if v == nil {
			parts = append(parts, fmt.Sprintf("%s=", k))
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", k, *v))
	}
	return strings.Join(parts, ", ")
}

func orDashPtr(s *string) string {
	if s == nil || strings.TrimSpace(*s) == "" {
		return "-"
	}
	return *s
}

func getSecurityGroupsClient(subscription string) (*armnetwork.SecurityGroupsClient, error) {
	cred, err := getAzureCreds()
	if err != nil {
		return nil, err
	}
	return armnetwork.NewSecurityGroupsClient(subscription, cred, nil)
}

func getSecurityRulesClient(subscription string) (*armnetwork.SecurityRulesClient, error) {
	cred, err := getAzureCreds()
	if err != nil {
		return nil, err
	}
	return armnetwork.NewSecurityRulesClient(subscription, cred, nil)
}
