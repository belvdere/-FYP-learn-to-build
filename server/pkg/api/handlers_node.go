package api

import (
	"example.com/fyp/pkg/types"
	"github.com/google/uuid"
)

// ============================================================================
// Node CRUD handlers
// ============================================================================

func (s *StdioServer) handleListNodes(req *StdioRequest) {
	nodes, err := s.store.GetAllGraphNodes()
	if err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to fetch nodes", err.Error())
		return
	}

	nodeResponses := make([]GraphNodeResponse, len(nodes))
	for i, node := range nodes {
		nodeResponses[i] = ToGraphNodeResponse(node)
	}

	s.sendResponse(req.ID, nodeResponses)
}

func (s *StdioServer) handleGetNode(req *StdioRequest) {
	nodeID := s.getStringParam(req, "id")
	if nodeID == "" {
		s.sendError(req.ID, ErrInvalidParams, "Missing id parameter", nil)
		return
	}

	node, err := s.store.GetGraphNode(nodeID)
	if err != nil {
		s.sendError(req.ID, ErrInternalError, "Node not found", err.Error())
		return
	}

	s.sendResponse(req.ID, ToGraphNodeResponse(*node))
}

func (s *StdioServer) handleCreateNode(req *StdioRequest) {
	label := s.getStringParam(req, "label")
	nodeType := s.getStringParam(req, "type")

	if label == "" {
		s.sendError(req.ID, ErrInvalidParams, "Label is required", nil)
		return
	}
	if nodeType == "" {
		s.sendError(req.ID, ErrInvalidParams, "Type is required", nil)
		return
	}

	node := types.VirtualNode{
		ID:               uuid.New().String(),
		Label:            label,
		Type:             nodeType,
		VirtualClassID:   s.getStringParam(req, "virtualClassId"),
		VirtualDirectory: s.getStringParam(req, "virtualDirectory"),
		IsVirtual:        true,
	}

	if err := s.store.InsertVirtualNode(node); err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to create node", err.Error())
		return
	}

	createdNode, err := s.store.GetGraphNode(node.ID)
	if err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to fetch created node", err.Error())
		return
	}

	s.sendResponse(req.ID, ToGraphNodeResponse(*createdNode))
}

func (s *StdioServer) handleUpdateNode(req *StdioRequest) {
	nodeID := s.getStringParam(req, "id")
	if nodeID == "" {
		s.sendError(req.ID, ErrInvalidParams, "Missing id parameter", nil)
		return
	}

	existingNode, err := s.store.GetVirtualNode(nodeID)
	if err != nil {
		s.sendError(req.ID, ErrInternalError, "Virtual node not found", err.Error())
		return
	}

	if label := s.getStringParam(req, "label"); label != "" {
		existingNode.Label = label
	}
	if nodeType := s.getStringParam(req, "type"); nodeType != "" {
		existingNode.Type = nodeType
	}
	existingNode.VirtualClassID = s.getStringParam(req, "virtualClassId")
	existingNode.VirtualDirectory = s.getStringParam(req, "virtualDirectory")
	if materialized, ok := s.getBoolParam(req, "materialized"); ok {
		existingNode.Materialized = materialized
	}

	if err := s.store.UpdateVirtualNode(*existingNode); err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to update node", err.Error())
		return
	}

	updatedNode, err := s.store.GetGraphNode(nodeID)
	if err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to fetch updated node", err.Error())
		return
	}

	s.sendResponse(req.ID, ToGraphNodeResponse(*updatedNode))
}

func (s *StdioServer) handleDeleteNode(req *StdioRequest) {
	nodeID := s.getStringParam(req, "id")
	if nodeID == "" {
		s.sendError(req.ID, ErrInvalidParams, "Missing id parameter", nil)
		return
	}

	if err := s.store.DeleteVirtualNodeCascade(nodeID); err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to delete node", err.Error())
		return
	}

	s.sendResponse(req.ID, map[string]string{"message": "Node deleted successfully", "id": nodeID})
}

// ============================================================================
// Node annotation handlers
// ============================================================================

func (s *StdioServer) handleGetNodeAnnotation(req *StdioRequest) {
	nodeID := s.getStringParam(req, "nodeId")
	if nodeID == "" {
		s.sendError(req.ID, ErrInvalidParams, "Missing nodeId parameter", nil)
		return
	}

	annotation, err := s.store.GetNodeAnnotation(nodeID)
	if err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to fetch annotation", err.Error())
		return
	}

	if annotation == nil {
		s.sendResponse(req.ID, NodeAnnotationResponse{})
		return
	}

	s.sendResponse(req.ID, NodeAnnotationResponse{
		Description:      annotation.Description,
		AIRemarks:        annotation.AIRemarks,
		CodeSnippet:      annotation.CodeSnippet,
		CustomProperties: annotation.CustomProperties,
	})
}

func (s *StdioServer) handleUpdateNodeAnnotation(req *StdioRequest) {
	nodeID := s.getStringParam(req, "nodeId")
	if nodeID == "" {
		s.sendError(req.ID, ErrInvalidParams, "Missing nodeId parameter", nil)
		return
	}

	// Load existing annotation to avoid wiping fields not included in this request
	existing, _ := s.store.GetNodeAnnotation(nodeID)

	annotation := types.NodeAnnotation{NodeID: nodeID}
	if existing != nil {
		annotation.Description = existing.Description
		annotation.AIRemarks = existing.AIRemarks
		annotation.CodeSnippet = existing.CodeSnippet
		annotation.CustomProperties = existing.CustomProperties
	}

	// Only overwrite fields explicitly provided in the request
	if v := s.getStringParam(req, "description"); v != "" || s.hasParam(req, "description") {
		annotation.Description = v
	}
	if v := s.getStringParam(req, "aiRemarks"); v != "" || s.hasParam(req, "aiRemarks") {
		annotation.AIRemarks = v
	}
	if v := s.getStringParam(req, "codeSnippet"); v != "" || s.hasParam(req, "codeSnippet") {
		annotation.CodeSnippet = v
	}
	if v := s.getStringParam(req, "customProperties"); v != "" || s.hasParam(req, "customProperties") {
		annotation.CustomProperties = v
	}

	if err := s.store.UpsertNodeAnnotation(annotation); err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to update annotation", err.Error())
		return
	}

	updated, err := s.store.GetNodeAnnotation(nodeID)
	if err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to fetch updated annotation", err.Error())
		return
	}

	s.sendResponse(req.ID, NodeAnnotationResponse{
		Description:      updated.Description,
		AIRemarks:        updated.AIRemarks,
		CodeSnippet:      updated.CodeSnippet,
		CustomProperties: updated.CustomProperties,
	})
}
