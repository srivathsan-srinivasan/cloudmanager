package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"

	"cloudmanager/internal/core"
	"cloudmanager/internal/providers"
)

// TreeNode represents a collapsible node in the context sidebar.
type TreeNode struct {
	ID       string
	Label    string
	Level    int
	Expanded bool
	IsLeaf   bool
	Context  core.CloudContext
	Children []*TreeNode
}

func (n *TreeNode) Title() string {
	indent := strings.Repeat("  ", n.Level)
	if n.IsLeaf {
		return fmt.Sprintf("%s\u2022 %s", indent, n.Label)
	}
	if n.Expanded {
		return fmt.Sprintf("%s\u25BC %s", indent, n.Label)
	}
	return fmt.Sprintf("%s\u25B6 %s", indent, n.Label)
}

func (n *TreeNode) Description() string {
	if n.IsLeaf {
		if meta, ok := providers.MetadataFor(n.Context.Provider); ok && n.Context.Region != "" && n.Context.Region != "global" && meta.GlobalLeafLabel != n.Label {
			return fmt.Sprintf("%s  Region: %s", strings.Repeat("  ", n.Level), n.Context.Region)
		}
		if n.Context.Provider == "AWS" {
			return fmt.Sprintf("%s  Region: %s", strings.Repeat("  ", n.Level), n.Context.Region)
		}
		return ""
	}
	count := 0
	for _, c := range n.Children {
		if c.IsLeaf {
			count++
		} else {
			count += len(c.Children)
		}
	}
	return fmt.Sprintf("%s  (%d items)", strings.Repeat("  ", n.Level), count)
}

func (n *TreeNode) FilterValue() string { return n.Label }

// BuildFlatList traverses the tree and returns the visible nodes as list.Items.
func BuildFlatList(nodes []*TreeNode) []list.Item {
	var items []list.Item
	for _, n := range nodes {
		items = append(items, n)
		if n.Expanded && len(n.Children) > 0 {
			items = append(items, BuildFlatList(n.Children)...)
		}
	}
	return items
}

// BuildContextTree groups CloudContexts into a Provider → Account → Region hierarchy.
func BuildContextTree(contexts []core.CloudContext) []*TreeNode {
	providerNodes := make(map[string]*TreeNode)
	for _, ctx := range contexts {
		pNode, ok := providerNodes[ctx.Provider]
		if !ok {
			pNode = &TreeNode{ID: ctx.Provider, Label: ctx.Provider, Level: 0, Expanded: true}
			providerNodes[ctx.Provider] = pNode
		}

		groupKey := ctx.AccountID
		if groupKey == "" {
			groupKey = ctx.AccountName
		}
		groupName := ctx.DisplayName()

		accountID := ctx.Provider + "-" + groupKey
		var aNode *TreeNode
		for _, child := range pNode.Children {
			if child.ID == accountID {
				aNode = child
				break
			}
		}
		if aNode == nil {
			aNode = &TreeNode{ID: accountID, Label: groupName, Level: 1, Expanded: false}
			pNode.Children = append(pNode.Children, aNode)
		}

		regionID := accountID + "-" + ctx.Region
		leafLabel := ctx.Region
		if meta, ok := providers.MetadataFor(ctx.Provider); ok && ctx.Region == "global" && meta.GlobalLeafLabel != "" {
			leafLabel = meta.GlobalLeafLabel
		}

		aNode.Children = append(aNode.Children, &TreeNode{
			ID: regionID, Label: leafLabel, Level: 2, IsLeaf: true, Context: ctx,
		})
	}

	for _, provider := range providerNodes {
		sort.Slice(provider.Children, func(i, j int) bool {
			return provider.Children[i].Label < provider.Children[j].Label
		})
		for _, account := range provider.Children {
			sort.Slice(account.Children, func(i, j int) bool {
				return account.Children[i].Label < account.Children[j].Label
			})
		}
	}

	var rootNodes []*TreeNode
	registeredNames := registeredProviderNamesMap()
	for _, registered := range providers.RegisteredProviders() {
		if node, ok := providerNodes[registered.Metadata.DisplayName]; ok {
			rootNodes = append(rootNodes, node)
		}
	}
	for providerName, node := range providerNodes {
		if _, ok := registeredNames[providerName]; !ok {
			rootNodes = append(rootNodes, node)
		}
	}
	return rootNodes
}

func registeredProviderNamesMap() map[string]struct{} {
	names := make(map[string]struct{})
	for _, registered := range providers.RegisteredProviders() {
		names[registered.Metadata.DisplayName] = struct{}{}
	}
	return names
}
