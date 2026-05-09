package api

import (
	"example.com/fyp/pkg/types"
)

// ============================================================================
// Graph query handlers
// ============================================================================

// handleGetGraph returns the full call graph: nodes (symbols + virtual), edges (method calls),
// and file/directory metadata for the tree view.
func (s *StdioServer) handleGetGraph(req *StdioRequest) {
	nodes, err := s.store.GetAllGraphNodes()
	if err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to fetch nodes", err.Error())
		return
	}

	methodCalls, err := s.store.GetAllMethodCalls()
	if err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to fetch method calls", err.Error())
		return
	}

	edgeAnnotations, err := s.store.GetAllEdgeAnnotations()
	if err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to fetch edge annotations", err.Error())
		return
	}

	// Build edge annotation map
	edgeAnnotationMap := make(map[string]*types.EdgeAnnotation)
	for i := range edgeAnnotations {
		edgeAnnotationMap[edgeAnnotations[i].EdgeID] = &edgeAnnotations[i]
	}

	// Compute parent IDs for the hierarchy (file/class/directory nesting)
	nodes = computeNodeParentIDs(nodes)

	// Map symbol name (e.g. "UserService.save") to node ID for edge resolution
	symbolToNodeID := make(map[string]string)
	for _, node := range nodes {
		symbolToNodeID[node.Label] = node.ID
		symbolToNodeID[node.ID] = node.ID
	}

	// Convert nodes
	nodeResponses := make([]GraphNodeResponse, len(nodes))
	for i, node := range nodes {
		nodeResponses[i] = ToGraphNodeResponse(node)
	}

	// Add virtual directories as drawable graph nodes (they can be edge endpoints)
	virtualDirs, err := s.store.GetAllVirtualDirectories()
	if err != nil {
		// Non-fatal: log and continue with an empty list
		virtualDirs = nil
		_ = err // already logged by the store; graph can still render without virtual dirs
	}
	for _, vd := range virtualDirs {
		dirNodeID := hashString(vd.Path)
		symbolToNodeID[vd.Name] = dirNodeID
		symbolToNodeID[dirNodeID] = dirNodeID // so edges whose CallerID/CalleeID is a dir hash resolve
		nodeResponses = append(nodeResponses, GraphNodeResponse{
			ID:        dirNodeID,
			Label:     vd.Name,
			Type:      "directory",
			IsVirtual: true,
			ParentID:  "",
		})
	}

	// Build file/directory data (real files only)
	files, directories := buildFileAndDirectoryData(nodes)

	// Aggregate method_calls into edges: multiple calls between same pair become one edge with count
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
			annotation := edgeAnnotationMap[edgeID]
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

	response := GraphData{
		Nodes:       nodeResponses,
		Edges:       edgeResponses,
		Files:       files,
		Directories: directories,
	}

	s.sendResponse(req.ID, response)
}

// handleGetNeighborhood returns a node plus its direct neighbors (callers and callees).
// Used when user double-clicks a node to expand it in the graph.
func (s *StdioServer) handleGetNeighborhood(req *StdioRequest) {
	nodeID := s.getStringParam(req, "nodeId")
	if nodeID == "" {
		s.sendError(req.ID, ErrInvalidParams, "Missing nodeId parameter", nil)
		return
	}

	node, err := s.store.GetGraphNode(nodeID)
	if err != nil {
		s.sendError(req.ID, ErrInternalError, "Node not found", err.Error())
		return
	}

	allNodes, err := s.store.GetAllGraphNodes()
	if err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to fetch nodes", err.Error())
		return
	}

	symbolToNodeID := make(map[string]string)
	nodeIDToNode := make(map[string]types.GraphNode)
	for _, n := range allNodes {
		symbolToNodeID[n.Label] = n.ID
		symbolToNodeID[n.ID] = n.ID
		nodeIDToNode[n.ID] = n
	}

	nodeSymbol := node.Label
	nodeSymbolID := node.ID

	methodCalls, err := s.store.GetAllMethodCalls()
	if err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to fetch method calls", err.Error())
		return
	}

	// Pre-fetch all edge annotations to avoid one DB query per edge
	allEdgeAnnotations, err := s.store.GetAllEdgeAnnotations()
	if err != nil {
		allEdgeAnnotations = nil
	}
	neighborEdgeAnnotationMap := make(map[string]*types.EdgeAnnotation, len(allEdgeAnnotations))
	for i := range allEdgeAnnotations {
		neighborEdgeAnnotationMap[allEdgeAnnotations[i].EdgeID] = &allEdgeAnnotations[i]
	}

	neighborIDs := make(map[string]bool)
	type nAgg struct {
		resp     GraphEdgeResponse
		total    int
		isManual bool
	}
	edgeAgg := make(map[string]*nAgg)

	for _, call := range methodCalls {
		var otherSymbol string
		isMatch := false

		if call.CallerID == nodeSymbol || call.CallerID == nodeSymbolID {
			otherSymbol = call.CalleeID
			isMatch = true
		} else if call.CalleeID == nodeSymbol || call.CalleeID == nodeSymbolID {
			otherSymbol = call.CallerID
			isMatch = true
		}

		if isMatch {
			sourceID := symbolToNodeID[call.CallerID]
			targetID := symbolToNodeID[call.CalleeID]
			otherID := symbolToNodeID[otherSymbol]

			if sourceID == "" || targetID == "" {
				continue
			}

			if otherID != "" {
				neighborIDs[otherID] = true
			}

			edgeID := createDirectedEdgeID(sourceID, targetID)

			existing := edgeAgg[edgeID]
			if existing == nil {
				existing = &nAgg{
					resp: GraphEdgeResponse{
						ID:     edgeID,
						Source: sourceID,
						Target: targetID,
					},
				}
				if annotation := neighborEdgeAnnotationMap[edgeID]; annotation != nil {
					existing.resp.Annotation = &EdgeAnnotationResponse{Remarks: annotation.Remarks}
				}
				edgeAgg[edgeID] = existing
			}

			if call.CallType == "manual" {
				existing.isManual = true
			}
			existing.total++
		}
	}

	edgeResponses := make([]GraphEdgeResponse, 0, len(edgeAgg))
	for _, a := range edgeAgg {
		a.resp.Count = a.total
		a.resp.IsManual = a.isManual
		edgeResponses = append(edgeResponses, a.resp)
	}

	nodeList := make([]types.GraphNode, 0, len(neighborIDs)+1)
	nodeList = append(nodeList, *node)
	for neighborID := range neighborIDs {
		if neighborNode, exists := nodeIDToNode[neighborID]; exists {
			nodeList = append(nodeList, neighborNode)
		}
	}
	nodeList = computeNodeParentIDs(nodeList)
	files, directories := buildFileAndDirectoryData(nodeList)

	resultNodes := make([]GraphNodeResponse, 0, len(nodeList))
	for _, n := range nodeList {
		resultNodes = append(resultNodes, ToGraphNodeResponse(n))
	}

	response := GraphData{
		Nodes:       resultNodes,
		Edges:       edgeResponses,
		Files:       files,
		Directories: directories,
	}

	s.sendResponse(req.ID, response)
}

func (s *StdioServer) handleSearchNodes(req *StdioRequest) {
	query := s.getStringParam(req, "q")
	if query == "" {
		s.sendError(req.ID, ErrInvalidParams, "Missing q (query) parameter", nil)
		return
	}

	limit := s.getIntParam(req, "limit", 50)

	nodes, err := s.store.SearchGraphNodes(query)
	if err != nil {
		s.sendError(req.ID, ErrInternalError, "Search failed", err.Error())
		return
	}

	if len(nodes) > limit {
		nodes = nodes[:limit]
	}

	nodeResponses := make([]GraphNodeResponse, len(nodes))
	for i, node := range nodes {
		nodeResponses[i] = ToGraphNodeResponse(node)
	}

	s.sendResponse(req.ID, nodeResponses)
}

func (s *StdioServer) handleClearVirtual(req *StdioRequest) {
	if err := s.store.ClearAllVirtualState(); err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to clear virtual state", err.Error())
		return
	}
	s.sendResponse(req.ID, map[string]bool{"success": true})
}

func (s *StdioServer) handleGetFileTree(req *StdioRequest) {
	builder := NewFileTreeBuilder(s.store, s.workspaceRoot, s.getSupportedSourceExtensions())
	tree, err := builder.BuildFileTree()
	if err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to build file tree", err.Error())
		return
	}
	s.sendResponse(req.ID, tree)
}

func (s *StdioServer) getSupportedSourceExtensions() []string {
	seen := make(map[string]struct{})
	exts := make([]string, 0, 8)

	for _, language := range s.langController.GetSupportedLanguages() {
		plugin, err := s.langController.GetPluginByLanguage(language)
		if err != nil {
			continue
		}
		for _, ext := range plugin.Extensions() {
			if ext == "" {
				continue
			}
			if _, ok := seen[ext]; ok {
				continue
			}
			seen[ext] = struct{}{}
			exts = append(exts, ext)
		}
	}

	return exts
}
