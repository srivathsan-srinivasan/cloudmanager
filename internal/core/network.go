package core

import (
	"fmt"
)

// Network represents a virtual private network (VPC/VNet).
type Network struct {
	Name, ID, State, CIDRBlock string
	IsDefault                  bool
	SubnetCount                int
	Provider, Region           string
	ResourceGroup, Labels      string
}

// --- Resource interface implementation ---

func (n Network) GetID() string   { return n.ID }
func (n Network) GetName() string { return n.Name }
func (n Network) GetKind() string { return "Network" }

func (n Network) GetField(col string) string {
	switch col {
	case "Name":
		return n.Name
	case "ID":
		return n.ID
	case "State":
		return n.State
	case "CIDR":
		return n.CIDRBlock
	case "Default":
		if n.IsDefault {
			return "Yes"
		}
		return "No"
	case "Subnets":
		return fmt.Sprintf("%d", n.SubnetCount)
	case "Region":
		return n.Region
	case "Resource Group":
		return n.ResourceGroup
	case "Labels":
		return n.Labels
	default:
		return "-"
	}
}

// DefaultNetworkColumns is the canonical list of all possible columns for networks.
var DefaultNetworkColumns = []string{
	"Name", "ID", "State", "CIDR", "Default", "Subnets", "Region",
}

// NetworkActions returns the list of available actions for a network.
func NetworkActions() []Action {
	return []Action{
		{"View Subnets", "View subnets within this network", false},
		{"Describe", "View full resource details", false},
		{"Open Console", "Open this network in the provider console", false},
	}
}
