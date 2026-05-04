package core

import (
	"fmt"
)

// Subnet represents a sub-network within a VPC/VNet.
type Subnet struct {
	Name, ID, State, CIDRBlock string
	AvailabilityZone           string
	NetworkID, NetworkName     string
	AvailableIPs               int
	MapPublicIPOnLaunch        bool
	Provider, Region           string
	ResourceGroup, Labels      string
}

// --- Resource interface implementation ---

func (s Subnet) GetID() string   { return s.ID }
func (s Subnet) GetName() string { return s.Name }
func (s Subnet) GetKind() string { return "Subnet" }

func (s Subnet) GetField(col string) string {
	switch col {
	case "Name":
		return s.Name
	case "ID":
		return s.ID
	case "CIDR":
		return s.CIDRBlock
	case "AZ":
		return s.AvailabilityZone
	case "Network":
		return s.NetworkName
	case "Network ID":
		return s.NetworkID
	case "Available IPs":
		return fmt.Sprintf("%d", s.AvailableIPs)
	case "Public IP on Launch":
		if s.MapPublicIPOnLaunch {
			return "Yes"
		}
		return "No"
	case "State":
		return s.State
	case "Region":
		return s.Region
	case "Resource Group":
		return s.ResourceGroup
	case "Labels":
		return s.Labels
	default:
		return "-"
	}
}

// DefaultSubnetColumns is the canonical list of all possible columns for subnets.
var DefaultSubnetColumns = []string{
	"Name", "ID", "CIDR", "AZ", "Network", "Available IPs",
}

// SubnetActions returns the list of available actions for a subnet.
func SubnetActions() []Action {
	return []Action{
		{"View VMs", "View instances within this subnet", false},
		{"Describe", "View full resource details", false},
	}
}
