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

func TestVMKubernetesNodeDetection(t *testing.T) {
	cases := []struct {
		name string
		vm   VM
		want bool
	}{
		{
			name: "eks tags",
			vm:   VM{Name: "ip-10-0-1-5", Labels: "eks:cluster-name=prod, eks:nodegroup-name=spot"},
			want: true,
		},
		{
			name: "gke labels",
			vm:   VM{Name: "gke-prod-pool-abc", Labels: "goog-gke-node=true, cloud.google.com/gke-nodepool=pool-a"},
			want: true,
		},
		{
			name: "aks resource group",
			vm:   VM{Name: "aks-nodepool1-123456-vmss000001", ResourceGroup: "MC_prod_rg_cluster_eastus", Labels: "aks-managed-cluster-name=prod"},
			want: true,
		},
		{
			name: "ordinary node word",
			vm:   VM{Name: "build-node-01", Labels: "role=worker"},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.vm.IsKubernetesNode(); got != tc.want {
				t.Fatalf("IsKubernetesNode()=%t want %t", got, tc.want)
			}
		})
	}
}

func TestVMKubernetesNodeLifecycle(t *testing.T) {
	vm := VM{Name: "gke-prod-pool-abc", Labels: "goog-gke-node=true, preemptible=true"}
	if got := vm.KubernetesNodeLifecycle(); got != "spot" {
		t.Fatalf("expected spot lifecycle, got %q", got)
	}
}

func TestVMKubernetesClusterAndNodePoolNames(t *testing.T) {
	cases := []struct {
		name        string
		vm          VM
		wantCluster string
		wantPool    string
	}{
		{
			name:        "eks labels",
			vm:          VM{Labels: "eks:cluster-name=prod, eks:nodegroup-name=spot-a"},
			wantCluster: "prod",
			wantPool:    "spot-a",
		},
		{
			name:        "gke labels",
			vm:          VM{Name: "gke-prod-pool-a-abcd", Labels: "goog-k8s-cluster-name=prod, cloud.google.com/gke-nodepool=pool-a"},
			wantCluster: "prod",
			wantPool:    "pool-a",
		},
		{
			name:        "aws cluster tag key",
			vm:          VM{Labels: "kubernetes.io/cluster/prod=owned, karpenter.sh/nodepool=default"},
			wantCluster: "prod",
			wantPool:    "default",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.vm.KubernetesClusterName(); got != tc.wantCluster {
				t.Fatalf("cluster=%q want %q", got, tc.wantCluster)
			}
			if got := tc.vm.KubernetesNodePoolName(); got != tc.wantPool {
				t.Fatalf("pool=%q want %q", got, tc.wantPool)
			}
		})
	}
}
