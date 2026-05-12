package core

import (
	"fmt"
	"strings"
)

// Disk represents a storage volume across any cloud provider.
// Implements the Resource interface.
type Disk struct {
	Name           string
	ID             string
	State          string
	SizeGB         int
	Type           string
	Zone           string
	AttachedToVM   string
	AttachedToVMID string
	IOPS           string
	Throughput     string
	Encrypted      bool
	CreatedAt      string
	ResourceGroup  string
	Labels         string
}

// DefaultDiskColumns is the canonical list of all possible columns for Disks.
var DefaultDiskColumns = []string{
	"Name", "ID", "State", "Size (GB)", "Type", "Attached To", "Zone", "Encrypted",
}

// --- Resource interface implementation ---

func (d Disk) GetID() string   { return d.ID }
func (d Disk) GetName() string { return d.Name }
func (d Disk) GetKind() string { return "Disk" }

func (d Disk) GetField(col string) string {
	switch col {
	case "Name":
		return d.Name
	case "ID":
		return d.ID
	case "State":
		return d.State
	case "Size (GB)":
		return fmt.Sprintf("%d", d.SizeGB)
	case "Type":
		return d.Type
	case "Attached To":
		return d.AttachedToVM
	case "Zone":
		return d.Zone
	case "Encrypted":
		if d.Encrypted {
			return "Yes"
		}
		return "No"
	case "IOPS":
		return d.IOPS
	case "Throughput":
		return d.Throughput
	case "Created At":
		return d.CreatedAt
	case "Resource Group":
		return d.ResourceGroup
	case "Labels":
		return d.Labels
	default:
		return "-"
	}
}

// DiskActions returns the standard actions available for Disks.
func DiskActions() []Action {
	return []Action{
		{"Detach", "Detach the disk from its VM", false},
		{"Delete", "Permanently delete the disk", true},
		{"Resize", "Increase the size of the disk", false},
		{"Create Snapshot", "Create a snapshot of the disk", false},
		{"Describe", "Show full resource details", false},
		{"Open Console", "Open this disk in the provider console", false},
	}
}

// DescribeDisk returns a human-readable summary of a Disk.
func DescribeDisk(d Disk) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Name:           %s\n", d.Name)
	fmt.Fprintf(&b, "ID:             %s\n", d.ID)
	fmt.Fprintf(&b, "State:          %s\n", d.State)
	fmt.Fprintf(&b, "Size (GB):      %d\n", d.SizeGB)
	fmt.Fprintf(&b, "Type:           %s\n", d.Type)
	fmt.Fprintf(&b, "Zone:           %s\n", d.Zone)
	fmt.Fprintf(&b, "Attached To:    %s (%s)\n", d.AttachedToVM, d.AttachedToVMID)
	fmt.Fprintf(&b, "IOPS:           %s\n", d.IOPS)
	fmt.Fprintf(&b, "Throughput:     %s\n", d.Throughput)
	fmt.Fprintf(&b, "Encrypted:      %v\n", d.Encrypted)
	fmt.Fprintf(&b, "Created At:     %s\n", d.CreatedAt)
	fmt.Fprintf(&b, "Resource Group: %s\n", d.ResourceGroup)
	fmt.Fprintf(&b, "Labels:         %s\n", d.Labels)
	return b.String()
}
