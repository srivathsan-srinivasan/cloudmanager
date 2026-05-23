package gcp

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"google.golang.org/api/compute/v1"

	"github.com/vyoogam/cloudmanager/internal/core"
)

func FetchSecurityGroupsSDK(ctx context.Context, project string) ([]core.SecurityGroup, error) {
	service, err := newComputeService(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create gcp compute service: %w", err)
	}

	networks, err := service.Networks.List(project).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list gcp networks: %w", err)
	}

	firewalls, err := service.Firewalls.List(project).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list gcp firewall rules: %w", err)
	}
	attachments, err := gcpNetworkAttachmentCounts(ctx, service, project)
	if err != nil {
		return nil, err
	}
	return gcpSecurityGroupsFromNetworkData(networks.Items, firewalls.Items, attachments), nil
}

func FetchFirewallRulesByNetworkSDK(ctx context.Context, project, networkName string) ([]core.FirewallRule, error) {
	service, err := newComputeService(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create gcp compute service: %w", err)
	}

	firewalls, err := service.Firewalls.List(project).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list gcp firewall rules: %w", err)
	}
	rules := gcpClassicFirewallRulesToCore(firewalls.Items, networkName)
	if effective, err := service.Networks.GetEffectiveFirewalls(project, networkName).Context(ctx).Do(); err == nil {
		rules = append(rules, gcpEffectivePolicyRulesToCore(effective, networkName)...)
	}

	rules = dedupeFirewallRules(rules)
	sort.Slice(rules, func(i, j int) bool {
		if rules[i].Priority != rules[j].Priority {
			return rules[i].Priority < rules[j].Priority
		}
		return rules[i].ID < rules[j].ID
	})
	return rules, nil
}

func gcpFirewallRuleToCore(rule *compute.Firewall, networkName string) []core.FirewallRule {
	direction := "Inbound"
	source := gcpClassicRuleSource(rule, networkName)
	destination := gcpClassicRuleDestination(rule, networkName)
	if strings.EqualFold(rule.Direction, "EGRESS") {
		direction = "Outbound"
	}

	var rows []core.FirewallRule
	baseDescription := strings.TrimSpace(rule.Description)

	if len(rule.Allowed) == 0 && len(rule.Denied) == 0 {
		rows = append(rows, core.FirewallRule{
			Name:         rule.Name,
			ID:           rule.Name,
			Direction:    direction,
			Protocol:     "all",
			PortRange:    "All",
			Source:       source,
			Destination:  destination,
			Action:       "Allow",
			Priority:     int(rule.Priority),
			Description:  gcpFirewallDescription(baseDescription, rule.Disabled),
			ResourceID:   networkName,
			ResourceName: networkName,
			NetworkID:    networkName,
			Provider:     "GCP",
		})
	}

	for idx, allowed := range rule.Allowed {
		rows = append(rows, core.FirewallRule{
			Name:         rule.Name,
			ID:           fmt.Sprintf("%s-allow-%d", rule.Name, idx),
			Direction:    direction,
			Protocol:     strings.ToLower(orDash(allowed.IPProtocol)),
			PortRange:    portsOrAll(allowed.Ports),
			Source:       source,
			Destination:  destination,
			Action:       "Allow",
			Priority:     int(rule.Priority),
			Description:  gcpFirewallDescription(baseDescription, rule.Disabled),
			ResourceID:   networkName,
			ResourceName: networkName,
			NetworkID:    networkName,
			Provider:     "GCP",
		})
	}

	for idx, denied := range rule.Denied {
		rows = append(rows, core.FirewallRule{
			Name:         rule.Name,
			ID:           fmt.Sprintf("%s-deny-%d", rule.Name, idx),
			Direction:    direction,
			Protocol:     strings.ToLower(orDash(denied.IPProtocol)),
			PortRange:    portsOrAll(denied.Ports),
			Source:       source,
			Destination:  destination,
			Action:       "Deny",
			Priority:     int(rule.Priority),
			Description:  gcpFirewallDescription(baseDescription, rule.Disabled),
			ResourceID:   networkName,
			ResourceName: networkName,
			NetworkID:    networkName,
			Provider:     "GCP",
		})
	}

	return rows
}

func gcpFirewallDescription(description string, disabled bool) string {
	description = strings.TrimSpace(description)
	if disabled {
		if description == "" {
			return "[Disabled]"
		}
		return fmt.Sprintf("%s [Disabled]", description)
	}
	return orDash(description)
}

func gcpEffectiveFirewallRulesToCore(effective *compute.NetworksGetEffectiveFirewallsResponse, networkName string) []core.FirewallRule {
	var rules []core.FirewallRule
	if effective == nil {
		return rules
	}
	for _, firewall := range effective.Firewalls {
		if shortName("", firewall.Network) != networkName {
			continue
		}
		rules = append(rules, gcpFirewallRuleToCore(firewall, networkName)...)
	}
	for _, policy := range effective.FirewallPolicys {
		rules = append(rules, gcpFirewallPolicyToCore(policy, networkName)...)
	}
	return rules
}

func gcpEffectivePolicyRulesToCore(effective *compute.NetworksGetEffectiveFirewallsResponse, networkName string) []core.FirewallRule {
	var rules []core.FirewallRule
	if effective == nil {
		return rules
	}
	for _, policy := range effective.FirewallPolicys {
		rules = append(rules, gcpFirewallPolicyToCore(policy, networkName)...)
	}
	return rules
}

func dedupeFirewallRules(rules []core.FirewallRule) []core.FirewallRule {
	if len(rules) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(rules))
	deduped := make([]core.FirewallRule, 0, len(rules))
	for _, rule := range rules {
		key := strings.Join([]string{
			rule.ID,
			rule.Direction,
			rule.Protocol,
			rule.PortRange,
			rule.Source,
			rule.Destination,
			rule.Action,
			fmt.Sprintf("%d", rule.Priority),
		}, "|")
		if seen[key] {
			continue
		}
		seen[key] = true
		deduped = append(deduped, rule)
	}
	return deduped
}

func gcpFirewallPolicyToCore(policy *compute.NetworksGetEffectiveFirewallsResponseEffectiveFirewallPolicy, networkName string) []core.FirewallRule {
	if policy == nil {
		return nil
	}

	policyName := shortName(policy.ShortName, policy.Name)
	if policyName == "-" {
		policyName = "firewall-policy"
	}

	var rows []core.FirewallRule
	for idx, rule := range policy.Rules {
		rows = append(rows, gcpFirewallPolicyRuleToCore(rule, networkName, policyName, policy.Type, idx)...)
	}
	return rows
}

func gcpFirewallPolicyRuleToCore(rule *compute.FirewallPolicyRule, networkName, policyName, policyType string, index int) []core.FirewallRule {
	if rule == nil {
		return nil
	}

	direction := "Inbound"
	if strings.EqualFold(rule.Direction, "EGRESS") {
		direction = "Outbound"
	}
	providerLabel := strings.TrimSpace(policyName)
	if strings.TrimSpace(policyType) != "" {
		providerLabel = fmt.Sprintf("%s %s", providerLabel, strings.ToLower(policyType))
	}
	description := strings.TrimSpace(rule.Description)
	if providerLabel != "" {
		if description == "" {
			description = fmt.Sprintf("Policy: %s", providerLabel)
		} else {
			description = fmt.Sprintf("%s [Policy: %s]", description, providerLabel)
		}
	}

	action := gcpPolicyActionLabel(rule.Action)
	source := gcpPolicyRuleSource(rule, networkName)
	destination := gcpPolicyRuleDestination(rule, networkName)
	layer4Configs := gcpPolicyLayer4Configs(rule.Match)
	if len(layer4Configs) == 0 {
		layer4Configs = []*compute.FirewallPolicyRuleMatcherLayer4Config{{IpProtocol: "all"}}
	}

	rows := make([]core.FirewallRule, 0, len(layer4Configs))
	for idx, cfg := range layer4Configs {
		protocol := strings.ToLower(orDash(cfg.IpProtocol))
		if protocol == "*" {
			protocol = "all"
		}
		portRange := portsOrAll(cfg.Ports)
		rows = append(rows, core.FirewallRule{
			ID:           fmt.Sprintf("%s-%d-%d", policyName, rule.Priority, idx+index),
			Direction:    direction,
			Protocol:     protocol,
			PortRange:    portRange,
			Source:       source,
			Destination:  destination,
			Action:       action,
			Priority:     int(rule.Priority),
			Description:  orDash(description),
			ResourceID:   policyName,
			ResourceName: policyName,
			NetworkID:    networkName,
			Provider:     "GCP",
		})
	}
	return rows
}

func gcpPolicyActionLabel(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "allow":
		return "Allow"
	case "deny":
		return "Deny"
	case "goto_next":
		return "Goto Next"
	case "apply_security_profile_group":
		return "Apply Profile"
	case "mirror":
		return "Mirror"
	case "do_not_mirror":
		return "Do Not Mirror"
	default:
		return strings.TrimSpace(action)
	}
}

func gcpClassicRuleSource(rule *compute.Firewall, networkName string) string {
	if rule == nil {
		return "-"
	}
	if strings.EqualFold(rule.Direction, "EGRESS") {
		targets := gcpTargetSelectors(rule.TargetTags, rule.TargetServiceAccounts)
		if len(targets) == 0 {
			return networkName
		}
		return strings.Join(targets, ", ")
	}
	sourceParts := append([]string{}, rule.SourceRanges...)
	sourceParts = append(sourceParts, gcpTagSelectors("tag", rule.SourceTags)...)
	sourceParts = append(sourceParts, gcpTagSelectors("sa", rule.SourceServiceAccounts)...)
	return joinOrDash(sourceParts)
}

func gcpClassicRuleDestination(rule *compute.Firewall, networkName string) string {
	if rule == nil {
		return "-"
	}
	if strings.EqualFold(rule.Direction, "EGRESS") {
		destinations := append([]string{}, rule.DestinationRanges...)
		if len(destinations) == 0 {
			destinations = []string{"0.0.0.0/0"}
		}
		return strings.Join(destinations, ", ")
	}
	targets := gcpTargetSelectors(rule.TargetTags, rule.TargetServiceAccounts)
	if len(targets) == 0 {
		return networkName
	}
	return strings.Join(targets, ", ")
}

func gcpPolicyRuleSource(rule *compute.FirewallPolicyRule, networkName string) string {
	if rule == nil || rule.Match == nil {
		return "-"
	}
	if strings.EqualFold(rule.Direction, "EGRESS") {
		targets := gcpPolicyTargets(rule, networkName)
		if len(targets) == 0 {
			return networkName
		}
		return strings.Join(targets, ", ")
	}
	sourceParts := append([]string{}, rule.Match.SrcIpRanges...)
	sourceParts = append(sourceParts, gcpSecureTagSelectors("secure", rule.Match.SrcSecureTags)...)
	sourceParts = append(sourceParts, gcpNetworkSelectors(rule.Match.SrcNetworks)...)
	return joinOrDash(sourceParts)
}

func gcpPolicyRuleDestination(rule *compute.FirewallPolicyRule, networkName string) string {
	if rule == nil {
		return "-"
	}
	if strings.EqualFold(rule.Direction, "EGRESS") {
		destinationParts := []string{}
		if rule.Match != nil {
			destinationParts = append(destinationParts, rule.Match.DestIpRanges...)
			destinationParts = append(destinationParts, rule.Match.DestFqdns...)
			destinationParts = append(destinationParts, rule.Match.DestAddressGroups...)
			destinationParts = append(destinationParts, gcpNonEmpty(rule.Match.DestThreatIntelligences)...)
			if strings.TrimSpace(rule.Match.DestNetworkContext) != "" {
				destinationParts = append(destinationParts, strings.ToLower(strings.TrimSpace(rule.Match.DestNetworkContext)))
			}
		}
		if len(destinationParts) == 0 {
			destinationParts = []string{"0.0.0.0/0"}
		}
		return strings.Join(destinationParts, ", ")
	}
	targets := gcpPolicyTargets(rule, networkName)
	if len(targets) == 0 {
		return networkName
	}
	return strings.Join(targets, ", ")
}

func gcpPolicyTargets(rule *compute.FirewallPolicyRule, networkName string) []string {
	if rule == nil {
		return nil
	}
	targets := gcpSecureTagSelectors("secure", rule.TargetSecureTags)
	targets = append(targets, gcpTagSelectors("sa", rule.TargetServiceAccounts)...)
	targets = append(targets, gcpNetworkSelectors(rule.TargetResources)...)
	if len(targets) == 0 && strings.TrimSpace(networkName) != "" {
		return []string{networkName}
	}
	return targets
}

func gcpPolicyLayer4Configs(match *compute.FirewallPolicyRuleMatcher) []*compute.FirewallPolicyRuleMatcherLayer4Config {
	if match == nil {
		return nil
	}
	return match.Layer4Configs
}

func gcpTargetSelectors(tags, serviceAccounts []string) []string {
	targets := gcpTagSelectors("tag", tags)
	targets = append(targets, gcpTagSelectors("sa", serviceAccounts)...)
	return targets
}

func gcpTagSelectors(prefix string, values []string) []string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s:%s", prefix, value))
	}
	return parts
}

func gcpSecureTagSelectors(prefix string, values []*compute.FirewallPolicyRuleSecureTag) []string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		if value == nil || strings.TrimSpace(value.Name) == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s:%s", prefix, value.Name))
	}
	return parts
}

func gcpNetworkSelectors(values []string) []string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		value = shortName("", strings.TrimSpace(value))
		if value == "" || value == "-" {
			continue
		}
		parts = append(parts, fmt.Sprintf("network:%s", value))
	}
	return parts
}

func gcpNonEmpty(values []string) []string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			parts = append(parts, value)
		}
	}
	return parts
}

func gcpRuleOpensPort(rule *compute.Firewall, port int) bool {
	if !strings.EqualFold(rule.Direction, "INGRESS") && rule.Direction != "" {
		return false
	}
	if !gcpRangesOpenToInternet(rule.SourceRanges) {
		return false
	}
	for _, allowed := range rule.Allowed {
		if gcpPortAllowed(allowed.IPProtocol, allowed.Ports, port) {
			return true
		}
	}
	return false
}

func gcpRangesOpenToInternet(ranges []string) bool {
	for _, value := range ranges {
		if value == "0.0.0.0/0" || value == "::/0" {
			return true
		}
	}
	return false
}

func gcpPortAllowed(protocol string, ports []string, port int) bool {
	proto := strings.ToLower(strings.TrimSpace(protocol))
	if proto == "" || proto == "all" {
		return true
	}
	if proto != "tcp" && proto != "udp" {
		return false
	}
	if len(ports) == 0 {
		return true
	}
	for _, value := range ports {
		if portInRange(value, port) {
			return true
		}
	}
	return false
}

func portInRange(value string, port int) bool {
	if value == "" {
		return true
	}
	if !strings.Contains(value, "-") {
		return fmt.Sprintf("%d", port) == value
	}
	var start, end int
	if _, err := fmt.Sscanf(value, "%d-%d", &start, &end); err != nil {
		return false
	}
	return port >= start && port <= end
}

func gcpNetworkAttachmentCounts(ctx context.Context, service *compute.Service, project string) (map[string]int, error) {
	counts := make(map[string]int)
	req := service.Instances.AggregatedList(project)
	if err := req.Pages(ctx, func(page *compute.InstanceAggregatedList) error {
		accumulateGCPNetworkAttachmentCounts(counts, page.Items)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to list gcp instances for firewall attachment counts: %w", err)
	}
	return counts, nil
}

func accumulateGCPNetworkAttachmentCounts(counts map[string]int, items map[string]compute.InstancesScopedList) {
	for _, scopedList := range items {
		for _, inst := range scopedList.Instances {
			addGCPInstanceNetworkAttachments(counts, inst)
		}
	}
}

func addGCPInstanceNetworkAttachments(counts map[string]int, inst *compute.Instance) {
	if counts == nil || inst == nil {
		return
	}
	for _, nic := range inst.NetworkInterfaces {
		if nic == nil {
			continue
		}
		networkName := shortName("", nic.Network)
		if networkName == "" || networkName == "-" {
			continue
		}
		counts[networkName]++
	}
}

func portsOrAll(ports []string) string {
	if len(ports) == 0 {
		return "All"
	}
	return strings.Join(ports, ",")
}

func joinOrDash(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	return strings.Join(values, ", ")
}

func shortName(name, selfLink string) string {
	if name != "" {
		return name
	}
	parts := strings.Split(strings.TrimRight(selfLink, "/"), "/")
	if len(parts) == 0 {
		return "-"
	}
	return parts[len(parts)-1]
}

func orDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}
