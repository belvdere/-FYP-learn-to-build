package types

import (
	"fmt"
	"sort"
	"strings"
)

// FormatSnapshotAsMarkdown formats a snapshot as markdown for the MCP tool.
// This is used by kg.getSnapshot to return context to the Copilot agent.
func FormatSnapshotAsMarkdown(snapshot *Snapshot) string {
	var sb strings.Builder

	// Header with summary
	sb.WriteString(fmt.Sprintf("# Graph Snapshot: %s\n\n", snapshot.ID))
	if snapshot.Name != "" {
		sb.WriteString(fmt.Sprintf("**Name:** %s\n\n", snapshot.Name))
	}

	// Executive summary
	sb.WriteString("**Summary:** ")
	parts := []string{}
	if len(snapshot.VirtualNodes) > 0 {
		parts = append(parts, fmt.Sprintf("%d virtual node(s)", len(snapshot.VirtualNodes)))
	}
	if len(snapshot.ContextNodes) > 0 {
		parts = append(parts, fmt.Sprintf("%d context node(s)", len(snapshot.ContextNodes)))
	}
	if len(snapshot.Edges) > 0 {
		parts = append(parts, fmt.Sprintf("%d relationship(s)", len(snapshot.Edges)))
	}
	if len(parts) > 0 {
		sb.WriteString(strings.Join(parts, ", ") + ".")
	} else {
		sb.WriteString("Empty snapshot.")
	}
	sb.WriteString("\n\n")
	sb.WriteString("*Use this context to inform your response. Proceed with whatever the user asked.*\n\n---\n\n")

	// Build lookup maps
	virtualIDs := make(map[string]bool)
	for _, n := range snapshot.VirtualNodes {
		virtualIDs[n.ID] = true
	}
	labelMap := make(map[string]string)
	for _, n := range snapshot.VirtualNodes {
		labelMap[n.ID] = n.Label
	}
	for _, n := range snapshot.ContextNodes {
		labelMap[n.ID] = n.Label
	}

	// Virtual Design — rendered as a hierarchy (dir → class → method)
	sb.WriteString("## Virtual Design\n\n")
	if len(snapshot.VirtualNodes) == 0 {
		sb.WriteString("No virtual nodes.\n\n")
	} else {
		sb.WriteString("Methods/classes to implement or refine:\n\n")
		formatVirtualHierarchy(&sb, snapshot.VirtualNodes, snapshot.Edges, labelMap, virtualIDs)
	}

	// Context Nodes (group by file)
	sb.WriteString("## Context Nodes (Existing Code)\n\n")
	if len(snapshot.ContextNodes) == 0 {
		sb.WriteString("No context nodes.\n\n")
	} else {
		sb.WriteString("Reference these existing implementations for patterns and integration:\n\n")
		fileToNodes := make(map[string][]SnapshotNode)
		for _, node := range snapshot.ContextNodes {
			key := node.FilePath
			if key == "" {
				key = "(no file)"
			}
			fileToNodes[key] = append(fileToNodes[key], node)
		}
		var files []string
		for f := range fileToNodes {
			files = append(files, f)
		}
		sort.Strings(files)
		for _, filePath := range files {
			nodes := fileToNodes[filePath]
			sb.WriteString(fmt.Sprintf("**File:** `%s`\n", filePath))
			for _, node := range nodes {
				sb.WriteString(fmt.Sprintf("- **%s**", node.Label))
				if node.Line > 0 {
					sb.WriteString(fmt.Sprintf(" (line %d)", node.Line))
				}
				sb.WriteString(fmt.Sprintf(" — %s", node.Type))
				if node.Description != "" {
					sb.WriteString(fmt.Sprintf(": %s", node.Description))
				}
				sb.WriteString("\n")
			}
			sb.WriteString("\n")
		}
	}

	// Relationships
	sb.WriteString("## Relationships\n\n")
	if len(snapshot.Edges) == 0 {
		sb.WriteString("No relationships defined.\n\n")
	} else {
		sb.WriteString("How components connect:\n\n")
		for _, edge := range snapshot.Edges {
			sourceLabel := labelMap[edge.SourceID]
			if sourceLabel == "" {
				sourceLabel = edge.SourceID
			}
			targetLabel := labelMap[edge.TargetID]
			if targetLabel == "" {
				targetLabel = edge.TargetID
			}
			sourceVirtual := virtualIDs[edge.SourceID]
			targetVirtual := virtualIDs[edge.TargetID]
			badge := ""
			if sourceVirtual && targetVirtual {
				badge = " (virtual → virtual)"
			} else if sourceVirtual {
				badge = " (virtual → context)"
			} else if targetVirtual {
				badge = " (context → virtual)"
			}
			sb.WriteString(fmt.Sprintf("- `%s` → `%s`%s", sourceLabel, targetLabel, badge))
			if edge.Remarks != "" {
				sb.WriteString(fmt.Sprintf(" — %s", edge.Remarks))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Usage note
	sb.WriteString("---\n\n")
	sb.WriteString("**Usage:** Use the virtual nodes, context nodes, and relationships above to inform your understanding. Respond to the user's actual request (e.g., generate, explain, refactor, add tests).\n")

	return sb.String()
}

// formatVirtualHierarchy renders virtual nodes in a directory → class → method tree.
func formatVirtualHierarchy(sb *strings.Builder, nodes []SnapshotNode, edges []SnapshotEdge, labelMap map[string]string, virtualIDs map[string]bool) {
	// Build id → node map
	nodeMap := make(map[string]*SnapshotNode)
	for i := range nodes {
		nodeMap[nodes[i].ID] = &nodes[i]
	}

	// Group by parentId
	// children[parentID] = []node
	children := make(map[string][]SnapshotNode)
	var roots []SnapshotNode

	for _, node := range nodes {
		if node.ParentID == "" {
			roots = append(roots, node)
		} else {
			children[node.ParentID] = append(children[node.ParentID], node)
		}
	}

	// Sort roots and children by label for stable output
	sortNodes(roots)
	for k := range children {
		sortNodes(children[k])
	}

	if len(roots) == 0 {
		// Fallback: all nodes flat if no hierarchy
		for i := range nodes {
			renderVirtualNode(sb, &nodes[i], i+1, edges, labelMap, virtualIDs)
		}
		return
	}

	for _, root := range roots {
		renderVirtualNodeTree(sb, &root, nodeMap, children, edges, labelMap, virtualIDs, 0)
	}
}

// renderVirtualNodeTree recursively renders a node and its children.
func renderVirtualNodeTree(sb *strings.Builder, node *SnapshotNode, nodeMap map[string]*SnapshotNode, children map[string][]SnapshotNode, edges []SnapshotEdge, labelMap map[string]string, virtualIDs map[string]bool, depth int) {
	switch depth {
	case 0: // virtual directory level
		sb.WriteString(fmt.Sprintf("### 📁 %s *(virtual directory)*\n\n", node.Label))
		if node.Description != "" {
			sb.WriteString(fmt.Sprintf("- Description: %s\n\n", node.Description))
		}
		for _, child := range children[node.ID] {
			childCopy := child
			renderVirtualNodeTree(sb, &childCopy, nodeMap, children, edges, labelMap, virtualIDs, depth+1)
		}
	case 1: // virtual class level
		sb.WriteString(fmt.Sprintf("#### 📦 %s *(virtual class)*\n", node.Label))
		if node.Description != "" {
			sb.WriteString(fmt.Sprintf("- Description: %s\n", node.Description))
		}
		if node.AIRemarks != "" {
			sb.WriteString(fmt.Sprintf("- AI Remarks: %s\n", node.AIRemarks))
		}
		related := getRelatedForNode(node.ID, edges, labelMap, virtualIDs)
		if len(related) > 0 {
			sb.WriteString(fmt.Sprintf("- Related: %s\n", strings.Join(related, "; ")))
		}
		sb.WriteString("\n")
		for _, child := range children[node.ID] {
			childCopy := child
			renderVirtualNodeTree(sb, &childCopy, nodeMap, children, edges, labelMap, virtualIDs, depth+1)
		}
	default: // method/leaf level
		sb.WriteString(fmt.Sprintf("##### %s *(virtual %s)*\n", node.Label, node.Type))
		sb.WriteString(fmt.Sprintf("- Type: %s\n", node.Type))
		if node.FilePath != "" {
			sb.WriteString(fmt.Sprintf("- Target file: `%s`\n", node.FilePath))
		}
		if node.Description != "" {
			sb.WriteString(fmt.Sprintf("- Description: %s\n", node.Description))
		}
		if node.AIRemarks != "" {
			sb.WriteString(fmt.Sprintf("- AI Remarks: %s\n", node.AIRemarks))
		}
		related := getRelatedForNode(node.ID, edges, labelMap, virtualIDs)
		if len(related) > 0 {
			sb.WriteString(fmt.Sprintf("- Related: %s\n", strings.Join(related, "; ")))
		}
		sb.WriteString("\n")
		// Render any children (shouldn't normally happen for methods)
		for _, child := range children[node.ID] {
			childCopy := child
			renderVirtualNodeTree(sb, &childCopy, nodeMap, children, edges, labelMap, virtualIDs, depth+1)
		}
	}
}

// renderVirtualNode renders a single virtual node at a flat (non-hierarchy) position.
func renderVirtualNode(sb *strings.Builder, node *SnapshotNode, index int, edges []SnapshotEdge, labelMap map[string]string, virtualIDs map[string]bool) {
	sb.WriteString(fmt.Sprintf("### %d. %s *(virtual)*\n", index, node.Label))
	sb.WriteString(fmt.Sprintf("- **Type:** %s\n", node.Type))
	if node.FilePath != "" {
		sb.WriteString(fmt.Sprintf("- **Target file:** `%s`\n", node.FilePath))
	}
	if node.Description != "" {
		sb.WriteString(fmt.Sprintf("- **Description:** %s\n", node.Description))
	}
	if node.AIRemarks != "" {
		sb.WriteString(fmt.Sprintf("- **AI Remarks:** %s\n", node.AIRemarks))
	}
	related := getRelatedForNode(node.ID, edges, labelMap, virtualIDs)
	if len(related) > 0 {
		sb.WriteString(fmt.Sprintf("- **Related:** %s\n", strings.Join(related, "; ")))
	}
	sb.WriteString("\n")
}

func sortNodes(nodes []SnapshotNode) {
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Label < nodes[j].Label
	})
}

// getRelatedForNode returns human-readable "X → Y" strings for edges involving the given node.
func getRelatedForNode(nodeID string, edges []SnapshotEdge, labelMap map[string]string, virtualIDs map[string]bool) []string {
	var result []string
	for _, e := range edges {
		if e.SourceID != nodeID && e.TargetID != nodeID {
			continue
		}
		src := labelMap[e.SourceID]
		tgt := labelMap[e.TargetID]
		if src == "" {
			src = e.SourceID
		}
		if tgt == "" {
			tgt = e.TargetID
		}
		if e.SourceID == nodeID {
			v := tgt
			if virtualIDs[e.TargetID] {
				v = tgt + " (virtual)"
			}
			result = append(result, fmt.Sprintf("→ %s", v))
		} else {
			v := src
			if virtualIDs[e.SourceID] {
				v = src + " (virtual)"
			}
			result = append(result, fmt.Sprintf("← %s", v))
		}
	}
	return result
}
