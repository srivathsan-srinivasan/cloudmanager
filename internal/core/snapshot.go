package core

import (
	"fmt"
	"strings"
)

// Snapshot represents a point-in-time copy of a disk across any cloud provider.
// Implements the Resource interface.
type Snapshot struct {
	Name           string
	ID             string
	State          string
	SizeGB         int
	SourceDiskName string
	SourceDiskID   string
	CreatedAt      string
	Zone           string
	ResourceGroup  string
	Description    string
	Labels         string
}

// DefaultSnapshotColumns is the canonical list of all possible columns for Snapshots.
var DefaultSnapshotColumns = []string{
	"Name", "ID", "State", "Size (GB)", "Source Disk", "Created At",
}

// --- Resource interface implementation ---

func (s Snapshot) GetID() string   { return s.ID }
func (s Snapshot) GetName() string { return s.Name }
func (s Snapshot) GetKind() string { return "Snapshot" }

func (s Snapshot) GetField(col string) string {
	switch col {
	case "Name":
		return s.Name
	case "ID":
		return s.ID
	case "State":
		return s.State
	case "Size (GB)":
		return fmt.Sprintf("%d", s.SizeGB)
	case "Source Disk":
		if s.SourceDiskName != "" {
			return s.SourceDiskName
		}
		return s.SourceDiskID
	case "Created At":
		return s.CreatedAt
	case "Zone":
		return s.Zone
	case "Resource Group":
		return s.ResourceGroup
	case "Description":
		return s.Description
	case "Labels":
		return s.Labels
	default:
		return "-"
	}
}

// SnapshotActions returns the standard actions available for Snapshots.
func SnapshotActions() []Action {
	return []Action{
		{"Create Disk", "Create a new disk from this snapshot", false},
		{"Delete", "Permanently delete the snapshot", true},
		{"Describe", "Show full resource details", false},
		{"Open Console", "Open this snapshot in the provider console", false},
	}
}

// DescribeSnapshot returns a human-readable summary of a Snapshot.
func DescribeSnapshot(s Snapshot) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Name:           %s\n", s.Name)
	fmt.Fprintf(&b, "ID:             %s\n", s.ID)
	fmt.Fprintf(&b, "State:          %s\n", s.State)
	fmt.Fprintf(&b, "Size (GB):      %d\n", s.SizeGB)
	fmt.Fprintf(&b, "Source Disk:    %s (%s)\n", s.SourceDiskName, s.SourceDiskID)
	fmt.Fprintf(&b, "Created At:     %s\n", s.CreatedAt)
	fmt.Fprintf(&b, "Zone:           %s\n", s.Zone)
	fmt.Fprintf(&b, "Resource Group: %s\n", s.ResourceGroup)
	fmt.Fprintf(&b, "Description:    %s\n", s.Description)
	fmt.Fprintf(&b, "Labels:         %s\n", s.Labels)
	return b.String()
}
