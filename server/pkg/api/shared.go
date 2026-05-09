package api

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"

	"example.com/fyp/pkg/types"
)

// hashString creates a stable hash-based ID from a string.
func hashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:8]) // First 8 bytes = 16 hex chars
}

// edgeSeparator is the delimiter used between source and target in edge IDs.
// Using "::" instead of "_" to avoid ambiguity with IDs that may contain underscores.
const edgeSeparator = "::"

// createDirectedEdgeID creates an edge ID preserving caller->callee direction.
// Returns source::target without sorting (source = caller, target = callee).
func createDirectedEdgeID(source, target string) string {
	return source + edgeSeparator + target
}

// buildFileAndDirectoryData extracts file and directory information from real (non-virtual) nodes.
// Virtual nodes are skipped since they no longer group under synthetic file paths.
func buildFileAndDirectoryData(nodes []types.GraphNode) ([]FileNodeResponse, []DirectoryResponse) {
	fileMap := make(map[string]*FileNodeResponse)
	dirMap := make(map[string]*DirectoryResponse)

	for _, node := range nodes {
		if node.IsVirtual || node.FilePath == "" {
			continue
		}

		// Create stable file ID from path hash
		fileID := hashString(node.FilePath)
		if _, exists := fileMap[fileID]; !exists {
			dirPath := filepath.Dir(node.FilePath)
			fileName := filepath.Base(node.FilePath)
			dirID := hashString(dirPath)

			fileMap[fileID] = &FileNodeResponse{
				ID:        fileID,
				Path:      node.FilePath,
				Name:      fileName,
				Directory: dirID,
				IsVirtual: false,
			}

			// Create directory if not exists
			if _, exists := dirMap[dirID]; !exists {
				dirMap[dirID] = &DirectoryResponse{
					ID:        dirID,
					Path:      dirPath,
					Name:      filepath.Base(dirPath),
					IsVirtual: false,
				}
			}
		}
	}

	// Convert maps to slices
	files := make([]FileNodeResponse, 0, len(fileMap))
	for _, f := range fileMap {
		files = append(files, *f)
	}

	dirs := make([]DirectoryResponse, 0, len(dirMap))
	for _, d := range dirMap {
		dirs = append(dirs, *d)
	}

	return files, dirs
}

// computeNodeParentIDs sets ParentID on each GraphNode based on the hierarchy rules:
// - Real methods/constructors/fields: parent = file hash (grouped by file)
// - Real classes/interfaces: no parent (standalone nodes on canvas)
// - Virtual methods with virtual_class_id: parent = virtual class node ID
// - Virtual classes with virtual_directory: parent = virtual directory hash
// - Other virtual nodes: no parent (top-level)
func computeNodeParentIDs(nodes []types.GraphNode) []types.GraphNode {
	for i := range nodes {
		n := &nodes[i]
		if !n.IsVirtual {
			if n.Type != "class" && n.Type != "interface" {
				if n.FilePath != "" {
					n.ParentID = hashString(n.FilePath)
				}
			}
			// Real classes and interfaces: ParentID stays "" (standalone nodes)
		} else {
			if n.VirtualClassID != "" {
				// Virtual method: parent = virtual class node
				n.ParentID = n.VirtualClassID
			} else if n.Type == "class" && n.VirtualDirectory != "" {
				// Virtual class: parent = virtual directory
				n.ParentID = hashString(n.VirtualDirectory)
			}
			// else: top-level virtual node (no parent)
		}
	}
	return nodes
}

// SnapshotPromptResponse represents the response for snapshot prompt endpoints.
type SnapshotPromptResponse struct {
	Prompt   string `json:"prompt"`
	IsCustom bool   `json:"isCustom"`
}
