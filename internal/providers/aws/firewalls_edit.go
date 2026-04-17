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

		// Revoke original rule
		err = revokeAWSRule(ctx, client, sgID, *rule.OriginalRule)
		if err != nil {
			return "", fmt.Errorf("failed to revoke original rule: %w", err)
		}

		// Authorize new rule
		err = authorizeAWSRule(ctx, client, sgID, rule)
		if err != nil {
			// Try to roll back? AWS doesn't have transactions. 
			// If authorize fails, the original rule is already gone.
			return "", fmt.Errorf("failed to authorize updated rule: %w", err)
		}

		applog.Infof("AUDIT: user modified firewall rule: edited rule %s in SG %s", rule.ID, sgID)
		return fmt.Sprintf("Successfully updated rule %s in %s", rule.ID, sgID), nil

	default:
		return "", fmt.Errorf("action %s not supported for AWS Firewalls via CloudManager SDK", action)
	}
}

func revokeAWSRule(ctx context.Context, client *ec2.Client, sgID string, rule core.FirewallRule) error {
	perm := toAWSPermission(rule)
	if rule.Direction == "Inbound" {
		_, err := client.RevokeSecurityGroupIngress(ctx, &ec2.RevokeSecurityGroupIngressInput{
			GroupId:       awssdk.String(sgID),
			IpPermissions: []ec2types.IpPermission{perm},
		})
		return err
	} else {
		_, err := client.RevokeSecurityGroupEgress(ctx, &ec2.RevokeSecurityGroupEgressInput{
			GroupId:       awssdk.String(sgID),
			IpPermissions: []ec2types.IpPermission{perm},
		})
		return err
	}
}

func authorizeAWSRule(ctx context.Context, client *ec2.Client, sgID string, rule core.FirewallRule) error {
	perm := toAWSPermission(rule)
	if rule.Direction == "Inbound" {
		_, err := client.AuthorizeSecurityGroupIngress(ctx, &ec2.AuthorizeSecurityGroupIngressInput{
			GroupId:       awssdk.String(sgID),
			IpPermissions: []ec2types.IpPermission{perm},
		})
		return err
	} else {
		_, err := client.AuthorizeSecurityGroupEgress(ctx, &ec2.AuthorizeSecurityGroupEgressInput{
			GroupId:       awssdk.String(sgID),
			IpPermissions: []ec2types.IpPermission{perm},
		})
		return err
	}
}

func toAWSPermission(rule core.FirewallRule) ec2types.IpPermission {
	protocol := rule.Protocol
	if protocol == "" || strings.EqualFold(protocol, "all") {
		protocol = "-1"
	}

	perm := ec2types.IpPermission{
		IpProtocol: awssdk.String(strings.ToLower(protocol)),
	}

	if protocol != "-1" {
		if rule.PortRange != "" && !strings.EqualFold(rule.PortRange, "all") && rule.PortRange != "-" {
			if strings.Contains(rule.PortRange, "-") {
				parts := strings.Split(rule.PortRange, "-")
				if from, err := strconv.Atoi(parts[0]); err == nil {
					perm.FromPort = awssdk.Int32(int32(from))
				}
				if to, err := strconv.Atoi(parts[1]); err == nil {
					perm.ToPort = awssdk.Int32(int32(to))
				}
			} else {
				if p, err := strconv.Atoi(rule.PortRange); err == nil {
					perm.FromPort = awssdk.Int32(int32(p))
					perm.ToPort = awssdk.Int32(int32(p))
				}
			}
		}
	}

	peer := rule.Source
	if rule.Direction == "Outbound" {
		peer = rule.Destination
	}

	// Determine if peer is CIDR, Security Group, or Prefix List
	if strings.Contains(peer, "/") {
		// CIDR
		if strings.Contains(peer, ":") {
			perm.Ipv6Ranges = []ec2types.Ipv6Range{{CidrIpv6: awssdk.String(peer), Description: awssdk.String(rule.Description)}}
		} else {
			perm.IpRanges = []ec2types.IpRange{{CidrIp: awssdk.String(peer), Description: awssdk.String(rule.Description)}}
		}
	} else if strings.HasPrefix(peer, "pl-") {
		// Prefix List
		perm.PrefixListIds = []ec2types.PrefixListId{{PrefixListId: awssdk.String(peer), Description: awssdk.String(rule.Description)}}
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
			perm.UserIdGroupPairs = []ec2types.UserIdGroupPair{{GroupId: awssdk.String(sgID), Description: awssdk.String(rule.Description)}}
		}
	}

	return perm
}
