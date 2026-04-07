package core

import (
	"testing"
)

func TestDisk_GetField(t *testing.T) {
	d := Disk{
		Name:           "data-disk-1",
		ID:             "vol-0123456789abcdef0",
		State:          "in-use",
		SizeGB:         100,
		Type:           "gp3",
		Zone:           "us-west-2a",
		AttachedToVM:   "web-server-1",
		AttachedToVMID: "i-0987654321fedcba0",
		IOPS:           "3000",
		Throughput:     "125",
		Encrypted:      true,
		CreatedAt:      "2023-10-27T10:00:00Z",
		ResourceGroup:  "prod-rg",
		Labels:         "env=prod",
	}

	tests := []struct {
		col      string
		expected string
	}{
		{"Name", "data-disk-1"},
		{"ID", "vol-0123456789abcdef0"},
		{"State", "in-use"},
		{"Size (GB)", "100"},
		{"Type", "gp3"},
		{"Attached To", "web-server-1"},
		{"Zone", "us-west-2a"},
		{"Encrypted", "Yes"},
		{"IOPS", "3000"},
		{"Throughput", "125"},
		{"Created At", "2023-10-27T10:00:00Z"},
		{"Resource Group", "prod-rg"},
		{"Labels", "env=prod"},
		{"Unknown", "-"},
	}

	for _, tt := range tests {
		t.Run(tt.col, func(t *testing.T) {
			if got := d.GetField(tt.col); got != tt.expected {
				t.Errorf("Disk.GetField(%q) = %v, want %v", tt.col, got, tt.expected)
			}
		})
	}

	d2 := Disk{Encrypted: false}
	if got := d2.GetField("Encrypted"); got != "No" {
		t.Errorf("Disk.GetField(\"Encrypted\") = %v, want No", got)
	}
}

func TestDisk_Interfaces(t *testing.T) {
	d := Disk{Name: "test", ID: "123"}
	
	if d.GetID() != "123" {
		t.Errorf("Expected GetID() to return '123', got '%s'", d.GetID())
	}
	if d.GetName() != "test" {
		t.Errorf("Expected GetName() to return 'test', got '%s'", d.GetName())
	}
	if d.GetKind() != "Disk" {
		t.Errorf("Expected GetKind() to return 'Disk', got '%s'", d.GetKind())
	}
}
