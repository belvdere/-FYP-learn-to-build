package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"example.com/fyp/pkg/config"
	"example.com/fyp/pkg/db"
	"example.com/fyp/pkg/lang"
)

// Server represents an MCP server
// Provides kg.maskCode and kg.getSnapshot tools for Copilot integration
type Server struct {
	store          *db.Store
	langController *lang.LanguageController
	reader         *bufio.Reader
	writer         io.Writer
	logFile        *os.File
	logMutex       sync.Mutex
	workspaceRoot  string
}

// NewServer creates a new MCP server
func NewServer(workspaceRoot string) (*Server, error) {
	// Initialize database
	store, err := db.NewStore(workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("init store: %w", err)
	}

	// Initialize language controller with all registered plugins
	langController := lang.NewLanguageControllerWithDefaults()

	// Create .fyp directory if it doesn't exist
	fypDir := filepath.Join(workspaceRoot, ".fyp")
	if err := config.EnsureDir(fypDir); err != nil {
		store.Close()
		return nil, fmt.Errorf("create .fyp directory: %w", err)
	}

	// Load .fyp/.env into process (FYP_OPTIMIZE_PROMPTS, OPENAI_API_KEY, etc.)
	loadFypEnvIntoProcess(workspaceRoot)

	// Open log file (append mode)
	logPath := filepath.Join(fypDir, "mcp-server.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		// Log to stderr but don't fail server startup
		fmt.Fprintf(os.Stderr, "fypd: warning: failed to open log file %s: %v (logging to stderr only)\n", logPath, err)
		logFile = nil
	}

	server := &Server{
		store:          store,
		langController: langController,
		reader:         bufio.NewReader(os.Stdin),
		writer:         os.Stdout,
		logFile:        logFile,
		workspaceRoot:  workspaceRoot,
	}

	if logFile != nil {
		fmt.Fprintf(os.Stderr, "fypd: MCP server logs will be written to %s\n", logPath)
	}

	return server, nil
}

// Close releases server resources
func (s *Server) Close() error {
	if s.logFile != nil {
		s.logFile.Close()
	}
	return s.store.Close()
}

// logFeatureUsage logs feature usage with timestamp and details
// Logs are written to both stderr (for immediate visibility) and log file (for persistence)
func (s *Server) logFeatureUsage(feature string, status string, details map[string]interface{}) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	logMsg := fmt.Sprintf("[%s] [MCP] [%s] [%s]", timestamp, feature, status)

	if len(details) > 0 {
		detailsStr := ""
		for k, v := range details {
			if detailsStr != "" {
				detailsStr += ", "
			}
			detailsStr += fmt.Sprintf("%s=%v", k, v)
		}
		logMsg += " " + detailsStr
	}

	// Always write to stderr for immediate visibility
	fmt.Fprintln(os.Stderr, logMsg)

	// Also write to log file if available
	s.logMutex.Lock()
	defer s.logMutex.Unlock()
	if s.logFile != nil {
		// Write to file (ignore errors to avoid disrupting the server)
		fmt.Fprintln(s.logFile, logMsg)
		// Flush to ensure logs are written immediately
		s.logFile.Sync()
	}
}

// Serve starts the MCP server loop
func (s *Server) Serve() error {
	fmt.Fprintln(os.Stderr, "fypd: MCP server ready")

	for {
		// Read a line
		line, err := s.reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read request: %w", err)
		}

		// Parse request
		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			s.sendError(nil, ParseError, "Parse error", err.Error())
			continue
		}

		// Handle request
		s.handleRequest(&req)
	}
}

// handleRequest routes requests to appropriate handlers
func (s *Server) handleRequest(req *Request) {
	switch req.Method {
	case MethodInitialize:
		s.handleInitialize(req)
	case MethodResourcesList:
		s.handleResourcesList(req)
	case MethodResourcesRead:
		s.handleResourcesRead(req)
	case MethodToolsList:
		s.handleToolsList(req)
	case MethodToolsCall:
		s.handleToolsCall(req)
	default:
		s.logFeatureUsage("unknown_method", "error", map[string]interface{}{
			"method": req.Method,
		})
		s.sendError(req.ID, MethodNotFound, "Method not found", req.Method)
	}
}

// handleInitialize handles initialization
func (s *Server) handleInitialize(req *Request) {
	s.logFeatureUsage("initialize", "called", nil)

	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: ServerCapabilities{
			Resources: map[string]interface{}{
				"listChanged": true,
			},
			Tools: map[string]interface{}{
				"listChanged": true,
			},
		},
		ServerInfo: ServerInfo{
			Name:    "fypd",
			Version: "0.2.0", // Updated version for simplified server
		},
	}

	s.sendResponse(req.ID, result)
	s.logFeatureUsage("initialize", "success", nil)
}

// sendResponse sends a success response
func (s *Server) sendResponse(id interface{}, result interface{}) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}

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

// sendError sends an error response
func (s *Server) sendError(id interface{}, code int, message string, data interface{}) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &Error{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}

	jsonData, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fypd: failed to marshal error: %v\n", err)
		return
	}

	jsonData = append(jsonData, '\n')
	if _, err := s.writer.Write(jsonData); err != nil {
		fmt.Fprintf(os.Stderr, "fypd: failed to write error: %v\n", err)
	}
}
