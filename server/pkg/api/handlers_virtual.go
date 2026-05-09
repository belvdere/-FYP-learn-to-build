package api

import (
	"example.com/fyp/pkg/types"
	"github.com/google/uuid"
)

// ============================================================================
// Virtual directory handlers
// ============================================================================

func (s *StdioServer) handleListVirtualDirectories(req *StdioRequest) {
	dirs, err := s.store.GetAllVirtualDirectories()
	if err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to fetch directories", err.Error())
		return
	}
	s.sendResponse(req.ID, dirs)
}

func (s *StdioServer) handleCreateVirtualDirectory(req *StdioRequest) {
	path := s.getStringParam(req, "path")
	name := s.getStringParam(req, "name")
	parentPath := s.getStringParam(req, "parentPath")

	if path == "" || name == "" {
		s.sendError(req.ID, ErrInvalidParams, "Path and name are required", nil)
		return
	}

	dir := types.VirtualDirectory{
		ID:         uuid.New().String(),
		Path:       path,
		Name:       name,
		ParentPath: parentPath,
	}

	if err := s.store.InsertVirtualDirectory(dir); err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to create directory", err.Error())
		return
	}

	s.sendResponse(req.ID, dir)
}

func (s *StdioServer) handleDeleteVirtualDirectory(req *StdioRequest) {
	dirID := s.getStringParam(req, "id")
	if dirID == "" {
		s.sendError(req.ID, ErrInvalidParams, "Missing id parameter", nil)
		return
	}

	if err := s.store.DeleteVirtualDirectory(dirID); err != nil {
		s.sendError(req.ID, ErrInternalError, "Failed to delete directory", err.Error())
		return
	}

	s.sendResponse(req.ID, map[string]string{"message": "Directory deleted successfully", "id": dirID})
}
