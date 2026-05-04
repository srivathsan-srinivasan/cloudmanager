package core

import (
	"testing"
)

func TestSnapshot_GetField(t *testing.T) {
	s := Snapshot{
		Name:           "backup-2023-10-27",
		ID:             "snap-0123456789abcdef0",
		State:          "completed",
		SizeGB:         100,
		SourceDiskName: "data-disk-1",
		SourceDiskID:   "vol-0123456789abcdef0",
		CreatedAt:      "2023-10-27T10:00:00Z",
		Zone:           "us-west-2a",
		ResourceGroup:  "prod-rg",
		Description:    "Daily backup",
		Labels:         "env=prod",
	}

	tests := []struct {
		col      string
		expected string
	}{
		{"Name", "backup-2023-10-27"},
		{"ID", "snap-0123456789abcdef0"},
		{"State", "completed"},
		{"Size (GB)", "100"},
		{"Source Disk", "data-disk-1"},
		{"Created At", "2023-10-27T10:00:00Z"},
		{"Zone", "us-west-2a"},
		{"Resource Group", "prod-rg"},
		{"Description", "Daily backup"},
		{"Labels", "env=prod"},
		{"Unknown", "-"},
	}

	for _, tt := range tests {
		t.Run(tt.col, func(t *testing.T) {
			if got := s.GetField(tt.col); got != tt.expected {
				t.Errorf("Snapshot.GetField(%q) = %v, want %v", tt.col, got, tt.expected)
			}
		})
	}

	s2 := Snapshot{SourceDiskID: "vol-123"}
	if got := s2.GetField("Source Disk"); got != "vol-123" {
		t.Errorf("Snapshot.GetField(\"Source Disk\") = %v, want vol-123", got)
	}
}

func TestSnapshot_Interfaces(t *testing.T) {
	s := Snapshot{Name: "test", ID: "123"}

	if s.GetID() != "123" {
		t.Errorf("Expected GetID() to return '123', got '%s'", s.GetID())
	}
	if s.GetName() != "test" {
		t.Errorf("Expected GetName() to return 'test', got '%s'", s.GetName())
	}
	if s.GetKind() != "Snapshot" {
		t.Errorf("Expected GetKind() to return 'Snapshot', got '%s'", s.GetKind())
	}
}
