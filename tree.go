package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
)

type treeNode struct {
	id       string
	label    string
	level    int
	expanded bool
	isLeaf   bool
	context  contextItem
	children []*treeNode
}

func (n *treeNode) Title() string {
	indent := strings.Repeat("  ", n.level)
	if n.isLeaf {
		return fmt.Sprintf("%s\u2022 %s", indent, n.label)
	}
	if n.expanded {
		return fmt.Sprintf("%s\u25BC %s", indent, n.label)
	}
	return fmt.Sprintf("%s\u25B6 %s", indent, n.label)
}

func (n *treeNode) Description() string {
	if n.isLeaf {
		if n.context.provider == "AWS" {
			return fmt.Sprintf("%s  Region: %s", strings.Repeat("  ", n.level), n.context.region)
		}
		return ""
	}
	count := 0
	for _, c := range n.children {
		if c.isLeaf {
			count++
		} else {
			count += len(c.children) // roughly
		}
	}
	return fmt.Sprintf("%s  (%d items)", strings.Repeat("  ", n.level), count)
}

func (n *treeNode) FilterValue() string {
	return n.label
}

// buildFlatList traverses the tree and returns the visible nodes as list.Item
func buildFlatList(nodes []*treeNode) []list.Item {
	var items []list.Item
	for _, n := range nodes {
		items = append(items, n)
		if n.expanded && len(n.children) > 0 {
			items = append(items, buildFlatList(n.children)...)
		}
	}
	return items
}

func buildContextTree(contexts []contextItem) []*treeNode {
	providers := make(map[string]*treeNode)

	for _, ctx := range contexts {
		// Level 0: Provider
		pNode, ok := providers[ctx.provider]
		if !ok {
			pNode = &treeNode{
				id:       ctx.provider,
				label:    ctx.provider,
				level:    0,
				expanded: true, // Expand root by default
				isLeaf:   false,
			}
			providers[ctx.provider] = pNode
		}

		// Level 1: Account / Project / Subscription
		accountID := ctx.provider + "-" + ctx.accountName
		var aNode *treeNode
		for _, child := range pNode.children {
			if child.id == accountID {
				aNode = child
				break
			}
		}
		if aNode == nil {
			aNode = &treeNode{
				id:       accountID,
				label:    ctx.accountName,
				level:    1,
				expanded: false, // Collapse accounts by default to save space
				isLeaf:   false,
			}
			pNode.children = append(pNode.children, aNode)
		}

		// Level 2: Region (Leaf)
		regionID := accountID + "-" + ctx.region
		leafLabel := ctx.region
		if ctx.provider == "GCP" || ctx.provider == "Azure" {
			leafLabel = "All Resources"
		}

		rNode := &treeNode{
			id:      regionID,
			label:   leafLabel,
			level:   2,
			isLeaf:  true,
			context: ctx,
		}
		aNode.children = append(aNode.children, rNode)
	}

	var rootNodes []*treeNode
	// Define a fixed order for providers if they exist
	for _, p := range []string{"AWS", "GCP", "Azure"} {
		if node, ok := providers[p]; ok {
			rootNodes = append(rootNodes, node)
		}
	}
	return rootNodes
}
