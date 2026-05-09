package db

import (
	"os"
	"testing"
	"time"

	"example.com/fyp/pkg/types"
)

// TestVirtualDirectoryCRUD tests basic directory operations
func TestVirtualDirectoryCRUD(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fyp-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	now := time.Now().Unix()

	// Create virtual directory
	dir := &types.VirtualDirectory{
		ID:         "vdir_test",
		Path:       "src/test",
		Name:       "test",
		ParentPath: "src",
		CreatedAt:  now,
	}

	err = store.InsertVirtualDirectory(*dir)
	if err != nil {
		t.Fatalf("InsertVirtualDirectory failed: %v", err)
	}

	// Retrieve by ID
	retrieved, err := store.GetVirtualDirectory("vdir_test")
	if err != nil {
		t.Fatalf("GetVirtualDirectory failed: %v", err)
	}

	if retrieved.ID != dir.ID {
		t.Errorf("expected ID %s, got %s", dir.ID, retrieved.ID)
	}
	if retrieved.Path != dir.Path {
		t.Errorf("expected Path %s, got %s", dir.Path, retrieved.Path)
	}

	// Retrieve by path
	byPath, err := store.GetVirtualDirectoryByPath("src/test")
	if err != nil {
		t.Fatalf("GetVirtualDirectoryByPath failed: %v", err)
	}

	if byPath.ID != dir.ID {
		t.Errorf("expected ID %s, got %s", dir.ID, byPath.ID)
	}

	// Delete directory
	err = store.DeleteVirtualDirectory("vdir_test")
	if err != nil {
		t.Fatalf("DeleteVirtualDirectory failed: %v", err)
	}

	// Verify deletion
	_, err = store.GetVirtualDirectory("vdir_test")
	if err == nil {
		t.Error("expected error when getting deleted directory, got nil")
	}
}

// TestGetAllVirtualDirectories tests retrieving all directories
func TestGetAllVirtualDirectories(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fyp-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	now := time.Now().Unix()

	dirs := []types.VirtualDirectory{
		{ID: "vdir_1", Path: "src/dir1", Name: "dir1", CreatedAt: now},
		{ID: "vdir_2", Path: "src/dir2", Name: "dir2", CreatedAt: now},
		{ID: "vdir_3", Path: "src/dir3", Name: "dir3", CreatedAt: now},
	}

	for _, dir := range dirs {
		err = store.InsertVirtualDirectory(dir)
		if err != nil {
			t.Fatalf("InsertVirtualDirectory failed: %v", err)
		}
	}

	allDirs, err := store.GetAllVirtualDirectories()
	if err != nil {
		t.Fatalf("GetAllVirtualDirectories failed: %v", err)
	}

	if len(allDirs) != 3 {
		t.Errorf("expected 3 directories, got %d", len(allDirs))
	}
}

// TestNestedDirectoryStructure tests complex nested hierarchies
func TestNestedDirectoryStructure(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fyp-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	now := time.Now().Unix()

	dirs := []types.VirtualDirectory{
		{ID: "vdir_src", Path: "src", Name: "src", ParentPath: "", CreatedAt: now},
		{ID: "vdir_features", Path: "src/features", Name: "features", ParentPath: "src", CreatedAt: now},
		{ID: "vdir_auth", Path: "src/features/auth", Name: "auth", ParentPath: "src/features", CreatedAt: now},
		{ID: "vdir_payment", Path: "src/features/payment", Name: "payment", ParentPath: "src/features", CreatedAt: now},
		{ID: "vdir_utils", Path: "src/utils", Name: "utils", ParentPath: "src", CreatedAt: now},
	}

	for _, dir := range dirs {
		err = store.InsertVirtualDirectory(dir)
		if err != nil {
			t.Fatalf("InsertVirtualDirectory failed for %s: %v", dir.Path, err)
		}
	}

	srcChildren, err := store.GetVirtualDirectoriesByParent("src")
	if err != nil {
		t.Fatalf("GetVirtualDirectoriesByParent(src) failed: %v", err)
	}

	if len(srcChildren) != 2 {
		t.Errorf("expected 2 children of src, got %d", len(srcChildren))
	}

	featuresChildren, err := store.GetVirtualDirectoriesByParent("src/features")
	if err != nil {
		t.Fatalf("GetVirtualDirectoriesByParent(src/features) failed: %v", err)
	}

	if len(featuresChildren) != 2 {
		t.Errorf("expected 2 children of src/features, got %d", len(featuresChildren))
	}

	authChildren, err := store.GetVirtualDirectoriesByParent("src/features/auth")
	if err != nil {
		t.Fatalf("GetVirtualDirectoriesByParent(src/features/auth) failed: %v", err)
	}

	if len(authChildren) != 0 {
		t.Errorf("expected 0 children of src/features/auth, got %d", len(authChildren))
	}
}

// TestVirtualDirectoryNonExistent tests error handling for non-existent directories
func TestVirtualDirectoryNonExistent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fyp-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	_, err = store.GetVirtualDirectory("nonexistent_dir")
	if err == nil {
		t.Error("expected error for non-existent directory, got nil")
	}

	_, err = store.GetVirtualDirectoryByPath("nonexistent/path")
	if err == nil {
		t.Error("expected error for non-existent directory path, got nil")
	}

	err = store.DeleteVirtualDirectory("nonexistent_dir")
	if err != nil {
		t.Errorf("DeleteVirtualDirectory should be idempotent, got error: %v", err)
	}
}

// TestDuplicateVirtualDirectoryPath tests unique constraint on directory paths
func TestDuplicateVirtualDirectoryPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fyp-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	now := time.Now().Unix()

	dir1 := &types.VirtualDirectory{
		ID:        "vdir_1",
		Path:      "src/test",
		Name:      "test",
		CreatedAt: now,
	}

	err = store.InsertVirtualDirectory(*dir1)
	if err != nil {
		t.Fatalf("InsertVirtualDirectory for first dir failed: %v", err)
	}

	dir2 := &types.VirtualDirectory{
		ID:        "vdir_2",
		Path:      "src/test", // Duplicate path
		Name:      "test",
		CreatedAt: now,
	}

	err = store.InsertVirtualDirectory(*dir2)
	if err == nil {
		t.Error("expected error when inserting duplicate directory path, got nil")
	}
}
