package db

import (
	"os"
	"testing"
	"time"

	"example.com/fyp/pkg/types"
)

// TestUnifiedGraphNodes tests combining real symbols and virtual nodes
// Realistic Use Case: User views unified graph showing both indexed code and planned features
func TestUnifiedGraphNodes(t *testing.T) {
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

	// Insert 3 real symbols
	realSymbols := []types.Symbol{
		{
			ID:       "UserService",
			Name:     "UserService",
			Kind:     "class",
			FilePath: "src/UserService.java",
			Line:     1,
		},
		{
			ID:       "UserService.createUser",
			Name:     "createUser",
			Kind:     "method",
			FilePath: "src/UserService.java",
			Line:     10,
		},
		{
			ID:       "UserService.deleteUser",
			Name:     "deleteUser",
			Kind:     "method",
			FilePath: "src/UserService.java",
			Line:     20,
		},
	}

	for _, sym := range realSymbols {
		err = store.InsertSymbol(sym)
		if err != nil {
			t.Fatalf("InsertSymbol failed: %v", err)
		}
	}

	// Insert 2 virtual nodes
	virtualNodes := []types.VirtualNode{
		{
			ID:        "vnode_planned_feature",
			Label:     "PlannedFeature.process",
			Type:      "method",
			IsVirtual: true,

			UpdatedAt: now,
		},
		{
			ID:        "vnode_planned_helper",
			Label:     "PlannedHelper.validate",
			Type:      "method",
			IsVirtual: true,

			UpdatedAt: now,
		},
	}

	for _, vn := range virtualNodes {
		err = store.InsertVirtualNode(vn)
		if err != nil {
			t.Fatalf("InsertVirtualNode failed: %v", err)
		}
	}

	// Get all graph nodes
	allNodes, err := store.GetAllGraphNodes()
	if err != nil {
		t.Fatalf("GetAllGraphNodes failed: %v", err)
	}

	if len(allNodes) != 5 {
		t.Fatalf("expected 5 nodes total, got %d", len(allNodes))
	}

	// Verify real nodes have isVirtual=false and line numbers
	realCount := 0
	virtualCount := 0
	for _, node := range allNodes {
		if node.IsVirtual {
			virtualCount++
			// Virtual nodes should not have line numbers
			if node.Line != 0 {
				t.Errorf("virtual node %s should have line=0, got %d", node.ID, node.Line)
			}
		} else {
			realCount++
			// Real nodes should have file paths and line numbers
			if node.FilePath == "" {
				t.Errorf("real node %s should have FilePath", node.ID)
			}
			if node.Line == 0 {
				t.Errorf("real node %s should have non-zero Line", node.ID)
			}
		}
	}

	if realCount != 3 {
		t.Errorf("expected 3 real nodes, got %d", realCount)
	}
	if virtualCount != 2 {
		t.Errorf("expected 2 virtual nodes, got %d", virtualCount)
	}
}

// TestGetNodeNeighbors_RealToReal tests finding neighbors via method_calls
// Realistic Use Case: User expands a real node to see its call relationships
func TestGetNodeNeighbors_RealToReal(t *testing.T) {
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

	// Create symbols
	symbols := []types.Symbol{
		{
			ID:       "ServiceA.save",
			Name:     "save",
			Kind:     "method",
			FilePath: "src/ServiceA.java",
			Line:     10,
		},
		{
			ID:       "RepositoryB.insert",
			Name:     "insert",
			Kind:     "method",
			FilePath: "src/RepositoryB.java",
			Line:     20,
		},
		{
			ID:       "RepositoryB.validate",
			Name:     "validate",
			Kind:     "method",
			FilePath: "src/RepositoryB.java",
			Line:     30,
		},
	}

	for _, sym := range symbols {
		err = store.InsertSymbol(sym)
		if err != nil {
			t.Fatalf("InsertSymbol failed: %v", err)
		}
	}

	// Create method calls: ServiceA.save calls RepositoryB.insert and RepositoryB.validate
	_, err = store.db.Exec(`
		INSERT INTO method_calls (caller_id, callee_id, call_type, file_path, line, created_at)
		VALUES 
			(?, ?, 'direct', 'src/ServiceA.java', 15, ?),
			(?, ?, 'direct', 'src/ServiceA.java', 16, ?)
	`, "ServiceA.save", "RepositoryB.insert", now, "ServiceA.save", "RepositoryB.validate", now)
	if err != nil {
		t.Fatalf("Insert method_calls failed: %v", err)
	}

	// Get neighbors of ServiceA.save
	neighbors, err := store.GetNodeNeighbors("ServiceA.save")
	if err != nil {
		t.Fatalf("GetNodeNeighbors failed: %v", err)
	}

	if len(neighbors) != 2 {
		t.Fatalf("expected 2 neighbors, got %d", len(neighbors))
	}

	// Verify neighbors are correct
	neighborIDs := make(map[string]bool)
	for _, n := range neighbors {
		neighborIDs[n.ID] = true
	}

	if !neighborIDs["RepositoryB.insert"] {
		t.Error("expected RepositoryB.insert in neighbors")
	}
	if !neighborIDs["RepositoryB.validate"] {
		t.Error("expected RepositoryB.validate in neighbors")
	}
}

// TestGetNodeNeighbors_VirtualToReal tests hybrid edges
// Realistic Use Case: User plans how new feature will call existing code
func TestGetNodeNeighbors_VirtualToReal(t *testing.T) {
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

	// Create real symbol
	realSymbol := types.Symbol{
		ID:       "ExistingService.validate",
		Name:     "validate",
		Kind:     "method",
		FilePath: "src/ExistingService.java",
		Line:     10,
	}
	err = store.InsertSymbol(realSymbol)
	if err != nil {
		t.Fatalf("InsertSymbol failed: %v", err)
	}

	// Create virtual node
	virtualNode := &types.VirtualNode{
		ID:        "vnode_new_feature",
		Label:     "NewFeature.process",
		Type:      "method",
		IsVirtual: true,

		UpdatedAt: now,
	}
	err = store.InsertVirtualNode(*virtualNode)
	if err != nil {
		t.Fatalf("InsertVirtualNode failed: %v", err)
	}

	// Create edge annotation: NewFeature.process → ExistingService.validate
	edgeID := "ExistingService.validate_vnode_new_feature" // Sorted
	annotation := &types.EdgeAnnotation{
		EdgeID:       edgeID,
		SourceNodeID: "vnode_new_feature",
		TargetNodeID: "ExistingService.validate",
		Remarks:      "NewFeature calls existing validation logic",
		CallCount:    1,

		UpdatedAt: now,
	}
	err = store.UpsertEdgeAnnotation(*annotation)
	if err != nil {
		t.Fatalf("UpsertEdgeAnnotation failed: %v", err)
	}

	// Get neighbors of virtual node
	neighbors, err := store.GetNodeNeighbors("vnode_new_feature")
	if err != nil {
		t.Fatalf("GetNodeNeighbors failed: %v", err)
	}

	if len(neighbors) != 1 {
		t.Fatalf("expected 1 neighbor, got %d", len(neighbors))
	}

	if neighbors[0].ID != "ExistingService.validate" {
		t.Errorf("expected neighbor ExistingService.validate, got %s", neighbors[0].ID)
	}
	if neighbors[0].IsVirtual {
		t.Error("neighbor should be real (isVirtual=false)")
	}
}

// TestGetNodeNeighbors_UndirectedEdges tests bidirectional edge traversal
// Realistic Use Case: User expects to find neighbors regardless of edge direction
func TestGetNodeNeighbors_UndirectedEdges(t *testing.T) {
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

	// Create two symbols
	symbols := []types.Symbol{
		{
			ID:       "NodeA",
			Name:     "NodeA",
			Kind:     "class",
			FilePath: "src/A.java",
			Line:     1,
		},
		{
			ID:       "NodeB",
			Name:     "NodeB",
			Kind:     "class",
			FilePath: "src/B.java",
			Line:     1,
		},
	}

	for _, sym := range symbols {
		err = store.InsertSymbol(sym)
		if err != nil {
			t.Fatalf("InsertSymbol failed: %v", err)
		}
	}

	// Create edge A → B via method_calls
	_, err = store.db.Exec(`
		INSERT INTO method_calls (caller_id, callee_id, call_type, file_path, line, created_at)
		VALUES (?, ?, 'direct', 'src/A.java', 10, ?)
	`, "NodeA", "NodeB", now)
	if err != nil {
		t.Fatalf("Insert method_call failed: %v", err)
	}

	// GetNodeNeighbors(A) should return B
	neighborsA, err := store.GetNodeNeighbors("NodeA")
	if err != nil {
		t.Fatalf("GetNodeNeighbors(NodeA) failed: %v", err)
	}

	if len(neighborsA) != 1 || neighborsA[0].ID != "NodeB" {
		t.Error("expected NodeA to have NodeB as neighbor")
	}

	// GetNodeNeighbors(B) should return A (undirected!)
	neighborsB, err := store.GetNodeNeighbors("NodeB")
	if err != nil {
		t.Fatalf("GetNodeNeighbors(NodeB) failed: %v", err)
	}

	if len(neighborsB) != 1 || neighborsB[0].ID != "NodeA" {
		t.Error("expected NodeB to have NodeA as neighbor (undirected edge)")
	}
}

// TestSearchGraphNodes tests searching across real and virtual nodes
// Realistic Use Case: User searches for nodes by name
func TestSearchGraphNodes(t *testing.T) {
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

	// Create real symbols
	symbols := []types.Symbol{
		{
			ID:       "UserService",
			Name:     "UserService",
			Kind:     "class",
			FilePath: "src/UserService.java",
			Line:     1,
		},
		{
			ID:       "UserRepository",
			Name:     "UserRepository",
			Kind:     "class",
			FilePath: "src/UserRepository.java",
			Line:     1,
		},
		{
			ID:       "ProductService",
			Name:     "ProductService",
			Kind:     "class",
			FilePath: "src/ProductService.java",
			Line:     1,
		},
	}

	for _, sym := range symbols {
		err = store.InsertSymbol(sym)
		if err != nil {
			t.Fatalf("InsertSymbol failed: %v", err)
		}
	}

	// Create virtual node
	vnode := &types.VirtualNode{
		ID:        "vnode_user_auth",
		Label:     "UserAuthService",
		Type:      "class",
		IsVirtual: true,

		UpdatedAt: now,
	}
	err = store.InsertVirtualNode(*vnode)
	if err != nil {
		t.Fatalf("InsertVirtualNode failed: %v", err)
	}

	// Search for "User" (should return 3 nodes: UserService, UserRepository, UserAuthService)
	results, err := store.SearchGraphNodes("User")
	if err != nil {
		t.Fatalf("SearchGraphNodes failed: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("expected 3 results for 'User', got %d", len(results))
	}

	// Verify results contain both real and virtual
	hasReal := false
	hasVirtual := false
	for _, node := range results {
		if node.IsVirtual {
			hasVirtual = true
		} else {
			hasReal = true
		}
	}

	if !hasReal || !hasVirtual {
		t.Error("expected search results to include both real and virtual nodes")
	}

	// Search for "nonexistent" (should return empty)
	emptyResults, err := store.SearchGraphNodes("nonexistent")
	if err != nil {
		t.Fatalf("SearchGraphNodes for nonexistent failed: %v", err)
	}

	if len(emptyResults) != 0 {
		t.Errorf("expected 0 results for 'nonexistent', got %d", len(emptyResults))
	}

	// Search for "Product" (should return 1 node: ProductService)
	productResults, err := store.SearchGraphNodes("Product")
	if err != nil {
		t.Fatalf("SearchGraphNodes for Product failed: %v", err)
	}

	if len(productResults) != 1 {
		t.Errorf("expected 1 result for 'Product', got %d", len(productResults))
	}
}

// TestGraphByRealFile tests querying nodes by real file path
// Realistic Use Case: User clicks on a file to see its methods
func TestGraphByRealFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fyp-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	_ = tmpDir // mark as used to avoid errors
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Create symbols in different files
	symbols := []types.Symbol{
		{
			ID:       "UserService.method1",
			Name:     "method1",
			Kind:     "method",
			FilePath: "src/User.java",
			Line:     10,
		},
		{
			ID:       "UserService.method2",
			Name:     "method2",
			Kind:     "method",
			FilePath: "src/User.java",
			Line:     20,
		},
		{
			ID:       "ProductService.method1",
			Name:     "method1",
			Kind:     "method",
			FilePath: "src/Product.java",
			Line:     10,
		},
	}

	for _, sym := range symbols {
		err = store.InsertSymbol(sym)
		if err != nil {
			t.Fatalf("InsertSymbol failed: %v", err)
		}
	}

	// Get nodes by User.java file
	userNodes, err := store.GetGraphNodesByFile("src/User.java")
	if err != nil {
		t.Fatalf("GetGraphNodesByFile failed: %v", err)
	}

	if len(userNodes) != 2 {
		t.Errorf("expected 2 nodes in src/User.java, got %d", len(userNodes))
	}

	// Verify all nodes belong to the file
	for _, node := range userNodes {
		if node.FilePath != "src/User.java" {
			t.Errorf("node %s has wrong FilePath: %s", node.ID, node.FilePath)
		}
		if node.IsVirtual {
			t.Errorf("node %s should be real (isVirtual=false)", node.ID)
		}
	}

	// Get nodes by Product.java file
	productNodes, err := store.GetGraphNodesByFile("src/Product.java")
	if err != nil {
		t.Fatalf("GetGraphNodesByFile for Product failed: %v", err)
	}

	if len(productNodes) != 1 {
		t.Errorf("expected 1 node in src/Product.java, got %d", len(productNodes))
	}
}

// TestGraphByVirtualClass tests that virtual nodes with a virtualClassId group correctly
func TestGraphByVirtualClass(t *testing.T) {
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

	// Create virtual class node
	classNode := types.VirtualNode{
		ID:               "vclass_auth",
		Label:            "AuthService",
		Type:             "class",
		VirtualDirectory: "src/auth",
		IsVirtual:        true,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	err = store.InsertVirtualNode(classNode)
	if err != nil {
		t.Fatalf("InsertVirtualNode (class) failed: %v", err)
	}

	// Create 3 virtual method nodes under that class
	vnodes := []types.VirtualNode{
		{ID: "vnode_1", Label: "AuthService.login", Type: "method", VirtualClassID: "vclass_auth", IsVirtual: true, CreatedAt: now, UpdatedAt: now},
		{ID: "vnode_2", Label: "AuthService.logout", Type: "method", VirtualClassID: "vclass_auth", IsVirtual: true, CreatedAt: now, UpdatedAt: now},
		{ID: "vnode_3", Label: "AuthService.refresh", Type: "method", VirtualClassID: "vclass_auth", IsVirtual: true, CreatedAt: now, UpdatedAt: now},
	}

	for _, vn := range vnodes {
		err = store.InsertVirtualNode(vn)
		if err != nil {
			t.Fatalf("InsertVirtualNode failed: %v", err)
		}
	}

	// Get all virtual nodes and verify virtualClassId is set correctly
	allNodes, err := store.GetAllVirtualNodes()
	if err != nil {
		t.Fatalf("GetAllVirtualNodes failed: %v", err)
	}

	if len(allNodes) != 4 { // 1 class + 3 methods
		t.Errorf("expected 4 virtual nodes, got %d", len(allNodes))
	}

	methodsWithClass := 0
	for _, n := range allNodes {
		if n.Type == "method" && n.VirtualClassID == "vclass_auth" {
			methodsWithClass++
		}
	}
	if methodsWithClass != 3 {
		t.Errorf("expected 3 method nodes with VirtualClassID='vclass_auth', got %d", methodsWithClass)
	}
}

// TestCountGraphNodes tests node counting
func TestCountGraphNodes(t *testing.T) {
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

	// Initially should be 0
	now := time.Now().Unix()
	count, err := store.CountGraphNodes()
	if err != nil {
		t.Fatalf("CountGraphNodes failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected count 0, got %d", count)
	}

	// Add 2 real symbols
	for i := 1; i <= 2; i++ {
		err = store.InsertSymbol(types.Symbol{
			ID:       "symbol_" + string(rune(i+'0')),
			Name:     "Symbol" + string(rune(i+'0')),
			Kind:     "class",
			FilePath: "test.java",
			Line:     i,
		})
		if err != nil {
			t.Fatalf("InsertSymbol failed: %v", err)
		}
	}

	// Add 3 virtual nodes
	for i := 1; i <= 3; i++ {
		err = store.InsertVirtualNode(types.VirtualNode{
			ID:        "vnode_" + string(rune(i+'0')),
			Label:     "VNode" + string(rune(i+'0')),
			Type:      "method",
			IsVirtual: true,

			UpdatedAt: now,
		})
		if err != nil {
			t.Fatalf("InsertVirtualNode failed: %v", err)
		}
	}

	// Count should be 5
	count, err = store.CountGraphNodes()
	if err != nil {
		t.Fatalf("CountGraphNodes failed: %v", err)
	}
	if count != 5 {
		t.Errorf("expected count 5, got %d", count)
	}
}

// TestGetGraphNode tests retrieving a single node by ID
func TestGetGraphNode(t *testing.T) {
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

	// Create a real symbol
	symbol := types.Symbol{
		ID:       "TestClass",
		Name:     "TestClass",
		Kind:     "class",
		FilePath: "src/Test.java",
		Line:     1,
	}
	err = store.InsertSymbol(symbol)
	if err != nil {
		t.Fatalf("InsertSymbol failed: %v", err)
	}

	// Get the real node
	node, err := store.GetGraphNode("TestClass")
	if err != nil {
		t.Fatalf("GetGraphNode failed: %v", err)
	}

	if node.ID != "TestClass" {
		t.Errorf("expected ID TestClass, got %s", node.ID)
	}
	if node.IsVirtual {
		t.Error("expected IsVirtual to be false")
	}

	// Create a virtual node
	vnode := &types.VirtualNode{
		ID:        "vnode_test",
		Label:     "VirtualTest",
		Type:      "class",
		IsVirtual: true,

		UpdatedAt: now,
	}
	err = store.InsertVirtualNode(*vnode)
	if err != nil {
		t.Fatalf("InsertVirtualNode failed: %v", err)
	}

	// Get the virtual node
	vnodeResult, err := store.GetGraphNode("vnode_test")
	if err != nil {
		t.Fatalf("GetGraphNode for virtual node failed: %v", err)
	}

	if vnodeResult.ID != "vnode_test" {
		t.Errorf("expected ID vnode_test, got %s", vnodeResult.ID)
	}
	if !vnodeResult.IsVirtual {
		t.Error("expected IsVirtual to be true")
	}

	// Try to get non-existent node
	_, err = store.GetGraphNode("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent node, got nil")
	}
}
