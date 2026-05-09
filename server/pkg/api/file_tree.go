package api

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"example.com/fyp/pkg/db"
	"example.com/fyp/pkg/types"
	"github.com/google/uuid"
)

// FileTreeBuilder provides file tree building functionality.
type FileTreeBuilder struct {
	store         *db.Store
	workspaceRoot string
	sourceExts    map[string]struct{}
}

// NewFileTreeBuilder creates a new FileTreeBuilder.
func NewFileTreeBuilder(store *db.Store, workspaceRoot string, sourceExtensions []string) *FileTreeBuilder {
	exts := make(map[string]struct{}, len(sourceExtensions))
	for _, ext := range sourceExtensions {
		normalized := strings.ToLower(strings.TrimSpace(ext))
		if normalized == "" {
			continue
		}
		exts[normalized] = struct{}{}
	}
	return &FileTreeBuilder{
		store:         store,
		workspaceRoot: workspaceRoot,
		sourceExts:    exts,
	}
}

// BuildFileTree builds the file tree structure from workspace and virtual containers.
func (b *FileTreeBuilder) BuildFileTree() (*FileTreeResponse, error) {
	// Get real directories and files
	realDirs, realFiles, err := b.scanWorkspace()
	if err != nil {
		return nil, err
	}

	// Get virtual directories
	virtualDirs, err := b.store.GetAllVirtualDirectories()
	if err != nil {
		return nil, fmt.Errorf("failed to get virtual directories: %w", err)
	}

	// Build directory tree
	dirTreeNodes := make([]DirectoryTreeNode, 0)
	for _, dir := range realDirs {
		dirTreeNodes = append(dirTreeNodes, DirectoryTreeNode{
			ID:         dir.ID,
			Path:       dir.Path,
			Name:       dir.Name,
			IsVirtual:  false,
			IsExpanded: false,
		})
	}

	for _, dir := range virtualDirs {
		dirTreeNodes = append(dirTreeNodes, DirectoryTreeNode{
			ID:         dir.ID,
			Path:       dir.Path,
			Name:       dir.Name,
			IsVirtual:  true,
			IsExpanded: false,
		})
	}

	// Build file tree nodes with methods (real files only)
	fileTreeNodes := make([]FileTreeNode, 0)
	for _, file := range realFiles {
		methods, _ := b.store.GetGraphNodesByFile(file.Path)

		methodNodes := make([]MethodTreeNode, 0)
		for _, method := range methods {
			var annotation *NodeAnnotationResponse
			if method.Annotation != nil {
				annotation = &NodeAnnotationResponse{
					Description:      method.Annotation.Description,
					AIRemarks:        method.Annotation.AIRemarks,
					CodeSnippet:      method.Annotation.CodeSnippet,
					CustomProperties: method.Annotation.CustomProperties,
				}
			}

			methodNodes = append(methodNodes, MethodTreeNode{
				ID:         method.ID,
				Name:       method.Label,
				Type:       method.Type,
				Line:       method.Line,
				IsVirtual:  false,
				Annotation: annotation,
			})
		}

		fileTreeNodes = append(fileTreeNodes, FileTreeNode{
			ID:        file.ID,
			Path:      file.Path,
			Name:      file.Name,
			IsVirtual: false,
			Methods:   methodNodes,
		})
	}

	return &FileTreeResponse{
		Directories: dirTreeNodes,
		Files:       fileTreeNodes,
	}, nil
}

// scanWorkspace scans the workspace for real directories and files.
func (b *FileTreeBuilder) scanWorkspace() ([]DirectoryResponse, []FileNodeResponse, error) {
	directories := make([]DirectoryResponse, 0)
	files := make([]FileNodeResponse, 0)

	// Walk workspace directory
	err := filepath.Walk(b.workspaceRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories and .fyp directory
		if info.IsDir() && (strings.HasPrefix(info.Name(), ".") || info.Name() == ".fyp") {
			return filepath.SkipDir
		}

		// Get relative path
		relPath, err := filepath.Rel(b.workspaceRoot, path)
		if err != nil {
			return err
		}

		if info.IsDir() && relPath != "." {
			directories = append(directories, DirectoryResponse{
				ID:        hashString(relPath), // Stable hash-based ID
				Path:      relPath,
				Name:      info.Name(),
				IsVirtual: false,
			})
		} else if !info.IsDir() && b.isSupportedSourceFile(info.Name()) {
			dirPath := filepath.Dir(relPath)
			files = append(files, FileNodeResponse{
				ID:        hashString(relPath), // Stable hash-based ID
				Path:      relPath,
				Name:      info.Name(),
				Directory: hashString(dirPath), // Use hash for directory reference too
				IsVirtual: false,
			})
		}

		return nil
	})

	if err != nil {
		return nil, nil, fmt.Errorf("failed to scan workspace: %w", err)
	}

	return directories, files, nil
}

func (b *FileTreeBuilder) isSupportedSourceFile(fileName string) bool {
	ext := strings.ToLower(filepath.Ext(fileName))
	if ext == "" {
		return false
	}
	_, ok := b.sourceExts[ext]
	return ok
}

// CreateVirtualDirectory creates a virtual directory.
func (b *FileTreeBuilder) CreateVirtualDirectory(path, name, parentPath string) (*types.VirtualDirectory, error) {
	dir := types.VirtualDirectory{
		ID:         uuid.New().String(),
		Path:       path,
		Name:       name,
		ParentPath: parentPath,
	}

	if err := b.store.InsertVirtualDirectory(dir); err != nil {
		return nil, err
	}

	return &dir, nil
}
