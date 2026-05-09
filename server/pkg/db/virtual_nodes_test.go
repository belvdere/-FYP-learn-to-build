package db

import (
	"os"
	"testing"
	"time"

	"example.com/fyp/pkg/types"
)

// TestVirtualNodeCRUD tests basic Create, Read, Update, Delete operations
func TestVirtualNodeCRUD(t *testing.T) {
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
	node := &types.VirtualNode{
		ID:               "vnode_delete_account",
		Label:            "UserService.deleteAccount",
		Type:             "method",
		VirtualClassID:   "vnode_user_service_class",
		VirtualDirectory: "src/services",
		IsVirtual:        true,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	err = store.InsertVirtualNode(*node)
	if err != nil {
		t.Fatalf("InsertVirtualNode failed: %v", err)
	}

	retrieved, err := store.GetVirtualNode("vnode_delete_account")
	if err != nil {
		t.Fatalf("GetVirtualNode failed: %v", err)
	}

	if retrieved.ID != node.ID {
		t.Errorf("expected ID %s, got %s", node.ID, retrieved.ID)
	}
	if retrieved.Label != node.Label {
		t.Errorf("expected Label %s, got %s", node.Label, retrieved.Label)
	}
	if retrieved.Type != node.Type {
		t.Errorf("expected Type %s, got %s", node.Type, retrieved.Type)
	}
	if !retrieved.IsVirtual {
		t.Error("expected IsVirtual to be true")
	}
	if retrieved.VirtualClassID != node.VirtualClassID {
		t.Errorf("expected VirtualClassID %s, got %s", node.VirtualClassID, retrieved.VirtualClassID)
	}

	// Test Update
	retrieved.Type = "class"
	retrieved.Label = "UserService"
	retrieved.UpdatedAt = time.Now().Unix()

	err = store.UpdateVirtualNode(*retrieved)
	if err != nil {
		t.Fatalf("UpdateVirtualNode failed: %v", err)
	}

	updated, err := store.GetVirtualNode("vnode_delete_account")
	if err != nil {
		t.Fatalf("GetVirtualNode after update failed: %v", err)
	}

	if updated.Type != "class" {
		t.Errorf("expected Type 'class', got %s", updated.Type)
	}
	if updated.Label != "UserService" {
		t.Errorf("expected Label 'UserService', got %s", updated.Label)
	}

	// Test Delete
	err = store.DeleteVirtualNode("vnode_delete_account")
	if err != nil {
		t.Fatalf("DeleteVirtualNode failed: %v", err)
	}

	_, err = store.GetVirtualNode("vnode_delete_account")
	if err == nil {
		t.Error("expected error when getting deleted node, got nil")
	}
}

// TestVirtualNodesInClassHierarchy tests virtual nodes organized by virtual class
func TestVirtualNodesInClassHierarchy(t *testing.T) {
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
	vdir := &types.VirtualDirectory{
		ID:         "vdir_auth",
		Path:       "src/features/auth",
		Name:       "auth",
		ParentPath: "src/features",
		CreatedAt:  now,
	}
	err = store.InsertVirtualDirectory(*vdir)
	if err != nil {
		t.Fatalf("InsertVirtualDirectory failed: %v", err)
	}

	// Create virtual class node
	classNode := types.VirtualNode{
		ID:               "vnode_auth_service_class",
		Label:            "AuthService",
		Type:             "class",
		VirtualDirectory: "src/features/auth",
		IsVirtual:        true,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	err = store.InsertVirtualNode(classNode)
	if err != nil {
		t.Fatalf("InsertVirtualNode for class failed: %v", err)
	}

	// Create virtual method nodes under the class
	methods := []types.VirtualNode{
		{
			ID:               "vnode_login",
			Label:            "AuthService.login",
			Type:             "method",
			VirtualClassID:   "vnode_auth_service_class",
			VirtualDirectory: "src/features/auth",
			IsVirtual:        true,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		{
			ID:               "vnode_logout",
			Label:            "AuthService.logout",
			Type:             "method",
			VirtualClassID:   "vnode_auth_service_class",
			VirtualDirectory: "src/features/auth",
			IsVirtual:        true,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		{
			ID:               "vnode_refresh_token",
			Label:            "AuthService.refreshToken",
			Type:             "method",
			VirtualClassID:   "vnode_auth_service_class",
			VirtualDirectory: "src/features/auth",
			IsVirtual:        true,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}

	for _, method := range methods {
		err = store.InsertVirtualNode(method)
		if err != nil {
			t.Fatalf("InsertVirtualNode failed for %s: %v", method.Label, err)
		}
	}

	// Query nodes by virtual directory (includes both class and methods)
	nodesByDir, err := store.GetVirtualNodesByDirectory("src/features/auth")
	if err != nil {
		t.Fatalf("GetVirtualNodesByDirectory failed: %v", err)
	}

	if len(nodesByDir) != 4 { // 1 class + 3 methods
		t.Errorf("expected 4 nodes in directory, got %d", len(nodesByDir))
	}

	// Verify methods have virtualClassId set
	methodCount := 0
	for _, node := range nodesByDir {
		if node.Type == "method" {
			methodCount++
			if node.VirtualClassID != "vnode_auth_service_class" {
				t.Errorf("node %s has wrong VirtualClassID: %s", node.ID, node.VirtualClassID)
			}
		}
	}
	if methodCount != 3 {
		t.Errorf("expected 3 method nodes, got %d", methodCount)
	}
}

// TestGetAllVirtualNodes tests retrieving all virtual nodes
func TestGetAllVirtualNodes(t *testing.T) {
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

	nodes := []types.VirtualNode{
		{ID: "vnode_1", Label: "ServiceA.method1", Type: "method", IsVirtual: true, CreatedAt: now, UpdatedAt: now},
		{ID: "vnode_2", Label: "ServiceB.method2", Type: "method", IsVirtual: true, CreatedAt: now, UpdatedAt: now},
		{ID: "vnode_3", Label: "ConceptC", Type: "concept", IsVirtual: true, CreatedAt: now, UpdatedAt: now},
	}

	for _, node := range nodes {
		err = store.InsertVirtualNode(node)
		if err != nil {
			t.Fatalf("InsertVirtualNode failed: %v", err)
		}
	}

	allNodes, err := store.GetAllVirtualNodes()
	if err != nil {
		t.Fatalf("GetAllVirtualNodes failed: %v", err)
	}

	if len(allNodes) != 3 {
		t.Errorf("expected 3 virtual nodes, got %d", len(allNodes))
	}

	for _, node := range allNodes {
		if !node.IsVirtual {
			t.Errorf("node %s should be virtual", node.ID)
		}
	}
}

// TestVirtualNodeNonExistent tests error handling for non-existent nodes
func TestVirtualNodeNonExistent(t *testing.T) {
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

	_, err = store.GetVirtualNode("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent node, got nil")
	}

	node := &types.VirtualNode{
		ID:        "nonexistent",
		Label:     "Test",
		Type:      "method",
		IsVirtual: true,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}
	err = store.UpdateVirtualNode(*node)
	if err == nil {
		t.Error("expected error when updating non-existent node, got nil")
	}

	err = store.DeleteVirtualNode("nonexistent")
	if err != nil {
		t.Errorf("DeleteVirtualNode should be idempotent, got error: %v", err)
	}
}
