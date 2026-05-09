package api

import (
	"fmt"

	"example.com/fyp/pkg/types"
	"github.com/google/uuid"
)

// ============================================================================
// Snapshot handlers
// ============================================================================

func (s *StdioServer) handleListSnapshots(req *StdioRequest) {
	snapshots, err := s.store.ListSnapshots()
	if err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to list snapshots", err.Error())
		return
	}

	response := make([]SnapshotResponse, 0, len(snapshots))
	for _, snap := range snapshots {
		response = append(response, SnapshotResponse{
			ID:               snap.ID,
			Name:             snap.Name,
			VirtualNodeCount: len(snap.VirtualNodes),
			ContextNodeCount: len(snap.ContextNodes),
			EdgeCount:        len(snap.Edges),
			CreatedAt:        snap.CreatedAt,
		})
	}

	s.sendResponse(req.ID, response)
}

// handleCreateSnapshot creates a snapshot for AI context (kg.getSnapshot).
// Virtual nodes = to-implement; context nodes = reference; edges = relationships.
func (s *StdioServer) handleCreateSnapshot(req *StdioRequest) {
	name := s.getStringParam(req, "name")

	// Parse virtual nodes (planned features, not yet in codebase)
	var virtualNodes []types.SnapshotNode
	if vn, ok := req.Params["virtualNodes"].([]interface{}); ok {
		for _, v := range vn {
			if m, ok := v.(map[string]interface{}); ok {
				virtualNodes = append(virtualNodes, parseSnapshotNode(m))
			}
		}
	}

	// Parse context nodes (existing code to reference for patterns)
	var contextNodes []types.SnapshotNode
	if cn, ok := req.Params["contextNodes"].([]interface{}); ok {
		for _, v := range cn {
			if m, ok := v.(map[string]interface{}); ok {
				contextNodes = append(contextNodes, parseSnapshotNode(m))
			}
		}
	}

	// Parse edges
	var edges []types.SnapshotEdge
	if e, ok := req.Params["edges"].([]interface{}); ok {
		for _, v := range e {
			if m, ok := v.(map[string]interface{}); ok {
				edges = append(edges, parseSnapshotEdge(m))
			}
		}
	}

	// Generate stable snapshot ID (e.g. snap_abc123def456)
	snapshotID := fmt.Sprintf("snap_%s", uuid.New().String()[:12])

	snapshot := &types.Snapshot{
		ID:           snapshotID,
		Name:         name,
		VirtualNodes: virtualNodes,
		ContextNodes: contextNodes,
		Edges:        edges,
	}

	if err := s.store.CreateSnapshot(snapshot); err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to create snapshot", err.Error())
		return
	}

	s.sendResponse(req.ID, SnapshotResponse{
		ID:               snapshot.ID,
		Name:             snapshot.Name,
		VirtualNodeCount: len(snapshot.VirtualNodes),
		ContextNodeCount: len(snapshot.ContextNodes),
		EdgeCount:        len(snapshot.Edges),
		CreatedAt:        snapshot.CreatedAt,
	})
}

func (s *StdioServer) handleGetSnapshot(req *StdioRequest) {
	id := s.getStringParam(req, "id")
	if id == "" {
		s.sendError(req.ID, ErrInvalidParams, "Missing id parameter", nil)
		return
	}

	snapshot, err := s.store.GetSnapshot(id)
	if err != nil {
		s.sendError(req.ID, ErrInternalError, "Snapshot not found", err.Error())
		return
	}

	virtualNodes := make([]SnapshotNodeRequest, 0, len(snapshot.VirtualNodes))
	for _, n := range snapshot.VirtualNodes {
		virtualNodes = append(virtualNodes, SnapshotNodeRequest{
			ID:          n.ID,
			Label:       n.Label,
			Type:        n.Type,
			FilePath:    n.FilePath,
			Line:        n.Line,
			IsVirtual:   n.IsVirtual,
			ParentID:    n.ParentID,
			Description: n.Description,
			AIRemarks:   n.AIRemarks,
		})
	}

	contextNodes := make([]SnapshotNodeRequest, 0, len(snapshot.ContextNodes))
	for _, n := range snapshot.ContextNodes {
		contextNodes = append(contextNodes, SnapshotNodeRequest{
			ID:          n.ID,
			Label:       n.Label,
			Type:        n.Type,
			FilePath:    n.FilePath,
			Line:        n.Line,
			IsVirtual:   n.IsVirtual,
			ParentID:    n.ParentID,
			Description: n.Description,
			AIRemarks:   n.AIRemarks,
		})
	}

	edgesReq := make([]SnapshotEdgeRequest, 0, len(snapshot.Edges))
	for _, e := range snapshot.Edges {
		edgesReq = append(edgesReq, SnapshotEdgeRequest{
			ID:       e.ID,
			SourceID: e.SourceID,
			TargetID: e.TargetID,
			Remarks:  e.Remarks,
		})
	}

	s.sendResponse(req.ID, SnapshotDetailResponse{
		ID:           snapshot.ID,
		Name:         snapshot.Name,
		VirtualNodes: virtualNodes,
		ContextNodes: contextNodes,
		Edges:        edgesReq,
		CreatedAt:    snapshot.CreatedAt,
	})
}

func (s *StdioServer) handleLoadSnapshot(req *StdioRequest) {
	id := s.getStringParam(req, "id")
	if id == "" {
		s.sendError(req.ID, ErrInvalidParams, "Missing id parameter", nil)
		return
	}

	snapshot, err := s.store.GetSnapshot(id)
	if err != nil {
		s.sendError(req.ID, ErrInternalError, "Snapshot not found", err.Error())
		return
	}

	restoredNodes, restoredEdges, missingContext, err := s.store.LoadSnapshotState(snapshot)
	if err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to load snapshot state", err.Error())
		return
	}

	s.sendResponse(req.ID, map[string]interface{}{
		"restoredVirtualNodes": restoredNodes,
		"restoredEdges":        restoredEdges,
		"missingContextNodes":  missingContext,
	})
}

func (s *StdioServer) handleDeleteSnapshot(req *StdioRequest) {
	id := s.getStringParam(req, "id")
	if id == "" {
		s.sendError(req.ID, ErrInvalidParams, "Missing id parameter", nil)
		return
	}

	if err := s.store.DeleteSnapshot(id); err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to delete snapshot", err.Error())
		return
	}

	s.sendResponse(req.ID, map[string]string{"message": "Snapshot deleted", "id": id})
}

func (s *StdioServer) handleGetSnapshotPrompt(req *StdioRequest) {
	id := s.getStringParam(req, "id")
	if id == "" {
		s.sendError(req.ID, ErrInvalidParams, "Missing id parameter", nil)
		return
	}

	snapshot, err := s.store.GetSnapshot(id)
	if err != nil {
		s.sendError(req.ID, ErrInternalError, "Snapshot not found", err.Error())
		return
	}

	var prompt string
	isCustom := false

	if snapshot.CustomPrompt != "" {
		prompt = snapshot.CustomPrompt
		isCustom = true
	} else {
		prompt = types.FormatSnapshotAsMarkdown(snapshot)
	}

	s.sendResponse(req.ID, SnapshotPromptResponse{
		Prompt:   prompt,
		IsCustom: isCustom,
	})
}

func (s *StdioServer) handleUpdateSnapshotPrompt(req *StdioRequest) {
	id := s.getStringParam(req, "id")
	prompt := s.getStringParam(req, "prompt")

	if id == "" {
		s.sendError(req.ID, ErrInvalidParams, "Missing id parameter", nil)
		return
	}
	if prompt == "" {
		s.sendError(req.ID, ErrInvalidParams, "Prompt is required", nil)
		return
	}

	if err := s.store.UpdateSnapshotCustomPrompt(id, prompt); err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to update prompt", err.Error())
		return
	}

	s.sendResponse(req.ID, map[string]string{"message": "Prompt updated", "id": id})
}

func (s *StdioServer) handleResetSnapshotPrompt(req *StdioRequest) {
	id := s.getStringParam(req, "id")
	if id == "" {
		s.sendError(req.ID, ErrInvalidParams, "Missing id parameter", nil)
		return
	}

	if err := s.store.ClearSnapshotCustomPrompt(id); err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to reset prompt", err.Error())
		return
	}

	snapshot, err := s.store.GetSnapshot(id)
	if err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to get snapshot", err.Error())
		return
	}

	s.sendResponse(req.ID, SnapshotPromptResponse{
		Prompt:   types.FormatSnapshotAsMarkdown(snapshot),
		IsCustom: false,
	})
}

// ============================================================================
// Helper functions
// ============================================================================

func parseSnapshotNode(m map[string]interface{}) types.SnapshotNode {
	node := types.SnapshotNode{}
	if v, ok := m["id"].(string); ok {
		node.ID = v
	}
	if v, ok := m["label"].(string); ok {
		node.Label = v
	}
	if v, ok := m["type"].(string); ok {
		node.Type = v
	}
	if v, ok := m["filePath"].(string); ok {
		node.FilePath = v
	}
	if v, ok := m["line"].(float64); ok {
		node.Line = int(v)
	}
	if v, ok := m["isVirtual"].(bool); ok {
		node.IsVirtual = v
	}
	if v, ok := m["parentId"].(string); ok {
		node.ParentID = v
	}
	if v, ok := m["description"].(string); ok {
		node.Description = v
	}
	if v, ok := m["aiRemarks"].(string); ok {
		node.AIRemarks = v
	}
	return node
}

func parseSnapshotEdge(m map[string]interface{}) types.SnapshotEdge {
	edge := types.SnapshotEdge{}
	if v, ok := m["id"].(string); ok {
		edge.ID = v
	}
	if v, ok := m["sourceId"].(string); ok {
		edge.SourceID = v
	}
	if v, ok := m["targetId"].(string); ok {
		edge.TargetID = v
	}
	if v, ok := m["remarks"].(string); ok {
		edge.Remarks = v
	}
	return edge
}
