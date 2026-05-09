package api

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"example.com/fyp/pkg/db"
	"example.com/fyp/pkg/indexer"
	"example.com/fyp/pkg/types"
)

type indexScanResponse struct {
	Symbols []types.ScannedSymbol `json:"symbols"`
}

type indexStoreParams struct {
	Rebuild bool                  `json:"rebuild"`
	Symbols []types.ScannedSymbol `json:"symbols"`
	Edges   []types.MethodCall    `json:"edges"`
}

type indexScanFileResponse struct {
	Symbols          []types.ScannedSymbol `json:"symbols"`
	ChangedCallerIDs []string              `json:"changedCallerIds"`
	RemovedCallerIDs []string              `json:"removedCallerIds"`
	FileHash         string                `json:"fileHash"`
}

type indexResolveLocationsParams struct {
	Locations []types.SymbolLocation `json:"locations"`
}

type indexStoreFileDeltaParams struct {
	FilePath         string                `json:"filePath"`
	FileHash         string                `json:"fileHash"`
	Symbols          []types.ScannedSymbol `json:"symbols"`
	ChangedCallerIDs []string              `json:"changedCallerIds"`
	RemovedCallerIDs []string              `json:"removedCallerIds"`
	Edges            []types.MethodCall    `json:"edges"`
}

// handleIndexRun is kept as a no-op compatibility stub.
func (s *StdioServer) handleIndexRun(req *StdioRequest) {
	s.sendResponse(req.ID, map[string]interface{}{
		"success":    true,
		"deprecated": true,
		"message":    "index.run is deprecated; use index.scan + index.storeEdges",
	})
}

func (s *StdioServer) handleIndexScan(req *StdioRequest) {
	sc := indexer.NewScanner()
	defer sc.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	symbols, err := sc.ScanSymbols(ctx, s.workspaceRoot)
	if err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to scan symbols", err.Error())
		return
	}

	s.sendResponse(req.ID, indexScanResponse{Symbols: symbols})
}

func (s *StdioServer) handleIndexStoreEdges(req *StdioRequest) {
	var params indexStoreParams
	symbolsProvided := false
	if req.Params != nil {
		if v, ok := req.Params["rebuild"].(bool); ok {
			params.Rebuild = v
		}
		if rawSymbolsValue, exists := req.Params["symbols"]; exists {
			symbolsProvided = true
			rawSymbols, ok := rawSymbolsValue.([]interface{})
			if !ok {
				s.sendError(req.ID, ErrInvalidParams, "symbols must be an array", nil)
				return
			}
			params.Symbols = make([]types.ScannedSymbol, 0, len(rawSymbols))
			for _, r := range rawSymbols {
				m, ok := r.(map[string]interface{})
				if !ok {
					continue
				}
				params.Symbols = append(params.Symbols, types.ScannedSymbol{
					ID:        toString(m["id"]),
					Name:      toString(m["name"]),
					Kind:      toString(m["kind"]),
					FilePath:  toString(m["filePath"]),
					URI:       toString(m["uri"]),
					Line:      toInt(m["line"]),
					EndLine:   toInt(m["endLine"]),
					Col:       toInt(m["col"]),
					Signature: toString(m["signature"]),
					BodyHash:  toString(m["bodyHash"]),
				})
			}
		}
		if rawEdgesValue, exists := req.Params["edges"]; exists {
			rawEdges, ok := rawEdgesValue.([]interface{})
			if !ok {
				s.sendError(req.ID, ErrInvalidParams, "edges must be an array", nil)
				return
			}
			params.Edges = make([]types.MethodCall, 0, len(rawEdges))
			for _, r := range rawEdges {
				m, ok := r.(map[string]interface{})
				if !ok {
					continue
				}
				params.Edges = append(params.Edges, types.MethodCall{
					CallerID: toString(m["callerSymbol"]),
					CalleeID: toString(m["calleeSymbol"]),
					CallType: toString(m["callType"]),
					FilePath: toString(m["filePath"]),
					Line:     toInt(m["line"]),
				})
			}
		}
	}

	if !symbolsProvided {
		s.sendError(req.ID, ErrInvalidParams, "symbols is required", nil)
		return
	}

	started := time.Now()
	convertedSymbols := make([]types.Symbol, 0, len(params.Symbols))
	for _, sym := range params.Symbols {
		convertedSymbols = append(convertedSymbols, types.Symbol{
			ID:        sym.ID,
			Name:      sym.Name,
			Kind:      sym.Kind,
			FilePath:  sym.FilePath,
			Line:      sym.Line,
			EndLine:   sym.EndLine,
			Signature: sym.Signature,
			BodyHash:  sym.BodyHash,
		})
	}

	if err := s.store.ReplaceIndexData(convertedSymbols, params.Edges, params.Rebuild); err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to store edges", err.Error())
		return
	}

	s.sendResponse(req.ID, map[string]interface{}{
		"success": true,
		"stats": map[string]interface{}{
			"symbols":    len(convertedSymbols),
			"edges":      len(params.Edges),
			"durationMs": time.Since(started).Milliseconds(),
		},
	})
}

// handleIndexAppendEdges appends a batch of edges without touching symbols.
// Called after the first index.storeEdges to send remaining edge batches.
func (s *StdioServer) handleIndexAppendEdges(req *StdioRequest) {
	var edges []types.MethodCall
	if rawEdges, ok := req.Params["edges"].([]interface{}); ok {
		edges = make([]types.MethodCall, 0, len(rawEdges))
		for _, r := range rawEdges {
			m, ok := r.(map[string]interface{})
			if !ok {
				continue
			}
			edges = append(edges, types.MethodCall{
				CallerID: toString(m["callerSymbol"]),
				CalleeID: toString(m["calleeSymbol"]),
				CallType: toString(m["callType"]),
				FilePath: toString(m["filePath"]),
				Line:     toInt(m["line"]),
			})
		}
	}

	if err := s.store.BulkInsertEdges(edges); err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to append edges", err.Error())
		return
	}
	s.sendResponse(req.ID, map[string]interface{}{"success": true, "edges": len(edges)})
}

func (s *StdioServer) handleIndexClearSymbols(req *StdioRequest) {
	if err := s.store.ReplaceIndexData(nil, nil, true); err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to clear symbols", err.Error())
		return
	}
	s.sendResponse(req.ID, map[string]interface{}{"success": true})
}

func (s *StdioServer) handleIndexScanFile(req *StdioRequest) {
	filePath := s.getStringParam(req, "filePath")
	if filePath == "" {
		s.sendError(req.ID, ErrInvalidParams, "filePath is required", nil)
		return
	}
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(s.workspaceRoot, filePath)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to read source file", err.Error())
		return
	}
	fileHash := db.ComputeFileHash(content)

	sc := indexer.NewScanner()
	defer sc.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	symbols, err := sc.ScanFileSymbols(ctx, filePath)
	if err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to scan file symbols", err.Error())
		return
	}

	existing, err := s.store.GetSymbolsByFile(filePath)
	if err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to fetch previous file symbols", err.Error())
		return
	}

	oldHashes := make(map[string]string, len(existing))
	for _, sym := range existing {
		if sym.Kind != "method" && sym.Kind != "constructor" {
			continue
		}
		oldHashes[sym.ID] = sym.BodyHash
	}
	newHashes := make(map[string]string, len(symbols))
	changed := make([]string, 0, len(symbols))
	for _, sym := range symbols {
		newHashes[sym.ID] = sym.BodyHash
		if oldHash, ok := oldHashes[sym.ID]; !ok || oldHash != sym.BodyHash {
			changed = append(changed, sym.ID)
		}
	}

	removed := make([]string, 0)
	for oldID := range oldHashes {
		if _, ok := newHashes[oldID]; !ok {
			removed = append(removed, oldID)
		}
	}

	s.sendResponse(req.ID, indexScanFileResponse{
		Symbols:          symbols,
		ChangedCallerIDs: changed,
		RemovedCallerIDs: removed,
		FileHash:         fileHash,
	})
}

func (s *StdioServer) handleResolveSymbolsByLocation(req *StdioRequest) {
	var params indexResolveLocationsParams
	if req.Params != nil {
		rawLocations, exists := req.Params["locations"]
		if !exists {
			s.sendError(req.ID, ErrInvalidParams, "locations is required", nil)
			return
		}
		raw, ok := rawLocations.([]interface{})
		if !ok {
			s.sendError(req.ID, ErrInvalidParams, "locations must be an array", nil)
			return
		}
		params.Locations = make([]types.SymbolLocation, 0, len(raw))
		for _, item := range raw {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			params.Locations = append(params.Locations, types.SymbolLocation{
				FilePath: toString(m["filePath"]),
				Line:     toInt(m["line"]),
			})
		}
	}

	resolved, err := s.store.ResolveSymbolIDsByLocations(params.Locations)
	if err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to resolve symbols by location", err.Error())
		return
	}
	s.sendResponse(req.ID, map[string]interface{}{"resolved": resolved})
}

func (s *StdioServer) handleIndexStoreFileDelta(req *StdioRequest) {
	var params indexStoreFileDeltaParams
	if req.Params != nil {
		params.FilePath = toString(req.Params["filePath"])
		params.FileHash = toString(req.Params["fileHash"])

		if rawChanged, ok := req.Params["changedCallerIds"].([]interface{}); ok {
			params.ChangedCallerIDs = make([]string, 0, len(rawChanged))
			for _, v := range rawChanged {
				params.ChangedCallerIDs = append(params.ChangedCallerIDs, toString(v))
			}
		}
		if rawRemoved, ok := req.Params["removedCallerIds"].([]interface{}); ok {
			params.RemovedCallerIDs = make([]string, 0, len(rawRemoved))
			for _, v := range rawRemoved {
				params.RemovedCallerIDs = append(params.RemovedCallerIDs, toString(v))
			}
		}
		if rawSymbols, ok := req.Params["symbols"].([]interface{}); ok {
			params.Symbols = make([]types.ScannedSymbol, 0, len(rawSymbols))
			for _, r := range rawSymbols {
				m, ok := r.(map[string]interface{})
				if !ok {
					continue
				}
				params.Symbols = append(params.Symbols, types.ScannedSymbol{
					ID:        toString(m["id"]),
					Name:      toString(m["name"]),
					Kind:      toString(m["kind"]),
					FilePath:  toString(m["filePath"]),
					URI:       toString(m["uri"]),
					Line:      toInt(m["line"]),
					EndLine:   toInt(m["endLine"]),
					Col:       toInt(m["col"]),
					Signature: toString(m["signature"]),
					BodyHash:  toString(m["bodyHash"]),
				})
			}
		}
		if rawEdges, ok := req.Params["edges"].([]interface{}); ok {
			params.Edges = make([]types.MethodCall, 0, len(rawEdges))
			for _, r := range rawEdges {
				m, ok := r.(map[string]interface{})
				if !ok {
					continue
				}
				params.Edges = append(params.Edges, types.MethodCall{
					CallerID: toString(m["callerSymbol"]),
					CalleeID: toString(m["calleeSymbol"]),
					CallType: toString(m["callType"]),
					FilePath: toString(m["filePath"]),
					Line:     toInt(m["line"]),
				})
			}
		}
	}

	if params.FilePath == "" {
		s.sendError(req.ID, ErrInvalidParams, "filePath is required", nil)
		return
	}
	if !filepath.IsAbs(params.FilePath) {
		params.FilePath = filepath.Join(s.workspaceRoot, params.FilePath)
	}

	converted := make([]types.Symbol, 0, len(params.Symbols))
	for _, sym := range params.Symbols {
		converted = append(converted, types.Symbol{
			ID:        sym.ID,
			Name:      sym.Name,
			Kind:      sym.Kind,
			FilePath:  sym.FilePath,
			Line:      sym.Line,
			EndLine:   sym.EndLine,
			Signature: sym.Signature,
			BodyHash:  sym.BodyHash,
		})
	}

	started := time.Now()
	if err := s.store.ApplyFileDelta(
		params.FilePath,
		params.FileHash,
		converted,
		params.ChangedCallerIDs,
		params.RemovedCallerIDs,
		params.Edges,
	); err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to store file delta", err.Error())
		return
	}

	s.sendResponse(req.ID, map[string]interface{}{
		"success": true,
		"stats": map[string]interface{}{
			"symbols":    len(converted),
			"edges":      len(params.Edges),
			"durationMs": time.Since(started).Milliseconds(),
		},
	})
}

func toString(v interface{}) string {
	s, _ := v.(string)
	return s
}

func toInt(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case float32:
		return int(n)
	default:
		return 0
	}
}
