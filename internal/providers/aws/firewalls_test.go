package aws

import (
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"cloudmanager/internal/core"
)

func TestAWSSecurityGroupRuleToCoreUsesRealRuleID(t *testing.T) {
	rule := ec2types.SecurityGroupRule{
		SecurityGroupRuleId: awssdk.String("sgr-123"),
		GroupId:             awssdk.String("sg-main"),
		IpProtocol:          awssdk.String("tcp"),
		FromPort:            awssdk.Int32(443),
		ToPort:              awssdk.Int32(443),
		CidrIpv4:            awssdk.String("0.0.0.0/0"),
		Description:         awssdk.String("public https"),
	}

	got := awsSecurityGroupRuleToCore(rule, "sg-main", "web-sg", "vpc-1", 7)
	if got.ID != "sgr-123" {
		t.Fatalf("expected AWS rule ID to round-trip, got %#v", got)
	}
	if got.Direction != core.Inbound {
		t.Fatalf("expected inbound direction, got %#v", got)
	}
	if got.Source != "0.0.0.0/0" || got.Destination != "web-sg" {
		t.Fatalf("unexpected AWS endpoints: %#v", got)
	}
}

func TestToAWSSecurityGroupRuleRequest(t *testing.T) {
	rule := core.FirewallRule{
		Direction:   core.Inbound,
		Protocol:    "tcp",
		PortRange:   "80-90",
		Source:      "10.0.0.0/8",
		Description: "web",
	}

	req, err := toAWSSecurityGroupRuleRequest(rule)
	if err != nil {
		t.Fatalf("expected AWS rule request to build, got error: %v", err)
	}
	if awssdk.ToString(req.IpProtocol) != "tcp" {
		t.Fatalf("unexpected protocol: %#v", req)
	}
	if awssdk.ToInt32(req.FromPort) != 80 || awssdk.ToInt32(req.ToPort) != 90 {
		t.Fatalf("unexpected port range: %#v", req)
	}
	if awssdk.ToString(req.CidrIpv4) != "10.0.0.0/8" {
		t.Fatalf("unexpected cidr: %#v", req)
	}
	if awssdk.ToString(req.Description) != "web" {
		t.Fatalf("unexpected description: %#v", req)
	}
}

func TestToAWSSecurityGroupRuleRequestRejectsMultiplePorts(t *testing.T) {
	rule := core.FirewallRule{
		Direction: core.Inbound,
		Protocol:  "tcp",
		PortRange: "443,8443",
		Source:    "10.0.0.0/8",
	}

	_, err := toAWSSecurityGroupRuleRequest(rule)
	if err == nil {
		t.Fatal("expected multi-port AWS edit request to be rejected for in-place modify")
	}
}

func TestCanModifyAWSRuleInPlaceRequiresRealRuleIDAndSamePeerKind(t *testing.T) {
	original := core.FirewallRule{
		ID:        "sgr-123",
		Direction: core.Inbound,
		PortRange: "443",
		Source:    "10.0.0.0/8",
	}
	updated := core.FirewallRule{
		ID:        "sgr-123",
		Direction: core.Inbound,
		PortRange: "443",
		Source:    "192.168.0.0/16",
	}
	if !canModifyAWSRuleInPlace(original, updated) {
		t.Fatal("expected in-place AWS modify to be allowed for same rule kind")
	}

	updated.Source = "sg-999"
	if canModifyAWSRuleInPlace(original, updated) {
		t.Fatal("did not expect in-place AWS modify across peer kinds")
	}
}

func TestCanModifyAWSRuleInPlaceRejectsMultiplePorts(t *testing.T) {
	original := core.FirewallRule{
		ID:        "sgr-123",
		Direction: core.Inbound,
		PortRange: "443",
		Source:    "10.0.0.0/8",
	}
	updated := core.FirewallRule{
		ID:        "sgr-123",
		Direction: core.Inbound,
		Source:    "10.0.0.0/8",
		PortRange: "443,8443",
	}

	if canModifyAWSRuleInPlace(original, updated) {
		t.Fatal("did not expect in-place AWS modify for comma-separated ports")
	}
}

func TestToAWSPermissionsSplitsCommaSeparatedPorts(t *testing.T) {
	rule := core.FirewallRule{
		Direction:   core.Inbound,
		Protocol:    "tcp",
		PortRange:   "443, 8443",
		Source:      "0.0.0.0/0",
		Description: "https",
	}

	perms, err := toAWSPermissions(rule)
	if err != nil {
		t.Fatalf("expected AWS permissions to build, got error: %v", err)
	}
	if len(perms) != 2 {
		t.Fatalf("expected two permissions for comma-separated ports, got %d", len(perms))
	}
	if awssdk.ToInt32(perms[0].FromPort) != 443 || awssdk.ToInt32(perms[1].FromPort) != 8443 {
		t.Fatalf("unexpected split port permissions: %#v", perms)
	}
}
