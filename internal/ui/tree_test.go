package ui

import (
	"testing"

	"github.com/srivathsan-srinivasan/cloudmanager/internal/core"
)

func TestBuildContextTree(t *testing.T) {
	contexts := []core.CloudContext{
		{Provider: "AWS", AccountID: "9431", AccountName: "main", Region: "ap-south-1", CredentialProfile: "main-aps1"},
		{Provider: "AWS", AccountID: "9431", AccountName: "main", Region: "us-east-2", CredentialProfile: "main-use2"},
		{Provider: "AWS", AccountID: "9824", AccountName: "c2shell", Region: "us-east-1", CredentialProfile: "9824-use1"},
		{Provider: "GCP", AccountName: "my-gcp-project", Region: "global"},
	}

	rootNodes := BuildContextTree(contexts)

	if len(rootNodes) != 2 {
		t.Fatalf("Expected 2 providers (AWS, GCP), got %d", len(rootNodes))
	}

	var awsNode *TreeNode
	for _, n := range rootNodes {
		if n.Label == "AWS" {
			awsNode = n
			break
		}
	}

	if awsNode == nil {
		t.Fatalf("AWS node not found")
	}

	if len(awsNode.Children) != 2 {
		t.Fatalf("Expected 2 AWS accounts, got %d", len(awsNode.Children))
	}

	var mainNode *TreeNode
	for _, n := range awsNode.Children {
		if n.Label == "9431 (main)" {
			mainNode = n
			break
		}
	}

	if mainNode == nil {
		t.Fatalf("9431 (main) account node not found")
	}

	if len(mainNode.Children) != 2 {
		t.Fatalf("Expected 2 regions for 9431 (main), got %d", len(mainNode.Children))
	}
}

func TestBuildFlatList(t *testing.T) {
	rootNodes := []*TreeNode{
		{
			Label:    "AWS",
			Expanded: true,
			Children: []*TreeNode{
				{
					Label:    "9431 (main)",
					Expanded: false,
					Children: []*TreeNode{
						{Label: "us-east-1", IsLeaf: true},
					},
				},
			},
		},
	}

	flat := BuildFlatList(rootNodes)

	if len(flat) != 2 {
		t.Fatalf("Expected 2 nodes in flat list (AWS, account), got %d", len(flat))
	}

	if flat[0].(*TreeNode).Label != "AWS" {
		t.Errorf("Expected first node to be AWS, got %s", flat[0].(*TreeNode).Label)
	}

	if flat[1].(*TreeNode).Label != "9431 (main)" {
		t.Errorf("Expected second node to be 9431 (main), got %s", flat[1].(*TreeNode).Label)
	}
}

func TestBuildContextTreeCollapsesManualHosts(t *testing.T) {
	rootNodes := BuildContextTree([]core.CloudContext{
		{Provider: "Manual", AccountID: "manual-hosts", AccountName: "Manual Hosts", Region: "global"},
	})

	if len(rootNodes) != 1 {
		t.Fatalf("expected one provider, got %d", len(rootNodes))
	}
	if rootNodes[0].Label != "Manual" {
		t.Fatalf("expected Manual provider node, got %q", rootNodes[0].Label)
	}
	if len(rootNodes[0].Children) != 1 {
		t.Fatalf("expected one direct child, got %+v", rootNodes[0].Children)
	}
	child := rootNodes[0].Children[0]
	if !child.IsLeaf || child.Level != 1 || child.Label != "Hosts" {
		t.Fatalf("expected direct Hosts leaf, got %+v", child)
	}
}
