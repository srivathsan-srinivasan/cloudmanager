package aws

import (
	"context"
	"fmt"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"cloudmanager/internal/core"
)

func FetchSecurityGroupsSDK(ctx context.Context, profile, region string) ([]core.SecurityGroup, error) {
	cfg, err := getAWSConfig(ctx, profile, region)
	if err != nil {
		return nil, fmt.Errorf("failed to load aws config: %w", err)
	}

	client := ec2.NewFromConfig(cfg)
	attachmentCounts, err := awsSecurityGroupAttachmentCounts(ctx, client)
	if err != nil {
		return nil, err
	}

	paginator := ec2.NewDescribeSecurityGroupsPaginator(client, &ec2.DescribeSecurityGroupsInput{})
	var groups []core.SecurityGroup
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe security groups: %w", err)
		}
		for _, group := range page.SecurityGroups {
			groupID := awssdk.ToString(group.GroupId)
			groups = append(groups, awsSecurityGroupToCore(group, region, attachmentCounts[groupID]))
		}
	}

	return groups, nil
}

func FetchFirewallRulesSDK(ctx context.Context, profile, region, groupID string) ([]core.FirewallRule, error) {
	cfg, err := getAWSConfig(ctx, profile, region)
	if err != nil {
		return nil, fmt.Errorf("failed to load aws config: %w", err)
	}

	client := ec2.NewFromConfig(cfg)
	resp, err := client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
		GroupIds: []string{groupID},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe security group %s: %w", groupID, err)
	}
	if len(resp.SecurityGroups) == 0 {
		return nil, fmt.Errorf("security group %s not found", groupID)
	}

	return awsSecurityGroupRules(resp.SecurityGroups[0]), nil
}

func awsSecurityGroupAttachmentCounts(ctx context.Context, client *ec2.Client) (map[string]int, error) {
	counts := make(map[string]int)
	paginator := ec2.NewDescribeNetworkInterfacesPaginator(client, &ec2.DescribeNetworkInterfacesInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe network interfaces: %w", err)
		}
		for _, iface := range page.NetworkInterfaces {
			seen := make(map[string]bool)
			for _, group := range iface.Groups {
				groupID := awssdk.ToString(group.GroupId)
				if groupID == "" || seen[groupID] {
					continue
				}
				counts[groupID]++
				seen[groupID] = true
			}
		}
	}
	return counts, nil
}

func awsSecurityGroupToCore(group ec2types.SecurityGroup, region string, attached int) core.SecurityGroup {
	name := orDash(awssdk.ToString(group.GroupName))
	labels := make([]string, 0, len(group.Tags))
	for _, tag := range group.Tags {
		labels = append(labels, fmt.Sprintf("%s=%s", awssdk.ToString(tag.Key), awssdk.ToString(tag.Value)))
	}
	hasOpenSSH, hasOpenRDP := awsAuditPermissions(group.IpPermissions)

	return core.SecurityGroup{
		Name:              name,
		ID:                orDash(awssdk.ToString(group.GroupId)),
		Description:       orDash(awssdk.ToString(group.Description)),
		NetworkID:         orDash(awssdk.ToString(group.VpcId)),
		NetworkName:       orDash(awssdk.ToString(group.VpcId)),
		InboundRuleCount:  len(group.IpPermissions),
		OutboundRuleCount: len(group.IpPermissionsEgress),
		AttachedResources: attached,
		HasOpenSSH:        hasOpenSSH,
		HasOpenRDP:        hasOpenRDP,
		Provider:          "AWS",
		Region:            region,
		Labels:            strings.Join(labels, ", "),
	}
}

func awsSecurityGroupRules(group ec2types.SecurityGroup) []core.FirewallRule {
	groupID := awssdk.ToString(group.GroupId)
	groupName := awssdk.ToString(group.GroupName)
	networkID := awssdk.ToString(group.VpcId)

	var rules []core.FirewallRule
	priority := 1
	priority = appendAWSPermissionRules(&rules, groupID, groupName, networkID, "Inbound", group.IpPermissions, priority)
	appendAWSPermissionRules(&rules, groupID, groupName, networkID, "Outbound", group.IpPermissionsEgress, priority)
	return rules
}

func appendAWSPermissionRules(dst *[]core.FirewallRule, groupID, groupName, networkID, direction string, permissions []ec2types.IpPermission, priority int) int {
	for _, permission := range permissions {
		protocol := awsPermissionProtocol(permission)
		portRange := awsPermissionPortRange(permission)
		peers := awsPermissionPeers(permission)
		if len(peers) == 0 {
			peers = []awsRulePeer{{value: "-", description: "-"}}
		}

		for _, peer := range peers {
			source := groupName
			destination := peer.value
			if direction == "Inbound" {
				source = peer.value
				destination = groupName
			}

			*dst = append(*dst, core.FirewallRule{
				ID:           fmt.Sprintf("%s-%s-%d", groupID, strings.ToLower(direction[:1]), priority),
				Direction:    direction,
				Protocol:     protocol,
				PortRange:    portRange,
				Source:       source,
				Destination:  destination,
				Action:       "Allow",
				Priority:     priority,
				Description:  peer.description,
				ResourceID:   groupID,
				ResourceName: groupName,
				NetworkID:    networkID,
				Provider:     "AWS",
			})
			priority++
		}
	}
	return priority
}

type awsRulePeer struct {
	value       string
	description string
}

func awsPermissionPeers(permission ec2types.IpPermission) []awsRulePeer {
	var peers []awsRulePeer

	for _, ipRange := range permission.IpRanges {
		peers = append(peers, awsRulePeer{
			value:       orDash(awssdk.ToString(ipRange.CidrIp)),
			description: orDash(awssdk.ToString(ipRange.Description)),
		})
	}
	for _, ipRange := range permission.Ipv6Ranges {
		peers = append(peers, awsRulePeer{
			value:       orDash(awssdk.ToString(ipRange.CidrIpv6)),
			description: orDash(awssdk.ToString(ipRange.Description)),
		})
	}
	for _, groupPair := range permission.UserIdGroupPairs {
		peer := awssdk.ToString(groupPair.GroupId)
		if groupName := awssdk.ToString(groupPair.GroupName); groupName != "" {
			peer = fmt.Sprintf("%s (%s)", groupName, awssdk.ToString(groupPair.GroupId))
		}
		peers = append(peers, awsRulePeer{
			value:       orDash(peer),
			description: orDash(awssdk.ToString(groupPair.Description)),
		})
	}
	for _, prefix := range permission.PrefixListIds {
		peers = append(peers, awsRulePeer{
			value:       orDash(awssdk.ToString(prefix.PrefixListId)),
			description: orDash(awssdk.ToString(prefix.Description)),
		})
	}

	return peers
}

func awsPermissionProtocol(permission ec2types.IpPermission) string {
	switch proto := awssdk.ToString(permission.IpProtocol); proto {
	case "", "-1":
		return "all"
	default:
		return strings.ToLower(proto)
	}
}

func awsPermissionPortRange(permission ec2types.IpPermission) string {
	protocol := awsPermissionProtocol(permission)
	if protocol == "all" {
		return "All"
	}
	if permission.FromPort == nil || permission.ToPort == nil {
		return "All"
	}

	from := awssdk.ToInt32(permission.FromPort)
	to := awssdk.ToInt32(permission.ToPort)
	if from == to {
		return fmt.Sprintf("%d", from)
	}
	return fmt.Sprintf("%d-%d", from, to)
}

func awsAuditPermissions(permissions []ec2types.IpPermission) (bool, bool) {
	var hasOpenSSH bool
	var hasOpenRDP bool
	for _, permission := range permissions {
		if !awsPermissionOpenToInternet(permission) {
			continue
		}
		if awsPermissionMatchesPort(permission, 22) {
			hasOpenSSH = true
		}
		if awsPermissionMatchesPort(permission, 3389) {
			hasOpenRDP = true
		}
	}
	return hasOpenSSH, hasOpenRDP
}

func awsPermissionOpenToInternet(permission ec2types.IpPermission) bool {
	for _, ipRange := range permission.IpRanges {
		if awssdk.ToString(ipRange.CidrIp) == "0.0.0.0/0" {
			return true
		}
	}
	for _, ipRange := range permission.Ipv6Ranges {
		if awssdk.ToString(ipRange.CidrIpv6) == "::/0" {
			return true
		}
	}
	return false
}

func awsPermissionMatchesPort(permission ec2types.IpPermission, port int32) bool {
	protocol := awsPermissionProtocol(permission)
	if protocol == "all" {
		return true
	}
	if protocol != "tcp" && protocol != "udp" {
		return false
	}
	if permission.FromPort == nil || permission.ToPort == nil {
		return true
	}
	return awssdk.ToInt32(permission.FromPort) <= port && awssdk.ToInt32(permission.ToPort) >= port
}
