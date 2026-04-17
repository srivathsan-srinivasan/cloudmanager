package core

import (
	"fmt"
	"strings"
)

// VM represents a virtual machine across any cloud provider.
// Implements the Resource interface.
type VM struct {
	Name           string
	ID             string
	Type           string
	State          string
	PrivateIP      string
	PublicIP       string
	Zone           string
	ResourceGroup  string
	Network        string
	Subnet         string
	Labels         string
	SecurityGroups string
	MonthlyCost    string
	CostTrend      string
	CPUPercent     string
	MemoryPercent  string
	DiskIORead     string
	DiskIOWrite    string
	NetworkIn      string
	NetworkOut     string
	Recommendation string
	EstSavings     string
}

// DefaultVMColumns is the canonical list of all possible columns for VMs.
var DefaultVMColumns = []string{
	"Name", "Instance ID", "Type", "State", "Private IP", "Public IP",
	"Network", "Subnet", "Security Groups", "Zone", "Resource Group", "Labels",
	"CPU %", "Memory %", "Disk Read", "Disk Write", "Net In", "Net Out",
	"Cost", "Cost Trend", "Recommendation", "Est. Savings",
}

// --- Resource interface implementation ---

func (v VM) GetID() string   { return v.ID }
func (v VM) GetName() string { return v.Name }
func (v VM) GetKind() string { return "VM" }

func (v VM) GetField(col string) string {
	switch col {
	case "Name":
		return v.Name
	case "Instance ID":
		return v.ID
	case "Type":
		return v.Type
	case "State":
		return v.State
	case "Private IP":
		return v.PrivateIP
	case "Public IP":
		return v.PublicIP
	case "Zone":
		return v.Zone
	case "Resource Group":
		return v.ResourceGroup
	case "Network":
		return v.Network
	case "Subnet":
		return v.Subnet
	case "Labels":
		return v.Labels
	case "Security Groups":
		return v.SecurityGroups
	case "Cost":
		return v.MonthlyCost
	case "Cost Trend":
		return v.CostTrend
	case "CPU %":
		return v.CPUPercent
	case "Memory %":
		return v.MemoryPercent
	case "Disk Read":
		return v.DiskIORead
	case "Disk Write":
		return v.DiskIOWrite
	case "Net In":
		return v.NetworkIn
	case "Net Out":
		return v.NetworkOut
	case "Recommendation":
		return v.Recommendation
	case "Est. Savings":
		return v.EstSavings
	default:
		return "-"
	}
}

// VMActions returns the standard actions available for VMs.
func VMActions() []Action {
	return []Action{
		{"Start", "Start the virtual machine", false},
		{"Stop", "Gracefully stop the virtual machine", false},
		{"Restart", "Reboot the virtual machine", false},
		{"Terminate", "Permanently delete the virtual machine", true},
		{"View Firewalls", "Open related firewalls or security groups for this VM", false},
		{"Describe", "Show full resource details", false},
		{"Cost", "Fetch on-demand cost report for this VM", false},
		{"FinOps", "Get Gemini AI cost & architecture recommendations", false},
		{"SSH", "Connect to the instance via SSH", false},
	}
}

// DescribeVM returns a human-readable summary of a VM.
func DescribeVM(v VM) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Name:           %s\n", v.Name)
	fmt.Fprintf(&b, "Instance ID:    %s\n", v.ID)
	fmt.Fprintf(&b, "Type:           %s\n", v.Type)
	fmt.Fprintf(&b, "State:          %s\n", v.State)
	fmt.Fprintf(&b, "Private IP:     %s\n", v.PrivateIP)
	fmt.Fprintf(&b, "Public IP:      %s\n", v.PublicIP)
	fmt.Fprintf(&b, "Zone:           %s\n", v.Zone)
	fmt.Fprintf(&b, "Resource Group: %s\n", v.ResourceGroup)
	fmt.Fprintf(&b, "Network:        %s\n", v.Network)
	fmt.Fprintf(&b, "Subnet:         %s\n", v.Subnet)
	fmt.Fprintf(&b, "SecurityGroups: %s\n", v.SecurityGroups)
	fmt.Fprintf(&b, "Labels:         %s\n", v.Labels)
	return b.String()
}
