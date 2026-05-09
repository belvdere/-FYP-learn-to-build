package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
	"sync/atomic"

	"example.com/fyp/pkg/db"
	"example.com/fyp/pkg/lang"
)

// StdioServer handles JSON-RPC requests over stdio for the graph API.
// Used by the VS Code extension to communicate with graph-viz webview.
// All requests arrive via stdin (newline-delimited JSON); responses go to stdout.
type StdioServer struct {
	store          *db.Store
	workspaceRoot  string
	reader         *bufio.Reader
	writer         io.Writer
	writeMu        sync.Mutex // Protects writer for concurrent access
	langController *lang.LanguageController

	// Indexing state
	indexing      atomic.Bool
	indexCancel   context.CancelFunc
	indexCancelMu sync.Mutex
}

// StdioRequest represents a JSON-RPC request.
type StdioRequest struct {
	ID     interface{}            `json:"id"`
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// StdioResponse represents a JSON-RPC response.
type StdioResponse struct {
	ID     interface{} `json:"id"`
	Result interface{} `json:"result,omitempty"`
	Error  *StdioError `json:"error,omitempty"`
}

// StdioError represents an error in the response.
type StdioError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Error codes
const (
	ErrParseError     = -32700
	ErrInvalidRequest = -32600
	ErrMethodNotFound = -32601
	ErrInvalidParams  = -32602
	ErrInternalError  = -32603
)

// NewStdioServer creates a new stdio API server.
func NewStdioServer(store *db.Store, workspaceRoot string) *StdioServer {
	return &StdioServer{
		store:          store,
		workspaceRoot:  workspaceRoot,
		reader:         bufio.NewReader(os.Stdin),
		writer:         os.Stdout,
		langController: lang.NewLanguageControllerWithDefaults(),
	}
}

// Serve starts the stdio server loop. Blocks until EOF or error.
// Each line from stdin is a JSON-RPC request; responses are written to stdout.
func (s *StdioServer) Serve() error {
	fmt.Fprintln(os.Stderr, "fypd: API stdio server ready")

	for {
		// Read one line (newline-delimited JSON per JSON-RPC over stdio)
		line, err := s.reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read request: %w", err)
		}

		var req StdioRequest
		if err := json.Unmarshal(line, &req); err != nil {
			s.sendError(nil, ErrParseError, "Parse error", err.Error())
			continue
		}

		s.handleRequest(&req)
	}
}

// handleRequest routes JSON-RPC requests to domain-specific handlers.
// Method names follow namespace.action pattern (e.g., graph.get, nodes.create).
func (s *StdioServer) handleRequest(req *StdioRequest) {
	switch req.Method {
	// Health & Database
	case "health.check":
		s.handleHealthCheck(req)
	case "db.refresh":
		s.handleRefreshDatabase(req)

	// Indexing
	case "index.run":
		s.handleIndexRun(req)
	case "index.scan":
		s.handleIndexScan(req)
	case "index.scanFile":
		s.handleIndexScanFile(req)
	case "index.storeEdges":
		s.handleIndexStoreEdges(req)
	case "index.resolveSymbolsByLocation":
		s.handleResolveSymbolsByLocation(req)
	case "index.storeFileDelta":
		s.handleIndexStoreFileDelta(req)
	case "index.appendEdges":
		s.handleIndexAppendEdges(req)
	case "index.clearSymbols":
		s.handleIndexClearSymbols(req)

	// Validation
	case "validation.run":
		s.handleValidationRun(req)

	// Graph
	case "graph.get":
		s.handleGetGraph(req)
	case "graph.neighborhood":
		s.handleGetNeighborhood(req)
	case "graph.search":
		s.handleSearchNodes(req)
	case "graph.clearVirtual":
		s.handleClearVirtual(req)

	// File tree
	case "filetree.get":
		s.handleGetFileTree(req)

	// Nodes
	case "nodes.list":
		s.handleListNodes(req)
	case "nodes.get":
		s.handleGetNode(req)
	case "nodes.create":
		s.handleCreateNode(req)
	case "nodes.update":
		s.handleUpdateNode(req)
	case "nodes.delete":
		s.handleDeleteNode(req)

	// Edges
	case "edges.list":
		s.handleListEdges(req)
	case "edges.create":
		s.handleCreateEdge(req)
	case "edges.delete":
		s.handleDeleteEdge(req)

	// Annotations
	case "annotations.node.get":
		s.handleGetNodeAnnotation(req)
	case "annotations.node.update":
		s.handleUpdateNodeAnnotation(req)
	case "annotations.edge.get":
		s.handleGetEdgeAnnotation(req)
	case "annotations.edge.update":
		s.handleUpdateEdgeAnnotation(req)

	// Virtual containers
	case "virtual.directories.list":
		s.handleListVirtualDirectories(req)
	case "virtual.directories.create":
		s.handleCreateVirtualDirectory(req)
	case "virtual.directories.delete":
		s.handleDeleteVirtualDirectory(req)

	// Snapshots
	case "snapshots.list":
		s.handleListSnapshots(req)
	case "snapshots.create":
		s.handleCreateSnapshot(req)
	case "snapshots.get":
		s.handleGetSnapshot(req)
	case "snapshots.load":
		s.handleLoadSnapshot(req)
	case "snapshots.delete":
		s.handleDeleteSnapshot(req)
	case "snapshots.prompt.get":
		s.handleGetSnapshotPrompt(req)
	case "snapshots.prompt.update":
		s.handleUpdateSnapshotPrompt(req)
	case "snapshots.prompt.reset":
		s.handleResetSnapshotPrompt(req)

	default:
		s.sendError(req.ID, ErrMethodNotFound, "Method not found", req.Method)
	}
}

// sendResponse sends a success response.
func (s *StdioServer) sendResponse(id interface{}, result interface{}) {
	resp := StdioResponse{
		ID:     id,
		Result: result,
	}
	s.writeResponse(resp)
}

// sendError sends an error response.
func (s *StdioServer) sendError(id interface{}, code int, message string, data interface{}) {
	resp := StdioResponse{
		ID: id,
		Error: &StdioError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	s.writeResponse(resp)
}

// writeResponse writes a response to stdout (thread-safe).
func (s *StdioServer) writeResponse(resp StdioResponse) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	data, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fypd: failed to marshal response: %v\n", err)
		return
	}
	data = append(data, '\n')
	if _, err := s.writer.Write(data); err != nil {
		fmt.Fprintf(os.Stderr, "fypd: failed to write response: %v\n", err)
	}
}

// sendNotification sends a JSON-RPC notification (no id field).
// Used for progress updates during long-running operations.
func (s *StdioServer) sendNotification(method string, params interface{}) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	notif := map[string]interface{}{
		"method": method,
		"params": params,
	}

	data, err := json.Marshal(notif)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fypd: failed to marshal notification: %v\n", err)
		return
	}
	data = append(data, '\n')
	if _, err := s.writer.Write(data); err != nil {
		fmt.Fprintf(os.Stderr, "fypd: failed to write notification: %v\n", err)
	}
}

// Helper to get string param (handles string, float64, int for JSON compatibility)
func (s *StdioServer) getStringParam(req *StdioRequest, key string) string {
	if req.Params == nil {
		return ""
	}
	switch v := req.Params[key].(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	}
	return ""
}

// hasParam returns true if the key is present in req.Params (even if the value is empty).
func (s *StdioServer) hasParam(req *StdioRequest, key string) bool {
	if req.Params == nil {
		return false
	}
	_, exists := req.Params[key]
	return exists
}

// Helper to get int param
func (s *StdioServer) getIntParam(req *StdioRequest, key string, defaultVal int) int {
	if req.Params == nil {
		return defaultVal
	}
	switch v := req.Params[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}

// Helper to get bool param; returns (value, ok) — ok is false if key absent.
func (s *StdioServer) getBoolParam(req *StdioRequest, key string) (bool, bool) {
	if req.Params == nil {
		return false, false
	}
	v, exists := req.Params[key]
	if !exists {
		return false, false
	}
	switch val := v.(type) {
	case bool:
		return val, true
	case float64:
		return val != 0, true
	case int:
		return val != 0, true
	case string:
		return val == "true" || val == "1", true
	}
	return false, false
}

// ============================================================================
// Health & Database handlers
// ============================================================================

func (s *StdioServer) handleHealthCheck(req *StdioRequest) {
	s.sendResponse(req.ID, map[string]string{"status": "ok"})
}

// handleRefreshDatabase closes and reopens the database connection.
// This is called after indexing to ensure the server sees the latest data.
func (s *StdioServer) handleRefreshDatabase(req *StdioRequest) {
	fmt.Fprintln(os.Stderr, "fypd: Refreshing database connection...")

	// Close the current store
	if err := s.store.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "fypd: Warning: error closing old database: %v\n", err)
		// Continue anyway - we still want to open a new connection
	}

	// Open a new store
	newStore, err := db.NewStore(s.workspaceRoot)
	if err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to refresh database", err.Error())
		return
	}

	// Replace the store
	s.store = newStore

	fmt.Fprintln(os.Stderr, "fypd: Database connection refreshed successfully")
	s.sendResponse(req.ID, map[string]interface{}{
		"status":  "ok",
		"message": "Database connection refreshed",
	})
}
