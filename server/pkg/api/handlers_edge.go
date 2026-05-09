package api

import (
	"fmt"
	"strings"

	"example.com/fyp/pkg/types"
)

// ============================================================================
// Edge CRUD handlers
// ============================================================================

func (s *StdioServer) handleListEdges(req *StdioRequest) {
	nodes, err := s.store.GetAllGraphNodes()
	if err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to fetch nodes", err.Error())
		return
	}

	symbolToNodeID := make(map[string]string)
	for _, node := range nodes {
		symbolToNodeID[node.Label] = node.ID
		symbolToNodeID[node.ID] = node.ID
	}

	// Also add virtual directories so edges whose endpoints are dir-hash IDs resolve correctly
	if virtualDirs, vdErr := s.store.GetAllVirtualDirectories(); vdErr == nil {
		for _, vd := range virtualDirs {
			dirNodeID := hashString(vd.Path)
			symbolToNodeID[vd.Name] = dirNodeID
			symbolToNodeID[dirNodeID] = dirNodeID
		}
	}

	methodCalls, err := s.store.GetAllMethodCalls()
	if err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to fetch edges", err.Error())
		return
	}

	edgeAnnotations, err := s.store.GetAllEdgeAnnotations()
	if err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to fetch annotations", err.Error())
		return
	}

	annotationMap := make(map[string]*types.EdgeAnnotation)
	for i := range edgeAnnotations {
		annotationMap[edgeAnnotations[i].EdgeID] = &edgeAnnotations[i]
	}

	type agg struct {
		resp     GraphEdgeResponse
		total    int
		isManual bool
	}
	edgeAgg := make(map[string]*agg)

	for _, call := range methodCalls {
		sourceID := symbolToNodeID[call.CallerID]
		targetID := symbolToNodeID[call.CalleeID]
		if sourceID == "" || targetID == "" {
			continue
		}

		edgeID := createDirectedEdgeID(sourceID, targetID)

		existing := edgeAgg[edgeID]
		if existing == nil {
			annotation := annotationMap[edgeID]
			existing = &agg{
				resp: GraphEdgeResponse{
					ID:     edgeID,
					Source: sourceID,
					Target: targetID,
				},
			}
			if annotation != nil {
				existing.resp.Annotation = &EdgeAnnotationResponse{Remarks: annotation.Remarks}
			}
			edgeAgg[edgeID] = existing
		}

		if call.CallType == "manual" {
			existing.isManual = true
		}
		existing.total++
	}

	edgeResponses := make([]GraphEdgeResponse, 0, len(edgeAgg))
	for _, a := range edgeAgg {
		a.resp.Count = a.total
		a.resp.IsManual = a.isManual
		edgeResponses = append(edgeResponses, a.resp)
	}

	s.sendResponse(req.ID, edgeResponses)
}

func (s *StdioServer) handleCreateEdge(req *StdioRequest) {
	sourceNodeID := s.getStringParam(req, "sourceNodeId")
	targetNodeID := s.getStringParam(req, "targetNodeId")
	remarks := s.getStringParam(req, "remarks")

	if sourceNodeID == "" || targetNodeID == "" {
		s.sendError(req.ID, ErrInvalidParams, "Source and target node IDs are required", nil)
		return
	}

	sourceNode, err := s.resolveEdgeEndpoint(sourceNodeID)
	if err != nil {
		s.sendError(req.ID, ErrInternalError, "Source node not found", sourceNodeID)
		return
	}

	targetNode, err := s.resolveEdgeEndpoint(targetNodeID)
	if err != nil {
		s.sendError(req.ID, ErrInternalError, "Target node not found", targetNodeID)
		return
	}

	call := types.MethodCall{
		CallerID: sourceNode.ID,
		CalleeID: targetNode.ID,
		CallType: "manual",
		FilePath: "",
		Line:     0,
	}

	if err := s.store.InsertMethodCall(call); err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to create edge", err.Error())
		return
	}

	edgeID := createDirectedEdgeID(sourceNode.ID, targetNode.ID)

	if remarks != "" {
		annotation := types.EdgeAnnotation{
			EdgeID:       edgeID,
			SourceNodeID: sourceNode.ID,
			TargetNodeID: targetNode.ID,
			Remarks:      remarks,
			CallCount:    1,
		}
		_ = s.store.UpsertEdgeAnnotation(annotation)
	}

	annotation, _ := s.store.GetEdgeAnnotation(edgeID)

	response := GraphEdgeResponse{
		ID:       edgeID,
		Source:   sourceNode.ID,
		Target:   targetNode.ID,
		Count:    1,
		IsManual: true,
	}

	if annotation != nil {
		response.Annotation = &EdgeAnnotationResponse{Remarks: annotation.Remarks}
		response.Count = annotation.CallCount
	}

	s.sendResponse(req.ID, response)
}

func (s *StdioServer) handleDeleteEdge(req *StdioRequest) {
	edgeID := s.getStringParam(req, "id")
	if edgeID == "" {
		s.sendError(req.ID, ErrInvalidParams, "Missing id parameter", nil)
		return
	}

	// Parse edge ID (format: sourceNodeID::targetNodeID) to get node IDs
	parts := splitEdgeID(edgeID)
	if len(parts) == 2 {
		// Delete manual edge rows directly by endpoint IDs.
		// This works for symbol IDs, virtual node IDs, and virtual-directory hash IDs.
		_ = s.store.DeleteManualMethodCall(parts[0], parts[1])
	}

	// Always remove edge annotation (remarks, etc.)
	_ = s.store.DeleteEdgeAnnotation(edgeID)
	s.sendResponse(req.ID, map[string]string{"message": "Edge deleted successfully", "id": edgeID})
}

// resolveEdgeEndpoint resolves any graph-visible node ID that can appear as an edge endpoint.
// Supports symbol/virtual-node IDs via GetGraphNode, plus virtual-directory hash IDs.
func (s *StdioServer) resolveEdgeEndpoint(nodeID string) (*types.GraphNode, error) {
	node, err := s.store.GetGraphNode(nodeID)
	if err == nil {
		return node, nil
	}

	virtualDirs, vdErr := s.store.GetAllVirtualDirectories()
	if vdErr != nil {
		return nil, fmt.Errorf("resolve endpoint %s: %w", nodeID, vdErr)
	}

	for _, vd := range virtualDirs {
		if hashString(vd.Path) == nodeID {
			return &types.GraphNode{
				ID:        nodeID,
				Label:     vd.Name,
				Type:      "directory",
				FilePath:  vd.Path,
				IsVirtual: true,
			}, nil
		}
	}

	return nil, fmt.Errorf("node not found: %s", nodeID)
}

// ============================================================================
// Edge annotation handlers
// ============================================================================

func (s *StdioServer) handleGetEdgeAnnotation(req *StdioRequest) {
	edgeID := s.getStringParam(req, "edgeId")
	if edgeID == "" {
		s.sendError(req.ID, ErrInvalidParams, "Missing edgeId parameter", nil)
		return
	}

	annotation, err := s.store.GetEdgeAnnotation(edgeID)
	if err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to fetch annotation", err.Error())
		return
	}

	if annotation == nil {
		s.sendResponse(req.ID, EdgeAnnotationResponse{})
		return
	}

	s.sendResponse(req.ID, EdgeAnnotationResponse{Remarks: annotation.Remarks})
}

func (s *StdioServer) handleUpdateEdgeAnnotation(req *StdioRequest) {
	edgeID := s.getStringParam(req, "edgeId")
	if edgeID == "" {
		s.sendError(req.ID, ErrInvalidParams, "Missing edgeId parameter", nil)
		return
	}

	remarks := s.getStringParam(req, "remarks")

	// Parse edge ID to get source and target
	parts := splitEdgeID(edgeID)
	if len(parts) != 2 {
		s.sendError(req.ID, ErrInvalidParams, "Invalid edge ID format", nil)
		return
	}

	callCount := 1
	if existing, _ := s.store.GetEdgeAnnotation(edgeID); existing != nil {
		callCount = existing.CallCount
	}

	annotation := types.EdgeAnnotation{
		EdgeID:       edgeID,
		SourceNodeID: parts[0],
		TargetNodeID: parts[1],
		Remarks:      remarks,
		CallCount:    callCount,
	}

	if err := s.store.UpsertEdgeAnnotation(annotation); err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to update annotation", err.Error())
		return
	}

	updated, err := s.store.GetEdgeAnnotation(edgeID)
	if err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to fetch updated annotation", err.Error())
		return
	}

	if updated == nil {
		s.sendResponse(req.ID, EdgeAnnotationResponse{})
		return
	}

	s.sendResponse(req.ID, EdgeAnnotationResponse{Remarks: updated.Remarks})
}

// ============================================================================
// Helper functions
// ============================================================================

func splitEdgeID(edgeID string) []string {
	// Edge ID format: "sourceId::targetId"
	if idx := strings.Index(edgeID, edgeSeparator); idx != -1 {
		return []string{edgeID[:idx], edgeID[idx+len(edgeSeparator):]}
	}
	return nil
}
