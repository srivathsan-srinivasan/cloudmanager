package core

import (
	"testing"
)

func TestVMGetField(t *testing.T) {
	vm := VM{
		Name:          "web-server-01",
		ID:            "i-1234567890abcdef0",
		State:         "running",
		Type:          "t3.micro",
		Zone:          "us-east-1a",
		PrivateIP:     "10.0.0.5",
		PublicIP:      "203.0.113.10",
		ResourceGroup: "prod-web-rg",
		Network:       "vpc-1",
		Subnet:        "subnet-1",
		Labels:        "env=prod,role=web",
	}

	tests := []struct {
		column   string
		expected string
	}{
		{"Name", "web-server-01"},
		{"Instance ID", "i-1234567890abcdef0"},
		{"State", "running"},
		{"Type", "t3.micro"},
		{"Zone", "us-east-1a"},
		{"Private IP", "10.0.0.5"},
		{"Public IP", "203.0.113.10"},
		{"Resource Group", "prod-web-rg"},
		{"Network", "vpc-1"},
		{"Subnet", "subnet-1"},
		{"Labels", "env=prod,role=web"},
		{"UnknownColumn", "-"},
	}

	for _, tt := range tests {
		t.Run(tt.column, func(t *testing.T) {
			actual := vm.GetField(tt.column)
			if actual != tt.expected {
				t.Errorf("GetField(%q) = %q; want %q", tt.column, actual, tt.expected)
			}
		})
	}
}
