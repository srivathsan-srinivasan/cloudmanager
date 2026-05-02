package aws

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"cloudmanager/internal/core"
	applog "cloudmanager/internal/logging"
)

func ExecuteFirewallActionSDK(ctx context.Context, action string, rule core.FirewallRule, cloudCtx core.CloudContext) (string, error) {
	cfg, err := getAWSConfig(ctx, cloudCtx.CredentialProfile, cloudCtx.Region)
	if err != nil {
		return "", fmt.Errorf("failed to load aws config: %w", err)
	}
	client := ec2.NewFromConfig(cfg)

	sgID := rule.ResourceID
	if sgID == "" {
		return "", fmt.Errorf("missing Security Group ID for rule")
	}

	switch action {
	case "Add":
		err = authorizeAWSRule(ctx, client, sgID, rule)
		if err != nil {
			return "", fmt.Errorf("failed to add new rule: %w", err)
		}
		applog.Infof("AUDIT: user modified firewall rule: added new rule to SG %s", sgID)
		return fmt.Sprintf("Successfully added new rule to %s", sgID), nil

	case "Delete":
		err = revokeAWSRule(ctx, client, sgID, rule)
		if err != nil {
			return "", err
		}
		applog.Infof("AUDIT: user modified firewall rule: deleted rule %s in SG %s", rule.ID, sgID)
		return fmt.Sprintf("Successfully deleted rule %s from %s", rule.ID, sgID), nil

	case "Edit":
		if rule.OriginalRule == nil {
			return "", fmt.Errorf("missing original rule for edit action")
		}

		if canModifyAWSRuleInPlace(*rule.OriginalRule, rule) {
			err = modifyAWSRule(ctx, client, sgID, *rule.OriginalRule, rule)
			if err != nil {
				return "", fmt.Errorf("failed to modify rule %s: %w", rule.OriginalRule.ID, err)
			}
		} else {
			err = revokeAWSRule(ctx, client, sgID, *rule.OriginalRule)
			if err != nil {
				return "", fmt.Errorf("failed to revoke original rule: %w", err)
			}

			err = authorizeAWSRule(ctx, client, sgID, rule)
			if err != nil {
				return "", fmt.Errorf("failed to authorize updated rule: %w", err)
			}
		}

		applog.Infof("AUDIT: user modified firewall rule: edited rule %s in SG %s", rule.ID, sgID)
		return fmt.Sprintf("Successfully updated rule %s in %s", rule.ID, sgID), nil

	default:
		return "", fmt.Errorf("action %s not supported for AWS Firewalls via CloudManager SDK", action)
	}
}

func revokeAWSRule(ctx context.Context, client *ec2.Client, sgID string, rule core.FirewallRule) error {
	if awsHasSecurityGroupRuleID(rule) {
		if rule.Direction == "Inbound" {
			_, err := client.RevokeSecurityGroupIngress(ctx, &ec2.RevokeSecurityGroupIngressInput{
				GroupId:              awssdk.String(sgID),
				SecurityGroupRuleIds: []string{rule.ID},
			})
			return err
		}
		_, err := client.RevokeSecurityGroupEgress(ctx, &ec2.RevokeSecurityGroupEgressInput{
			GroupId:              awssdk.String(sgID),
			SecurityGroupRuleIds: []string{rule.ID},
		})
		return err
	}

	perms, err := toAWSPermissions(rule)
	if err != nil {
		return err
	}
	if rule.Direction == "Inbound" {
		_, err = client.RevokeSecurityGroupIngress(ctx, &ec2.RevokeSecurityGroupIngressInput{
			GroupId:       awssdk.String(sgID),
			IpPermissions: perms,
		})
		return err
	} else {
		_, err = client.RevokeSecurityGroupEgress(ctx, &ec2.RevokeSecurityGroupEgressInput{
			GroupId:       awssdk.String(sgID),
			IpPermissions: perms,
		})
		return err
	}
}

func modifyAWSRule(ctx context.Context, client *ec2.Client, sgID string, original, updated core.FirewallRule) error {
	request, err := toAWSSecurityGroupRuleRequest(updated)
	if err != nil {
		return err
	}
	_, err = client.ModifySecurityGroupRules(ctx, &ec2.ModifySecurityGroupRulesInput{
		GroupId: awssdk.String(sgID),
		SecurityGroupRules: []ec2types.SecurityGroupRuleUpdate{{
			SecurityGroupRuleId: awssdk.String(original.ID),
			SecurityGroupRule:   request,
		}},
	})
	return err
}

func canModifyAWSRuleInPlace(original, updated core.FirewallRule) bool {
	if !awsHasSecurityGroupRuleID(original) {
		return false
	}
	if strings.TrimSpace(original.Direction) != strings.TrimSpace(updated.Direction) {
		return false
	}
	if len(awsPortSegments(updated.PortRange)) != 1 {
		return false
	}
	return awsRulePeerKind(original) == awsRulePeerKind(updated)
}

func awsHasSecurityGroupRuleID(rule core.FirewallRule) bool {
	return strings.HasPrefix(strings.TrimSpace(rule.ID), "sgr-")
}

func authorizeAWSRule(ctx context.Context, client *ec2.Client, sgID string, rule core.FirewallRule) error {
	perms, err := toAWSPermissions(rule)
	if err != nil {
		return err
	}
	if rule.Direction == "Inbound" {
		_, err := client.AuthorizeSecurityGroupIngress(ctx, &ec2.AuthorizeSecurityGroupIngressInput{
			GroupId:       awssdk.String(sgID),
			IpPermissions: perms,
		})
		return err
	} else {
		_, err := client.AuthorizeSecurityGroupEgress(ctx, &ec2.AuthorizeSecurityGroupEgressInput{
			GroupId:       awssdk.String(sgID),
			IpPermissions: perms,
		})
		return err
	}
}

func toAWSSecurityGroupRuleRequest(rule core.FirewallRule) (*ec2types.SecurityGroupRuleRequest, error) {
	protocol := strings.ToLower(strings.TrimSpace(rule.Protocol))
	if protocol == "" || protocol == "all" || protocol == "*" {
		protocol = "-1"
	}

	request := &ec2types.SecurityGroupRuleRequest{
		IpProtocol: awssdk.String(protocol),
	}

	if protocol != "-1" {
		segments := awsPortSegments(rule.PortRange)
		if len(segments) != 1 {
			return nil, fmt.Errorf("AWS security group rule edits support one port/range per rule; use comma-separated ports to create multiple rules")
		}
		if fromPort, toPort, ok := awsPortRangeBounds(segments[0]); ok {
			request.FromPort = awssdk.Int32(fromPort)
			request.ToPort = awssdk.Int32(toPort)
		} else {
			return nil, fmt.Errorf("invalid AWS port or range %q", rule.PortRange)
		}
	}

	if desc := awsCleanRuleValue(rule.Description); desc != "" {
		request.Description = awssdk.String(desc)
	}

	peer := awsRulePeerValue(rule)
	switch kind := awsPeerKindFromValue(peer); kind {
	case "ipv4":
		request.CidrIpv4 = awssdk.String(peer)
	case "ipv6":
		request.CidrIpv6 = awssdk.String(peer)
	case "prefix-list":
		request.PrefixListId = awssdk.String(peer)
	case "sg":
		request.ReferencedGroupId = awssdk.String(awsReferencedGroupID(peer))
	}

	if awsPeerKindFromValue(peer) == "" {
		return nil, fmt.Errorf("AWS peer must be a CIDR, sg-..., or pl-...")
	}

	return request, nil
}

func toAWSPermissions(rule core.FirewallRule) ([]ec2types.IpPermission, error) {
	segments := awsPortSegments(rule.PortRange)
	if len(segments) == 0 {
		segments = []string{rule.PortRange}
	}
	perms := make([]ec2types.IpPermission, 0, len(segments))
	for _, segment := range segments {
		perm, err := toAWSPermission(rule, segment)
		if err != nil {
			return nil, err
		}
		perms = append(perms, perm)
	}
	return perms, nil
}

func toAWSPermission(rule core.FirewallRule, portRange string) (ec2types.IpPermission, error) {
	protocol := strings.ToLower(awsCleanRuleValue(rule.Protocol))
	if protocol == "" || protocol == "all" || protocol == "*" {
		protocol = "-1"
	}

	perm := ec2types.IpPermission{
		IpProtocol: awssdk.String(protocol),
	}

	if protocol != "-1" {
		if fromPort, toPort, ok := awsPortRangeBounds(portRange); ok {
			perm.FromPort = awssdk.Int32(fromPort)
			perm.ToPort = awssdk.Int32(toPort)
		} else if cleaned := awsCleanRuleValue(portRange); cleaned != "" && !strings.EqualFold(cleaned, "all") {
			return ec2types.IpPermission{}, fmt.Errorf("invalid AWS port or range %q", portRange)
		}
	}

	peer := rule.Source
	if rule.Direction == "Outbound" {
		peer = rule.Destination
	}
	peer = awsCleanRuleValue(peer)
	desc := awsCleanRuleValue(rule.Description)

	// Determine if peer is CIDR, Security Group, or Prefix List
	if strings.Contains(peer, "/") {
		// CIDR
		if strings.Contains(peer, ":") {
			ipv6Range := ec2types.Ipv6Range{CidrIpv6: awssdk.String(peer)}
			if desc != "" {
				ipv6Range.Description = awssdk.String(desc)
			}
			perm.Ipv6Ranges = []ec2types.Ipv6Range{ipv6Range}
		} else {
			ipRange := ec2types.IpRange{CidrIp: awssdk.String(peer)}
			if desc != "" {
				ipRange.Description = awssdk.String(desc)
			}
			perm.IpRanges = []ec2types.IpRange{ipRange}
		}
	} else if strings.HasPrefix(peer, "pl-") {
		// Prefix List
		prefixList := ec2types.PrefixListId{PrefixListId: awssdk.String(peer)}
		if desc != "" {
			prefixList.Description = awssdk.String(desc)
		}
		perm.PrefixListIds = []ec2types.PrefixListId{prefixList}
	} else {
		// Security Group
		// It might be "name (sg-id)" or just "sg-id"
		sgID := peer
		if strings.Contains(peer, "(") && strings.Contains(peer, ")") {
			start := strings.Index(peer, "(")
			end := strings.Index(peer, ")")
			sgID = peer[start+1 : end]
		}
		if strings.HasPrefix(sgID, "sg-") {
			groupPair := ec2types.UserIdGroupPair{GroupId: awssdk.String(sgID)}
			if desc != "" {
				groupPair.Description = awssdk.String(desc)
			}
			perm.UserIdGroupPairs = []ec2types.UserIdGroupPair{groupPair}
		} else {
			return ec2types.IpPermission{}, fmt.Errorf("AWS peer must be a CIDR, sg-..., or pl-...")
		}
	}

	return perm, nil
}

func awsRulePeerValue(rule core.FirewallRule) string {
	peer := rule.Source
	if strings.EqualFold(strings.TrimSpace(rule.Direction), core.Outbound) {
		peer = rule.Destination
	}
	return awsCleanRuleValue(peer)
}

func awsRulePeerKind(rule core.FirewallRule) string {
	return awsPeerKindFromValue(awsRulePeerValue(rule))
}

func awsPeerKindFromValue(peer string) string {
	peer = strings.TrimSpace(peer)
	switch {
	case peer == "":
		return ""
	case strings.Contains(peer, "/") && strings.Contains(peer, ":"):
		return "ipv6"
	case strings.Contains(peer, "/"):
		return "ipv4"
	case strings.HasPrefix(peer, "pl-"):
		return "prefix-list"
	case strings.HasPrefix(awsReferencedGroupID(peer), "sg-"):
		return "sg"
	default:
		return ""
	}
}

func awsReferencedGroupID(peer string) string {
	peer = strings.TrimSpace(peer)
	if strings.Contains(peer, "(") && strings.Contains(peer, ")") {
		start := strings.LastIndex(peer, "(")
		end := strings.LastIndex(peer, ")")
		if start >= 0 && end > start {
			return strings.TrimSpace(peer[start+1 : end])
		}
	}
	return peer
}

func awsPortRangeBounds(portRange string) (int32, int32, bool) {
	portRange = awsCleanRuleValue(portRange)
	if portRange == "" || strings.EqualFold(portRange, "all") {
		return 0, 0, false
	}
	if strings.Contains(portRange, "-") {
		parts := strings.SplitN(portRange, "-", 2)
		from, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
		to, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err1 != nil || err2 != nil {
			return 0, 0, false
		}
		return int32(from), int32(to), true
	}
	port, err := strconv.Atoi(strings.TrimSpace(portRange))
	if err != nil {
		return 0, 0, false
	}
	return int32(port), int32(port), true
}

func awsPortSegments(portRange string) []string {
	portRange = awsCleanRuleValue(portRange)
	if portRange == "" || strings.EqualFold(portRange, "all") {
		return nil
	}
	parts := strings.Split(portRange, ",")
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		part = awsCleanRuleValue(part)
		if part != "" {
			segments = append(segments, part)
		}
	}
	return segments
}

func awsCleanRuleValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "-" {
		return ""
	}
	return value
}
