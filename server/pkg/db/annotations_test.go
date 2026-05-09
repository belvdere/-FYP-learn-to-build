package db

import (
	"os"
	"testing"
	"time"

	"example.com/fyp/pkg/types"
)

// TestNodeAnnotationWorkflow tests the complete workflow of annotating a node
// Realistic Use Case: User annotates nodes with AI instructions
func TestNodeAnnotationWorkflow(t *testing.T) {
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

	// First, create a real symbol (simulating existing code)
	symbol := types.Symbol{
		ID:        "UserService.createUser",
		Name:      "createUser",
		Kind:      "method",
		FilePath:  "src/services/UserService.java",
		Line:      45,
		Signature: "public User createUser(String email, String password)",
	}
	err = store.InsertSymbol(symbol)
	if err != nil {
		t.Fatalf("InsertSymbol failed: %v", err)
	}

	// Add annotation with description and AI remarks
	annotation := &types.NodeAnnotation{
		NodeID:      "UserService.createUser",
		Description: "Creates a new user account with email validation",
		AIRemarks:   "Must check email uniqueness before inserting. Hash password with bcrypt. Return error if email exists.",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	err = store.UpsertNodeAnnotation(*annotation)
	if err != nil {
		t.Fatalf("UpsertNodeAnnotation failed: %v", err)
	}

	// Retrieve annotation
	retrieved, err := store.GetNodeAnnotation("UserService.createUser")
	if err != nil {
		t.Fatalf("GetNodeAnnotation failed: %v", err)
	}

	if retrieved.NodeID != annotation.NodeID {
		t.Errorf("expected NodeID %s, got %s", annotation.NodeID, retrieved.NodeID)
	}
	if retrieved.Description != annotation.Description {
		t.Errorf("expected Description %s, got %s", annotation.Description, retrieved.Description)
	}
	if retrieved.AIRemarks != annotation.AIRemarks {
		t.Errorf("expected AIRemarks %s, got %s", annotation.AIRemarks, retrieved.AIRemarks)
	}

	// Update annotation with code snippet
	retrieved.CodeSnippet = `public User createUser(String email, String password) {
	if (userRepository.existsByEmail(email)) {
		throw new EmailExistsException();
	}
	String hashedPassword = bcrypt.hash(password);
	return userRepository.save(new User(email, hashedPassword));
}`
	retrieved.CustomProperties = `{"critical": true, "reviewed": false}`
	retrieved.UpdatedAt = time.Now().Unix()

	err = store.UpsertNodeAnnotation(*retrieved)
	if err != nil {
		t.Fatalf("UpsertNodeAnnotation update failed: %v", err)
	}

	// Verify update
	updated, err := store.GetNodeAnnotation("UserService.createUser")
	if err != nil {
		t.Fatalf("GetNodeAnnotation after update failed: %v", err)
	}

	if updated.CodeSnippet == "" {
		t.Error("expected CodeSnippet to be set")
	}
	if updated.CustomProperties != `{"critical": true, "reviewed": false}` {
		t.Errorf("expected CustomProperties, got %s", updated.CustomProperties)
	}

	// Delete annotation
	err = store.DeleteNodeAnnotation("UserService.createUser")
	if err != nil {
		t.Fatalf("DeleteNodeAnnotation failed: %v", err)
	}

	// Verify deletion
	annotation, err = store.GetNodeAnnotation("UserService.createUser")
	if err != nil {
		t.Fatalf("GetNodeAnnotation failed: %v", err)
	}
	if annotation != nil {
		t.Error("expected nil annotation after deletion, got non-nil")
	}
}

// TestEdgeAnnotationWorkflow tests the complete workflow of annotating an edge
// Realistic Use Case: User annotates relationships between nodes
func TestEdgeAnnotationWorkflow(t *testing.T) {
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

	// Create two symbols (nodes)
	symbols := []types.Symbol{
		{
			ID:       "UserService.createUser",
			Name:     "createUser",
			Kind:     "method",
			FilePath: "src/services/UserService.java",
			Line:     45,
		},
		{
			ID:       "UserRepository.save",
			Name:     "save",
			Kind:     "method",
			FilePath: "src/repositories/UserRepository.java",
			Line:     20,
		},
	}

	for _, sym := range symbols {
		err = store.InsertSymbol(sym)
		if err != nil {
			t.Fatalf("InsertSymbol failed: %v", err)
		}
	}

	// Add edge annotation with remarks
	edgeID := "UserRepository.save_UserService.createUser" // Alphabetically sorted
	annotation := &types.EdgeAnnotation{
		EdgeID:       edgeID,
		SourceNodeID: "UserService.createUser",
		TargetNodeID: "UserRepository.save",
		Remarks:      "Critical path - handle errors and ensure transaction rollback",
		CallCount:    1,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	err = store.UpsertEdgeAnnotation(*annotation)
	if err != nil {
		t.Fatalf("UpsertEdgeAnnotation failed: %v", err)
	}

	// Retrieve by edge ID
	retrieved, err := store.GetEdgeAnnotation(edgeID)
	if err != nil {
		t.Fatalf("GetEdgeAnnotation failed: %v", err)
	}

	if retrieved.EdgeID != edgeID {
		t.Errorf("expected EdgeID %s, got %s", edgeID, retrieved.EdgeID)
	}
	if retrieved.Remarks != annotation.Remarks {
		t.Errorf("expected Remarks %s, got %s", annotation.Remarks, retrieved.Remarks)
	}
	if retrieved.CallCount != 1 {
		t.Errorf("expected CallCount 1, got %d", retrieved.CallCount)
	}

	// Retrieve by node IDs
	byNodes, err := store.GetEdgeAnnotationByNodes("UserService.createUser", "UserRepository.save")
	if err != nil {
		t.Fatalf("GetEdgeAnnotationByNodes failed: %v", err)
	}

	if byNodes.EdgeID != edgeID {
		t.Errorf("expected EdgeID %s, got %s", edgeID, byNodes.EdgeID)
	}

	// Update call count
	retrieved.CallCount = 5
	retrieved.Remarks = "Critical path - handle errors and ensure transaction rollback. Called 5 times in workflow."
	retrieved.UpdatedAt = time.Now().Unix()

	err = store.UpsertEdgeAnnotation(*retrieved)
	if err != nil {
		t.Fatalf("UpsertEdgeAnnotation update failed: %v", err)
	}

	// Verify update
	updated, err := store.GetEdgeAnnotation(edgeID)
	if err != nil {
		t.Fatalf("GetEdgeAnnotation after update failed: %v", err)
	}

	if updated.CallCount != 5 {
		t.Errorf("expected CallCount 5, got %d", updated.CallCount)
	}

	// Delete annotation
	err = store.DeleteEdgeAnnotation(edgeID)
	if err != nil {
		t.Fatalf("DeleteEdgeAnnotation failed: %v", err)
	}

	// Verify deletion
	annotation, err = store.GetEdgeAnnotation(edgeID)
	if err != nil {
		t.Fatalf("GetEdgeAnnotation failed: %v", err)
	}
	if annotation != nil {
		t.Error("expected nil annotation after deletion, got non-nil")
	}
}

// TestAnnotationForVirtualNode tests annotating a virtual node
// Realistic Use Case: User annotates planned features with AI context
func TestAnnotationForVirtualNode(t *testing.T) {
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

	// Create a virtual node
	vnode := &types.VirtualNode{
		ID:        "vnode_auth_login",
		Label:     "AuthService.login",
		Type:      "method",
		IsVirtual: true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	err = store.InsertVirtualNode(*vnode)
	if err != nil {
		t.Fatalf("InsertVirtualNode failed: %v", err)
	}

	// Annotate the virtual node
	annotation := &types.NodeAnnotation{
		NodeID:      "vnode_auth_login",
		Description: "Authenticates user with email and password",
		AIRemarks:   "Validate email format. Check password against hashed value in database. Return JWT token on success.",
		CodeSnippet: `public LoginResponse login(String email, String password) {
	// Validate email
	// Check credentials
	// Generate JWT
	return new LoginResponse(token);
}`,
		CreatedAt: now,
		UpdatedAt: now,
	}

	err = store.UpsertNodeAnnotation(*annotation)
	if err != nil {
		t.Fatalf("UpsertNodeAnnotation failed: %v", err)
	}

	// Retrieve and verify
	retrieved, err := store.GetNodeAnnotation("vnode_auth_login")
	if err != nil {
		t.Fatalf("GetNodeAnnotation failed: %v", err)
	}

	if retrieved.NodeID != "vnode_auth_login" {
		t.Errorf("expected NodeID vnode_auth_login, got %s", retrieved.NodeID)
	}
	if retrieved.CodeSnippet == "" {
		t.Error("expected CodeSnippet to be set")
	}
}

// TestGetAllAnnotations tests retrieving all annotations
func TestGetAllAnnotations(t *testing.T) {
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

	// Create multiple node annotations
	nodeAnnotations := []types.NodeAnnotation{
		{
			NodeID:      "node1",
			Description: "Node 1 description",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			NodeID:      "node2",
			Description: "Node 2 description",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			NodeID:      "node3",
			Description: "Node 3 description",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}

	for _, ann := range nodeAnnotations {
		err = store.UpsertNodeAnnotation(ann)
		if err != nil {
			t.Fatalf("UpsertNodeAnnotation failed: %v", err)
		}
	}

	// Create multiple edge annotations
	edgeAnnotations := []types.EdgeAnnotation{
		{
			EdgeID:       "edge1",
			SourceNodeID: "node1",
			TargetNodeID: "node2",
			Remarks:      "Edge 1 remarks",
			CallCount:    1,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			EdgeID:       "edge2",
			SourceNodeID: "node2",
			TargetNodeID: "node3",
			Remarks:      "Edge 2 remarks",
			CallCount:    2,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}

	for _, ann := range edgeAnnotations {
		err = store.UpsertEdgeAnnotation(ann)
		if err != nil {
			t.Fatalf("UpsertEdgeAnnotation failed: %v", err)
		}
	}

	// Get all node annotations
	allNodeAnns, err := store.GetAllNodeAnnotations()
	if err != nil {
		t.Fatalf("GetAllNodeAnnotations failed: %v", err)
	}

	if len(allNodeAnns) != 3 {
		t.Errorf("expected 3 node annotations, got %d", len(allNodeAnns))
	}

	// Get all edge annotations
	allEdgeAnns, err := store.GetAllEdgeAnnotations()
	if err != nil {
		t.Fatalf("GetAllEdgeAnnotations failed: %v", err)
	}

	if len(allEdgeAnns) != 2 {
		t.Errorf("expected 2 edge annotations, got %d", len(allEdgeAnns))
	}
}

// TestUpsertAnnotationIdempotency tests that upsert can insert and update
func TestUpsertAnnotationIdempotency(t *testing.T) {
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

	// First upsert (insert)
	annotation := &types.NodeAnnotation{
		NodeID:      "test_node",
		Description: "Original description",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	err = store.UpsertNodeAnnotation(*annotation)
	if err != nil {
		t.Fatalf("UpsertNodeAnnotation (insert) failed: %v", err)
	}

	// Second upsert (update)
	annotation.Description = "Updated description"
	annotation.AIRemarks = "New AI remarks"
	annotation.UpdatedAt = time.Now().Unix()

	err = store.UpsertNodeAnnotation(*annotation)
	if err != nil {
		t.Fatalf("UpsertNodeAnnotation (update) failed: %v", err)
	}

	// Verify final state
	final, err := store.GetNodeAnnotation("test_node")
	if err != nil {
		t.Fatalf("GetNodeAnnotation failed: %v", err)
	}

	if final.Description != "Updated description" {
		t.Errorf("expected updated description, got %s", final.Description)
	}
	if final.AIRemarks != "New AI remarks" {
		t.Errorf("expected AI remarks, got %s", final.AIRemarks)
	}
}

// TestEdgeAnnotationNonExistent tests error handling for non-existent edges
func TestEdgeAnnotationNonExistent(t *testing.T) {
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

	// Try to get non-existent edge annotation
	annotation, err := store.GetEdgeAnnotation("nonexistent_edge")
	if err != nil {
		t.Fatalf("GetEdgeAnnotation failed: %v", err)
	}
	if annotation != nil {
		t.Error("expected nil for non-existent edge annotation, got non-nil")
	}

	// Try to get by non-existent node IDs
	annotation, err = store.GetEdgeAnnotationByNodes("node_a", "node_b")
	if err != nil {
		t.Fatalf("GetEdgeAnnotationByNodes failed: %v", err)
	}
	if annotation != nil {
		t.Error("expected nil for non-existent edge annotation by nodes, got non-nil")
	}
}
