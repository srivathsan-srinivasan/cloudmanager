package core

import (
	"fmt"
	"strings"
)

// SecurityGroup represents a firewall grouping construct across providers.
type SecurityGroup struct {
	Name              string
	ID                string
	Description       string
	NetworkID         string
	NetworkName       string
	InboundRuleCount  int
	OutboundRuleCount int
	AttachedResources int
	HasOpenSSH        bool
	HasOpenRDP        bool
	Provider          string
	Region            string
	ResourceGroup     string
	Labels            string
}

func (s SecurityGroup) HasAuditRisk() bool {
	return s.HasOpenSSH || s.HasOpenRDP
}

var DefaultSecurityGroupColumns = []string{
	"Name", "ID", "Network", "Inbound Rules", "Outbound Rules", "Attached", "Description",
}

func (s SecurityGroup) GetID() string   { return s.ID }
func (s SecurityGroup) GetName() string { return s.Name }
func (s SecurityGroup) GetKind() string { return "Security Group" }

func (s SecurityGroup) GetField(col string) string {
	switch col {
	case "Name":
		return s.Name
	case "ID":
		return s.ID
	case "Network":
		if s.NetworkName != "" && s.NetworkName != "-" {
			return s.NetworkName
		}
		return s.NetworkID
	case "Inbound Rules":
		return fmt.Sprintf("%d", s.InboundRuleCount)
	case "Outbound Rules":
		return fmt.Sprintf("%d", s.OutboundRuleCount)
	case "Attached":
		return fmt.Sprintf("%d", s.AttachedResources)
	case "Description":
		return s.Description
	case "Provider":
		return s.Provider
	case "Region":
		return s.Region
	case "Resource Group":
		return s.ResourceGroup
	case "Labels":
		return s.Labels
	case "Audit":
		switch {
		case s.HasOpenSSH && s.HasOpenRDP:
			return "Open SSH,RDP"
		case s.HasOpenSSH:
			return "Open SSH"
		case s.HasOpenRDP:
			return "Open RDP"
		default:
			return "-"
		}
	default:
		return "-"
	}
}

func SecurityGroupActions() []Action {
	return []Action{
		{"View Rules", "Inspect inbound and outbound firewall rules", false},
		{"Describe", "Show full resource details", false},
		{"Open Console", "Open this firewall resource in the provider console", false},
	}
}

func DescribeSecurityGroup(s SecurityGroup) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Name:              %s\n", s.Name)
	fmt.Fprintf(&b, "ID:                %s\n", s.ID)
	fmt.Fprintf(&b, "Description:       %s\n", s.Description)
	fmt.Fprintf(&b, "Network:           %s\n", s.GetField("Network"))
	fmt.Fprintf(&b, "Inbound Rules:     %d\n", s.InboundRuleCount)
	fmt.Fprintf(&b, "Outbound Rules:    %d\n", s.OutboundRuleCount)
	fmt.Fprintf(&b, "Attached:          %d\n", s.AttachedResources)
	fmt.Fprintf(&b, "Provider:          %s\n", s.Provider)
	fmt.Fprintf(&b, "Region:            %s\n", s.Region)
	fmt.Fprintf(&b, "Resource Group:    %s\n", s.ResourceGroup)
	fmt.Fprintf(&b, "Labels:            %s\n", s.Labels)
	fmt.Fprintf(&b, "Audit:             %s\n", s.GetField("Audit"))
	return b.String()
}
