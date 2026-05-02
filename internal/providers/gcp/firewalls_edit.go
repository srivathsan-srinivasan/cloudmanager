package gcp

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"google.golang.org/api/compute/v1"

	"cloudmanager/internal/core"
	applog "cloudmanager/internal/logging"
)

func ExecuteFirewallActionSDK(ctx context.Context, action string, rule core.FirewallRule, cloudCtx core.CloudContext) (string, error) {
	service, err := newComputeService(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to create gcp compute service: %w", err)
	}

	project := cloudCtx.AccountID

	if !gcpEditableFirewallRule(rule) {
		return "", fmt.Errorf("policy-derived effective firewall rules are read-only here; edit the source firewall policy instead")
	}

	targetRule := rule
	if rule.OriginalRule != nil {
		targetRule = *rule.OriginalRule
	}

	fwName := gcpFirewallRuleResourceName(targetRule)
	if strings.TrimSpace(fwName) == "" || fwName == "-" {
		return "", fmt.Errorf("missing firewall rule name")
	}

	switch action {
	case "Add":
		newFwName := fmt.Sprintf("fw-rule-%d", time.Now().Unix())
		fw := gcpFirewallFromRule(project, newFwName, rule)

		_, err := service.Firewalls.Insert(project, fw).Context(ctx).Do()
		if err != nil {
			return "", fmt.Errorf("failed to add firewall rule %s: %w", newFwName, err)
		}
		applog.Infof("AUDIT: user modified firewall rule: added rule %s in project %s", newFwName, project)
		return fmt.Sprintf("Successfully added rule %s", newFwName), nil

	case "Delete":
		existingFw, err := service.Firewalls.Get(project, fwName).Context(ctx).Do()
		if err != nil {
			return "", fmt.Errorf("failed to get firewall rule %s for delete: %w", fwName, err)
		}

		patch, deleteEntire, err := gcpBuildFirewallDeletePatch(existingFw, targetRule)
		if err != nil {
			return "", err
		}
		if deleteEntire {
			_, err = service.Firewalls.Delete(project, fwName).Context(ctx).Do()
			if err != nil {
				return "", fmt.Errorf("failed to delete firewall rule %s: %w", fwName, err)
			}
		} else {
			_, err = service.Firewalls.Patch(project, fwName, patch).Context(ctx).Do()
			if err != nil {
				return "", fmt.Errorf("failed to delete entry from firewall rule %s: %w", fwName, err)
			}
		}
		applog.Infof("AUDIT: user modified firewall rule: deleted rule %s in project %s", fwName, project)
		return fmt.Sprintf("Successfully deleted rule %s", fwName), nil

	case "Enable", "Disable":
		disabled := action == "Disable"
		patch := &compute.Firewall{
			Disabled:        disabled,
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

		patch, err := gcpBuildFirewallEditPatch(existingFw, targetRule, rule)
		if err != nil {
			return "", err
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

func gcpEditableFirewallRule(rule core.FirewallRule) bool {
	resourceID := strings.TrimSpace(rule.ResourceID)
	networkID := strings.TrimSpace(rule.NetworkID)
	if resourceID == "" || networkID == "" {
		return true
	}
	return resourceID == networkID
}

func gcpFirewallRuleResourceName(rule core.FirewallRule) string {
	name := strings.TrimSpace(rule.Name)
	if name == "" || name == "-" {
		name = strings.TrimSpace(rule.ID)
	}
	if idx := strings.LastIndex(name, "-allow-"); idx != -1 {
		return name[:idx]
	}
	if idx := strings.LastIndex(name, "-deny-"); idx != -1 {
		return name[:idx]
	}
	return name
}

func gcpFirewallFromRule(project, name string, rule core.FirewallRule) *compute.Firewall {
	fw := &compute.Firewall{
		Name:    name,
		Network: fmt.Sprintf("projects/%s/global/networks/%s", project, rule.ResourceID),
	}
	gcpApplyRuleScope(fw, rule)
	gcpSetRuleEntries(fw, rule.Action, gcpAllowedEntryFromRule(rule), gcpDeniedEntryFromRule(rule))
	if rule.Priority > 0 {
		fw.Priority = int64(rule.Priority)
	}
	if desc := gcpSanitizeRuleDescription(rule.Description); desc != "" {
		fw.Description = desc
	}
	return fw
}

func gcpBuildFirewallDeletePatch(existing *compute.Firewall, target core.FirewallRule) (*compute.Firewall, bool, error) {
	next := gcpCloneFirewallForMutation(existing)
	kind, index, err := gcpFirewallEntryRef(target)
	if err != nil {
		return nil, false, err
	}
	switch kind {
	case "allow":
		next.Allowed, err = gcpRemoveAllowedEntry(next.Allowed, index)
	case "deny":
		next.Denied, err = gcpRemoveDeniedEntry(next.Denied, index)
	default:
		return nil, false, fmt.Errorf("unsupported GCP firewall action target %q", kind)
	}
	if err != nil {
		return nil, false, err
	}
	if len(next.Allowed) == 0 && len(next.Denied) == 0 {
		return nil, true, nil
	}
	return gcpPatchFromFirewall(next, existing), false, nil
}

func gcpBuildFirewallEditPatch(existing *compute.Firewall, original, updated core.FirewallRule) (*compute.Firewall, error) {
	next := gcpCloneFirewallForMutation(existing)
	gcpApplyRuleScope(next, updated)
	if updated.Priority > 0 {
		next.Priority = int64(updated.Priority)
	}
	if desc := gcpSanitizeRuleDescription(updated.Description); desc != "" {
		next.Description = desc
	}

	kind, index, err := gcpFirewallEntryRef(original)
	if err != nil {
		return nil, err
	}

	allowedEntry := gcpAllowedEntryFromRule(updated)
	deniedEntry := gcpDeniedEntryFromRule(updated)
	switch strings.ToLower(strings.TrimSpace(updated.Action)) {
	case "allow":
		next.Allowed, err = gcpUpsertAllowedEntry(next.Allowed, kind, index, allowedEntry)
		if err != nil {
			return nil, err
		}
		if kind == "deny" {
			next.Denied, err = gcpRemoveDeniedEntry(next.Denied, index)
		}
	case "deny":
		next.Denied, err = gcpUpsertDeniedEntry(next.Denied, kind, index, deniedEntry)
		if err != nil {
			return nil, err
		}
		if kind == "allow" {
			next.Allowed, err = gcpRemoveAllowedEntry(next.Allowed, index)
		}
	default:
		err = fmt.Errorf("unsupported GCP firewall action %q", updated.Action)
	}
	if err != nil {
		return nil, err
	}

	return gcpPatchFromFirewall(next, existing), nil
}

func gcpCloneFirewallForMutation(existing *compute.Firewall) *compute.Firewall {
	if existing == nil {
		return &compute.Firewall{}
	}
	return &compute.Firewall{
		Direction:             existing.Direction,
		Priority:              existing.Priority,
		Description:           existing.Description,
		Disabled:              existing.Disabled,
		SourceRanges:          append([]string{}, existing.SourceRanges...),
		SourceTags:            append([]string{}, existing.SourceTags...),
		SourceServiceAccounts: append([]string{}, existing.SourceServiceAccounts...),
		TargetTags:            append([]string{}, existing.TargetTags...),
		TargetServiceAccounts: append([]string{}, existing.TargetServiceAccounts...),
		DestinationRanges:     append([]string{}, existing.DestinationRanges...),
		Allowed:               gcpCloneAllowed(existing.Allowed),
		Denied:                gcpCloneDenied(existing.Denied),
	}
}

func gcpCloneAllowed(in []*compute.FirewallAllowed) []*compute.FirewallAllowed {
	out := make([]*compute.FirewallAllowed, 0, len(in))
	for _, entry := range in {
		if entry == nil {
			out = append(out, nil)
			continue
		}
		out = append(out, &compute.FirewallAllowed{
			IPProtocol: entry.IPProtocol,
			Ports:      append([]string{}, entry.Ports...),
		})
	}
	return out
}

func gcpCloneDenied(in []*compute.FirewallDenied) []*compute.FirewallDenied {
	out := make([]*compute.FirewallDenied, 0, len(in))
	for _, entry := range in {
		if entry == nil {
			out = append(out, nil)
			continue
		}
		out = append(out, &compute.FirewallDenied{
			IPProtocol: entry.IPProtocol,
			Ports:      append([]string{}, entry.Ports...),
		})
	}
	return out
}

func gcpFirewallEntryRef(rule core.FirewallRule) (string, int, error) {
	id := strings.TrimSpace(rule.ID)
	if idx := strings.LastIndex(id, "-allow-"); idx != -1 {
		entryIndex, err := strconv.Atoi(strings.TrimSpace(id[idx+7:]))
		if err != nil {
			return "", 0, fmt.Errorf("invalid GCP allow rule identifier %q", id)
		}
		return "allow", entryIndex, nil
	}
	if idx := strings.LastIndex(id, "-deny-"); idx != -1 {
		entryIndex, err := strconv.Atoi(strings.TrimSpace(id[idx+6:]))
		if err != nil {
			return "", 0, fmt.Errorf("invalid GCP deny rule identifier %q", id)
		}
		return "deny", entryIndex, nil
	}
	switch strings.ToLower(strings.TrimSpace(rule.Action)) {
	case "allow":
		return "allow", 0, nil
	case "deny":
		return "deny", 0, nil
	default:
		return "", 0, fmt.Errorf("unable to resolve GCP firewall entry from rule %q", id)
	}
}

func gcpAllowedEntryFromRule(rule core.FirewallRule) *compute.FirewallAllowed {
	return &compute.FirewallAllowed{
		IPProtocol: gcpNormalizeRuleProtocol(rule.Protocol),
		Ports:      gcpRulePorts(rule.PortRange),
	}
}

func gcpDeniedEntryFromRule(rule core.FirewallRule) *compute.FirewallDenied {
	return &compute.FirewallDenied{
		IPProtocol: gcpNormalizeRuleProtocol(rule.Protocol),
		Ports:      gcpRulePorts(rule.PortRange),
	}
}

func gcpNormalizeRuleProtocol(protocol string) string {
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	switch protocol {
	case "", "-", "*", "all":
		return "all"
	default:
		return protocol
	}
}

func gcpRulePorts(portRange string) []string {
	portRange = strings.TrimSpace(portRange)
	if portRange == "" || portRange == "-" || strings.EqualFold(portRange, "all") {
		return nil
	}
	parts := strings.Split(portRange, ",")
	ports := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			ports = append(ports, part)
		}
	}
	return ports
}

func gcpApplyRuleScope(fw *compute.Firewall, rule core.FirewallRule) {
	if fw == nil {
		return
	}
	networkName := shortName("", fw.Network)
	if networkName == "-" {
		networkName = strings.TrimSpace(rule.NetworkID)
	}
	if networkName == "" {
		networkName = strings.TrimSpace(rule.ResourceID)
	}

	if strings.EqualFold(strings.TrimSpace(rule.Direction), core.Outbound) {
		fw.Direction = "EGRESS"
		targetTags, targetSAs := gcpParseTargetSelectors(rule.Source, networkName)
		destinationRanges := gcpParseRangeSelectors(rule.Destination, networkName)
		if len(destinationRanges) == 0 {
			destinationRanges = []string{"0.0.0.0/0"}
		}
		fw.SourceRanges = []string{}
		fw.SourceTags = []string{}
		fw.SourceServiceAccounts = []string{}
		fw.TargetTags = targetTags
		fw.TargetServiceAccounts = targetSAs
		fw.DestinationRanges = destinationRanges
		return
	}

	fw.Direction = "INGRESS"
	sourceRanges, sourceTags, sourceSAs := gcpParseIngressSource(rule.Source, networkName)
	targetTags, targetSAs := gcpParseTargetSelectors(rule.Destination, networkName)
	if len(sourceRanges) == 0 && len(sourceTags) == 0 && len(sourceSAs) == 0 {
		sourceRanges = []string{"0.0.0.0/0"}
	}
	fw.SourceRanges = sourceRanges
	fw.SourceTags = sourceTags
	fw.SourceServiceAccounts = sourceSAs
	fw.TargetTags = targetTags
	fw.TargetServiceAccounts = targetSAs
	fw.DestinationRanges = []string{}
}

func gcpParseIngressSource(value, networkName string) ([]string, []string, []string) {
	var ranges []string
	var tags []string
	var serviceAccounts []string
	for _, part := range gcpSplitSelectorList(value) {
		switch {
		case strings.HasPrefix(part, "tag:"):
			tags = append(tags, strings.TrimSpace(part[4:]))
		case strings.HasPrefix(part, "sa:"):
			serviceAccounts = append(serviceAccounts, strings.TrimSpace(part[3:]))
		case part == networkName:
		default:
			ranges = append(ranges, part)
		}
	}
	return ranges, tags, serviceAccounts
}

func gcpParseTargetSelectors(value, networkName string) ([]string, []string) {
	var tags []string
	var serviceAccounts []string
	for _, part := range gcpSplitSelectorList(value) {
		switch {
		case strings.HasPrefix(part, "tag:"):
			tags = append(tags, strings.TrimSpace(part[4:]))
		case strings.HasPrefix(part, "sa:"):
			serviceAccounts = append(serviceAccounts, strings.TrimSpace(part[3:]))
		case part == networkName:
		}
	}
	return tags, serviceAccounts
}

func gcpParseRangeSelectors(value, networkName string) []string {
	var ranges []string
	for _, part := range gcpSplitSelectorList(value) {
		switch {
		case strings.HasPrefix(part, "tag:"), strings.HasPrefix(part, "sa:"):
		case part == networkName:
		default:
			ranges = append(ranges, part)
		}
	}
	return ranges
}

func gcpSplitSelectorList(value string) []string {
	rawParts := strings.Split(value, ",")
	parts := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		part = strings.TrimSpace(part)
		if part == "" || part == "-" {
			continue
		}
		parts = append(parts, part)
	}
	return parts
}

func gcpSetRuleEntries(fw *compute.Firewall, action string, allow *compute.FirewallAllowed, deny *compute.FirewallDenied) {
	if fw == nil {
		return
	}
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "deny":
		fw.Allowed = nil
		fw.Denied = []*compute.FirewallDenied{deny}
	default:
		fw.Allowed = []*compute.FirewallAllowed{allow}
		fw.Denied = nil
	}
}

func gcpUpsertAllowedEntry(entries []*compute.FirewallAllowed, originalKind string, index int, entry *compute.FirewallAllowed) ([]*compute.FirewallAllowed, error) {
	switch originalKind {
	case "allow":
		if index < 0 || index >= len(entries) {
			return nil, fmt.Errorf("GCP allow rule index %d out of range", index)
		}
		next := append([]*compute.FirewallAllowed{}, entries...)
		next[index] = entry
		return next, nil
	case "deny":
		return append(entries, entry), nil
	default:
		return nil, fmt.Errorf("unsupported original GCP rule kind %q", originalKind)
	}
}

func gcpUpsertDeniedEntry(entries []*compute.FirewallDenied, originalKind string, index int, entry *compute.FirewallDenied) ([]*compute.FirewallDenied, error) {
	switch originalKind {
	case "deny":
		if index < 0 || index >= len(entries) {
			return nil, fmt.Errorf("GCP deny rule index %d out of range", index)
		}
		next := append([]*compute.FirewallDenied{}, entries...)
		next[index] = entry
		return next, nil
	case "allow":
		return append(entries, entry), nil
	default:
		return nil, fmt.Errorf("unsupported original GCP rule kind %q", originalKind)
	}
}

func gcpRemoveAllowedEntry(entries []*compute.FirewallAllowed, index int) ([]*compute.FirewallAllowed, error) {
	if index < 0 || index >= len(entries) {
		return nil, fmt.Errorf("GCP allow rule index %d out of range", index)
	}
	next := append([]*compute.FirewallAllowed{}, entries[:index]...)
	next = append(next, entries[index+1:]...)
	return next, nil
}

func gcpRemoveDeniedEntry(entries []*compute.FirewallDenied, index int) ([]*compute.FirewallDenied, error) {
	if index < 0 || index >= len(entries) {
		return nil, fmt.Errorf("GCP deny rule index %d out of range", index)
	}
	next := append([]*compute.FirewallDenied{}, entries[:index]...)
	next = append(next, entries[index+1:]...)
	return next, nil
}

func gcpPatchFromFirewall(next, previous *compute.Firewall) *compute.Firewall {
	patch := &compute.Firewall{
		Direction:             next.Direction,
		Priority:              next.Priority,
		Description:           next.Description,
		SourceRanges:          next.SourceRanges,
		SourceTags:            next.SourceTags,
		SourceServiceAccounts: next.SourceServiceAccounts,
		TargetTags:            next.TargetTags,
		TargetServiceAccounts: next.TargetServiceAccounts,
		DestinationRanges:     next.DestinationRanges,
		Allowed:               next.Allowed,
		Denied:                next.Denied,
	}
	patch.ForceSendFields = []string{"Direction", "Priority", "Description"}
	if gcpShouldForceStringSlice(next.SourceRanges, previous.SourceRanges) {
		patch.ForceSendFields = append(patch.ForceSendFields, "SourceRanges")
	}
	if gcpShouldForceStringSlice(next.SourceTags, previous.SourceTags) {
		patch.ForceSendFields = append(patch.ForceSendFields, "SourceTags")
	}
	if gcpShouldForceStringSlice(next.SourceServiceAccounts, previous.SourceServiceAccounts) {
		patch.ForceSendFields = append(patch.ForceSendFields, "SourceServiceAccounts")
	}
	if gcpShouldForceStringSlice(next.TargetTags, previous.TargetTags) {
		patch.ForceSendFields = append(patch.ForceSendFields, "TargetTags")
	}
	if gcpShouldForceStringSlice(next.TargetServiceAccounts, previous.TargetServiceAccounts) {
		patch.ForceSendFields = append(patch.ForceSendFields, "TargetServiceAccounts")
	}
	if gcpShouldForceStringSlice(next.DestinationRanges, previous.DestinationRanges) {
		patch.ForceSendFields = append(patch.ForceSendFields, "DestinationRanges")
	}
	if gcpShouldForceAllowed(next.Allowed, previous.Allowed) {
		patch.ForceSendFields = append(patch.ForceSendFields, "Allowed")
	}
	if gcpShouldForceDenied(next.Denied, previous.Denied) {
		patch.ForceSendFields = append(patch.ForceSendFields, "Denied")
	}
	return patch
}

func gcpShouldForceStringSlice(next, previous []string) bool {
	return next != nil && (len(next) > 0 || len(previous) > 0)
}

func gcpShouldForceAllowed(next, previous []*compute.FirewallAllowed) bool {
	return next != nil && (len(next) > 0 || len(previous) > 0)
}

func gcpShouldForceDenied(next, previous []*compute.FirewallDenied) bool {
	return next != nil && (len(next) > 0 || len(previous) > 0)
}

func gcpSanitizeRuleDescription(description string) string {
	description = strings.TrimSpace(description)
	description = strings.TrimSuffix(description, "[Disabled]")
	return strings.TrimSpace(description)
}
