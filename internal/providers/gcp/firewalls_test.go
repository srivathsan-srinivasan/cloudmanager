package gcp

import (
	"strings"
	"testing"

	"google.golang.org/api/compute/v1"
)

func TestGCPFirewallRuleToCoreIngressAllowAndDeny(t *testing.T) {
	rule := &compute.Firewall{
		Name:         "allow-web",
		Network:      "https://www.googleapis.com/compute/v1/projects/p1/global/networks/prod-vpc",
		Direction:    "INGRESS",
		SourceRanges: []string{"0.0.0.0/0"},
		Allowed: []*compute.FirewallAllowed{
			{IPProtocol: "tcp", Ports: []string{"80", "443"}},
		},
		Denied: []*compute.FirewallDenied{
			{IPProtocol: "tcp", Ports: []string{"22"}},
		},
		Priority:    1000,
		Description: "public access",
	}

	rows := gcpFirewallRuleToCore(rule, "prod-vpc")
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	if rows[0].Action != "Allow" || rows[0].Protocol != "tcp" || rows[0].PortRange != "80,443" {
		t.Fatalf("unexpected allow row: %#v", rows[0])
	}
	if rows[0].Source != "0.0.0.0/0" || rows[0].Destination != "prod-vpc" {
		t.Fatalf("unexpected allow endpoints: %#v", rows[0])
	}
	if rows[1].Action != "Deny" || rows[1].PortRange != "22" {
		t.Fatalf("unexpected deny row: %#v", rows[1])
	}
}

func TestGCPRuleOpensPort(t *testing.T) {
	rule := &compute.Firewall{
		Direction:    "INGRESS",
		SourceRanges: []string{"0.0.0.0/0"},
		Allowed: []*compute.FirewallAllowed{
			{IPProtocol: "tcp", Ports: []string{"22"}},
		},
	}

	if !gcpRuleOpensPort(rule, 22) {
		t.Fatal("expected ingress rule to be marked as open ssh")
	}
	if gcpRuleOpensPort(rule, 3389) {
		t.Fatal("did not expect ingress rule to be marked as open rdp")
	}
}

func TestShortNamePrefersExplicitName(t *testing.T) {
	if got := shortName("prod-vpc", "https://example/networks/ignored"); got != "prod-vpc" {
		t.Fatalf("unexpected short name: %s", got)
	}
	if got := shortName("", "https://example/networks/prod-vpc"); got != "prod-vpc" {
		t.Fatalf("unexpected derived short name: %s", got)
	}
}

func TestAccumulateGCPNetworkAttachmentCounts(t *testing.T) {
	counts := make(map[string]int)
	accumulateGCPNetworkAttachmentCounts(counts, map[string]compute.InstancesScopedList{
		"zones/us-east1-b": {
			Instances: []*compute.Instance{
				{
					Name: "web-1",
					NetworkInterfaces: []*compute.NetworkInterface{
						{Network: "https://www.googleapis.com/compute/v1/projects/p1/global/networks/prod-vpc"},
						{Network: "https://www.googleapis.com/compute/v1/projects/p1/global/networks/shared-vpc"},
					},
				},
				{
					Name: "web-2",
					NetworkInterfaces: []*compute.NetworkInterface{
						{Network: "https://www.googleapis.com/compute/v1/projects/p1/global/networks/prod-vpc"},
					},
				},
			},
		},
	})

	if counts["prod-vpc"] != 2 {
		t.Fatalf("expected prod-vpc attachment count 2, got %d", counts["prod-vpc"])
	}
	if counts["shared-vpc"] != 1 {
		t.Fatalf("expected shared-vpc attachment count 1, got %d", counts["shared-vpc"])
	}
}

func TestGCPFirewallRuleToCoreIncludesTags(t *testing.T) {
	rule := &compute.Firewall{
		Name:                  "allow-app",
		Network:               "https://www.googleapis.com/compute/v1/projects/p1/global/networks/prod-vpc",
		Direction:             "INGRESS",
		SourceRanges:          []string{"10.0.0.0/8"},
		SourceTags:            []string{"batch"},
		TargetTags:            []string{"web"},
		TargetServiceAccounts: []string{"app@project.iam.gserviceaccount.com"},
		Allowed: []*compute.FirewallAllowed{
			{IPProtocol: "tcp", Ports: []string{"443"}},
		},
	}

	rows := gcpFirewallRuleToCore(rule, "prod-vpc")
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if !strings.Contains(rows[0].Source, "tag:batch") {
		t.Fatalf("expected source tags in source field, got %#v", rows[0])
	}
	if !strings.Contains(rows[0].Destination, "tag:web") || !strings.Contains(rows[0].Destination, "sa:app@project.iam.gserviceaccount.com") {
		t.Fatalf("expected target selectors in destination field, got %#v", rows[0])
	}
}

func TestGCPEffectiveFirewallRulesIncludePolicies(t *testing.T) {
	resp := &compute.NetworksGetEffectiveFirewallsResponse{
		FirewallPolicys: []*compute.NetworksGetEffectiveFirewallsResponseEffectiveFirewallPolicy{
			{
				ShortName: "prod-policy",
				Type:      "NETWORK",
				Rules: []*compute.FirewallPolicyRule{
					{
						Action:      "allow",
						Direction:   "INGRESS",
						Priority:    900,
						Description: "policy ingress",
						TargetSecureTags: []*compute.FirewallPolicyRuleSecureTag{
							{Name: "env/prod"},
						},
						Match: &compute.FirewallPolicyRuleMatcher{
							SrcIpRanges: []string{"0.0.0.0/0"},
							Layer4Configs: []*compute.FirewallPolicyRuleMatcherLayer4Config{
								{IpProtocol: "tcp", Ports: []string{"443"}},
							},
						},
					},
				},
			},
		},
	}

	rules := gcpEffectiveFirewallRulesToCore(resp, "prod-vpc")
	if len(rules) != 1 {
		t.Fatalf("expected 1 policy rule, got %d", len(rules))
	}
	if rules[0].Action != "Allow" || rules[0].Priority != 900 {
		t.Fatalf("unexpected policy rule: %#v", rules[0])
	}
	if !strings.Contains(rules[0].Destination, "secure:env/prod") {
		t.Fatalf("expected secure target tags in destination, got %#v", rules[0])
	}
	if !strings.Contains(rules[0].Description, "prod-policy") {
		t.Fatalf("expected policy name in description, got %#v", rules[0])
	}
}

func TestGCPCombinedFirewallRulesIncludeClassicTagsAndPolicies(t *testing.T) {
	classic := []*compute.Firewall{
		{
			Name:       "allow-app",
			Network:    "https://www.googleapis.com/compute/v1/projects/p1/global/networks/prod-vpc",
			Direction:  "INGRESS",
			SourceTags: []string{"batch"},
			TargetTags: []string{"web"},
			Allowed: []*compute.FirewallAllowed{
				{IPProtocol: "tcp", Ports: []string{"443"}},
			},
		},
	}
	effective := &compute.NetworksGetEffectiveFirewallsResponse{
		FirewallPolicys: []*compute.NetworksGetEffectiveFirewallsResponseEffectiveFirewallPolicy{
			{
				ShortName: "prod-policy",
				Type:      "NETWORK",
				Rules: []*compute.FirewallPolicyRule{
					{
						Action:    "allow",
						Direction: "INGRESS",
						Priority:  900,
						TargetSecureTags: []*compute.FirewallPolicyRuleSecureTag{
							{Name: "env/prod"},
						},
						Match: &compute.FirewallPolicyRuleMatcher{
							SrcIpRanges: []string{"0.0.0.0/0"},
							Layer4Configs: []*compute.FirewallPolicyRuleMatcherLayer4Config{
								{IpProtocol: "tcp", Ports: []string{"8443"}},
							},
						},
					},
				},
			},
		},
	}

	rules := append(gcpClassicFirewallRulesToCore(classic, "prod-vpc"), gcpEffectivePolicyRulesToCore(effective, "prod-vpc")...)
	rules = dedupeFirewallRules(rules)
	if len(rules) != 2 {
		t.Fatalf("expected classic and policy rules, got %d: %#v", len(rules), rules)
	}

	if !strings.Contains(rules[0].Source+rules[1].Source, "tag:batch") {
		t.Fatalf("expected classic network tag to survive merge, got %#v", rules)
	}
	if !strings.Contains(rules[0].Destination+rules[1].Destination, "secure:env/prod") {
		t.Fatalf("expected policy secure tag to survive merge, got %#v", rules)
	}
}
