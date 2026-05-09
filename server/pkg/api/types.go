package api

import (
	"example.com/fyp/pkg/types"
)

// GraphData represents the complete graph structure for visualization.
type GraphData struct {
	Nodes       []GraphNodeResponse `json:"nodes"`
	Edges       []GraphEdgeResponse `json:"edges"`
	Files       []FileNodeResponse  `json:"files"`
	Directories []DirectoryResponse `json:"directories"`
}

// GraphNodeResponse represents a node in the graph (real or virtual).
type GraphNodeResponse struct {
	ID             string                  `json:"id"`
	Label          string                  `json:"label"`
	Type           string                  `json:"type"` // "method", "class", "concept"
	FileID         string                  `json:"fileId,omitempty"`
	FilePath       string                  `json:"filePath,omitempty"`
	Line           int                     `json:"line,omitempty"`
	IsVirtual      bool                    `json:"isVirtual"`
	Materialized   bool                    `json:"materialized,omitempty"`
	ParentID       string                  `json:"parentId,omitempty"`
	VirtualClassID string                  `json:"virtualClassId,omitempty"`
	Annotation     *NodeAnnotationResponse `json:"annotation,omitempty"`
}

// GraphEdgeResponse represents a directed edge in the graph (source -> target).
type GraphEdgeResponse struct {
	ID         string                  `json:"id"`
	Source     string                  `json:"source"`
	Target     string                  `json:"target"`
	Count      int                     `json:"count"`
	IsManual   bool                    `json:"isManual,omitempty"`
	Annotation *EdgeAnnotationResponse `json:"annotation,omitempty"`
}

// NodeAnnotationResponse represents node annotation data.
type NodeAnnotationResponse struct {
	Description      string `json:"description,omitempty"`
	AIRemarks        string `json:"aiRemarks,omitempty"`
	CodeSnippet      string `json:"codeSnippet,omitempty"`
	CustomProperties string `json:"customProperties,omitempty"`
}

// EdgeAnnotationResponse represents edge annotation data.
type EdgeAnnotationResponse struct {
	Remarks string `json:"remarks,omitempty"`
}

// FileNodeResponse represents a file (real or virtual) in the tree.
type FileNodeResponse struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	Name      string `json:"name"`
	Directory string `json:"directory"`
	IsVirtual bool   `json:"isVirtual"`
}

// DirectoryResponse represents a directory (real or virtual).
type DirectoryResponse struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	Name      string `json:"name"`
	IsVirtual bool   `json:"isVirtual"`
}

// FileTreeResponse represents the complete file tree structure.
type FileTreeResponse struct {
	Directories []DirectoryTreeNode `json:"directories"`
	Files       []FileTreeNode      `json:"files"`
}

// DirectoryTreeNode represents a directory node in the tree.
type DirectoryTreeNode struct {
	ID         string              `json:"id"`
	Path       string              `json:"path"`
	Name       string              `json:"name"`
	IsVirtual  bool                `json:"isVirtual"`
	Children   []DirectoryTreeNode `json:"children,omitempty"`
	Files      []FileTreeNode      `json:"files,omitempty"`
	IsExpanded bool                `json:"isExpanded"`
}

// FileTreeNode represents a file node in the tree with methods.
type FileTreeNode struct {
	ID        string           `json:"id"`
	Path      string           `json:"path"`
	Name      string           `json:"name"`
	IsVirtual bool             `json:"isVirtual"`
	Methods   []MethodTreeNode `json:"methods,omitempty"`
}

// MethodTreeNode represents a method node in the file tree.
type MethodTreeNode struct {
	ID         string                  `json:"id"`
	Name       string                  `json:"name"`
	Type       string                  `json:"type"`
	Line       int                     `json:"line,omitempty"`
	IsVirtual  bool                    `json:"isVirtual"`
	Annotation *NodeAnnotationResponse `json:"annotation,omitempty"`
}

// CreateVirtualNodeRequest represents a request to create a virtual node.
type CreateVirtualNodeRequest struct {
	Label            string `json:"label"`
	Type             string `json:"type"`
	VirtualClassID   string `json:"virtualClassId,omitempty"`
	VirtualDirectory string `json:"virtualDirectory,omitempty"`
}

// UpdateVirtualNodeRequest represents a request to update a virtual node.
type UpdateVirtualNodeRequest struct {
	Label            string `json:"label"`
	Type             string `json:"type"`
	VirtualClassID   string `json:"virtualClassId,omitempty"`
	VirtualDirectory string `json:"virtualDirectory,omitempty"`
}

// CreateEdgeRequest represents a request to create an edge.
type CreateEdgeRequest struct {
	SourceNodeID string `json:"sourceNodeId"`
	TargetNodeID string `json:"targetNodeId"`
	Remarks      string `json:"remarks,omitempty"`
}

// UpdateAnnotationRequest represents a request to update node/edge annotation.
type UpdateAnnotationRequest struct {
	Description      string `json:"description,omitempty"`
	AIRemarks        string `json:"aiRemarks,omitempty"`
	CodeSnippet      string `json:"codeSnippet,omitempty"`
	CustomProperties string `json:"customProperties,omitempty"`
	Remarks          string `json:"remarks,omitempty"` // For edge annotations
}

// CreateVirtualDirectoryRequest represents a request to create a virtual directory.
type CreateVirtualDirectoryRequest struct {
	Path       string `json:"path"`
	Name       string `json:"name"`
	ParentPath string `json:"parentPath,omitempty"`
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// ============================================================================
// Snapshot Types (for agentic code generation)
// ============================================================================

// SnapshotNodeRequest represents a node in a snapshot creation request.
type SnapshotNodeRequest struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Type        string `json:"type"`
	FilePath    string `json:"filePath,omitempty"`
	Line        int    `json:"line,omitempty"`
	IsVirtual   bool   `json:"isVirtual"`
	ParentID    string `json:"parentId,omitempty"`
	Description string `json:"description,omitempty"`
	AIRemarks   string `json:"aiRemarks,omitempty"`
}

// SnapshotEdgeRequest represents an edge in a snapshot creation request.
type SnapshotEdgeRequest struct {
	ID       string `json:"id"`
	SourceID string `json:"sourceId"`
	TargetID string `json:"targetId"`
	Remarks  string `json:"remarks,omitempty"`
}

// CreateSnapshotRequest represents a request to create a snapshot.
type CreateSnapshotRequest struct {
	Name         string                `json:"name,omitempty"`
	VirtualNodes []SnapshotNodeRequest `json:"virtualNodes"`
	ContextNodes []SnapshotNodeRequest `json:"contextNodes"`
	Edges        []SnapshotEdgeRequest `json:"edges"`
}

// SnapshotResponse represents a snapshot in API responses.
type SnapshotResponse struct {
	ID               string `json:"id"`
	Name             string `json:"name,omitempty"`
	VirtualNodeCount int    `json:"virtualNodeCount"`
	ContextNodeCount int    `json:"contextNodeCount"`
	EdgeCount        int    `json:"edgeCount"`
	CreatedAt        int64  `json:"createdAt"`
}

// SnapshotDetailResponse represents a detailed snapshot response.
type SnapshotDetailResponse struct {
	ID           string                `json:"id"`
	Name         string                `json:"name,omitempty"`
	VirtualNodes []SnapshotNodeRequest `json:"virtualNodes"`
	ContextNodes []SnapshotNodeRequest `json:"contextNodes"`
	Edges        []SnapshotEdgeRequest `json:"edges"`
	CreatedAt    int64                 `json:"createdAt"`
}

// Helper functions to convert between types and API responses

// ToGraphNodeResponse converts a types.GraphNode to GraphNodeResponse.
func ToGraphNodeResponse(node types.GraphNode) GraphNodeResponse {
	response := GraphNodeResponse{
		ID:             node.ID,
		Label:          node.Label,
		Type:           node.Type,
		FileID:         node.FilePath, // Use file path as fileId for now
		FilePath:       node.FilePath,
		Line:           node.Line,
		IsVirtual:      node.IsVirtual,
		Materialized:   node.Materialized,
		ParentID:       node.ParentID,
		VirtualClassID: node.VirtualClassID,
	}

	if node.Annotation != nil {
		response.Annotation = &NodeAnnotationResponse{
			Description:      node.Annotation.Description,
			AIRemarks:        node.Annotation.AIRemarks,
			CodeSnippet:      node.Annotation.CodeSnippet,
			CustomProperties: node.Annotation.CustomProperties,
		}
	}

	return response
}

// ToGraphEdgeResponse converts method call data to GraphEdgeResponse.
func ToGraphEdgeResponse(call types.MethodCall, annotation *types.EdgeAnnotation) GraphEdgeResponse {
	edgeID := createDirectedEdgeID(call.CallerID, call.CalleeID)
	response := GraphEdgeResponse{
		ID:     edgeID,
		Source: call.CallerID,
		Target: call.CalleeID,
		Count:  1,
	}

	if annotation != nil {
		response.Annotation = &EdgeAnnotationResponse{
			Remarks: annotation.Remarks,
		}
		response.Count = annotation.CallCount
	}

	return response
}

// ToGraphEdgeResponseWithIDs converts method call data to GraphEdgeResponse using node IDs.
func ToGraphEdgeResponseWithIDs(call types.MethodCall, annotation *types.EdgeAnnotation, sourceID, targetID string) GraphEdgeResponse {
	edgeID := createDirectedEdgeID(sourceID, targetID)
	response := GraphEdgeResponse{
		ID:     edgeID,
		Source: sourceID,
		Target: targetID,
		Count:  1,
	}

	if annotation != nil {
		response.Annotation = &EdgeAnnotationResponse{
			Remarks: annotation.Remarks,
		}
		response.Count = annotation.CallCount
	}

	return response
}

