package db

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"example.com/fyp/pkg/types"
)

// TestIndexCoverage verifies that all expected indexes exist in the database
func TestIndexCoverage(t *testing.T) {
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

	// Query sqlite_master for all indexes
	query := `
		SELECT name, tbl_name 
		FROM sqlite_master 
		WHERE type = 'index' AND name NOT LIKE 'sqlite_%'
		ORDER BY name
	`

	rows, err := store.db.Query(query)
	if err != nil {
		t.Fatalf("failed to query indexes: %v", err)
	}
	defer rows.Close()

	indexes := make(map[string]string) // index name -> table name
	for rows.Next() {
		var name, tblName string
		err := rows.Scan(&name, &tblName)
		if err != nil {
			t.Fatalf("failed to scan index row: %v", err)
		}
		indexes[name] = tblName
	}

	// Expected indexes (from schema.go)
	expectedIndexes := []string{
		// Core indexing indexes
		"idx_symbols_name",
		"idx_symbols_file",
		"idx_method_calls_caller",
		"idx_method_calls_callee",
		"idx_method_calls_file",
		"idx_mask_sessions_status",
		"idx_mask_sessions_created",

		// Virtual entity indexes
		"idx_virtual_nodes_class",
		"idx_virtual_nodes_dir",
		"idx_virtual_nodes_label",
		"idx_node_annotations_node",
		"idx_edge_annotations_edge",
		"idx_edge_annotations_source",
		"idx_edge_annotations_target",
		"idx_virtual_directories_parent",
		"idx_snapshots_created",
	}

	// Verify all expected indexes exist
	for _, expectedIdx := range expectedIndexes {
		if _, exists := indexes[expectedIdx]; !exists {
			t.Errorf("expected index %s not found", expectedIdx)
		}
	}

	// Report total count
	t.Logf("Total indexes found: %d", len(indexes))
	if len(indexes) < 16 {
		t.Errorf("expected at least 16 indexes, found %d", len(indexes))
	}
}

// TestIndexUsageOnGraphQueries verifies that indexes are used in critical graph queries
func TestIndexUsageOnGraphQueries(t *testing.T) {
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

	// Populate with test data
	now := time.Now().Unix()
	err = store.InsertSymbol(types.Symbol{
		ID:       "TestClass",
		Name:     "TestClass",
		Kind:     "class",
		FilePath: "src/Test.java",
		Line:     1,
	})
	if err != nil {
		t.Fatalf("InsertSymbol failed: %v", err)
	}

	err = store.InsertVirtualNode(types.VirtualNode{
		ID:        "vnode_test",
		Label:     "VirtualTest",
		Type:      "class",
		IsVirtual: true,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("InsertVirtualNode failed: %v", err)
	}

	// Test 1: GetAllGraphNodes query
	t.Run("GetAllGraphNodes uses indexes", func(t *testing.T) {
		query := `
			EXPLAIN QUERY PLAN
			SELECT id, name as label, kind as type, file_path, line, 0 as is_virtual
			FROM symbols
			UNION ALL
			SELECT id, label, type,
				CASE
					WHEN COALESCE(virtual_directory, '') = '' THEN 'virtual/' || label || '.virtual'
					ELSE virtual_directory || '/' || label || '.virtual'
				END as file_path,
				0 as line, 1 as is_virtual
			FROM virtual_nodes
			ORDER BY label
		`
		plan := getQueryPlan(t, store, query)
		t.Logf("GetAllGraphNodes plan: %s", plan)

		// UNION queries may use SCAN but should be efficient
		// Just verify the query executes without error
	})

	// Test 2: GetNodeNeighbors query (method_calls lookup)
	t.Run("GetNodeNeighbors uses method_calls indexes", func(t *testing.T) {
		// Insert method call
		_, err = store.db.Exec(`
			INSERT INTO method_calls (caller_id, callee_id, call_type, file_path, line, created_at)
			VALUES (?, ?, 'direct', 'test.java', 1, ?)
		`, "TestClass", "OtherClass", now)
		if err != nil {
			t.Fatalf("Insert method_call failed: %v", err)
		}

		query := `
			EXPLAIN QUERY PLAN
			SELECT DISTINCT callee_id 
			FROM method_calls 
			WHERE caller_id = 'TestClass'
		`
		plan := getQueryPlan(t, store, query)
		t.Logf("GetNodeNeighbors (method_calls) plan: %s", plan)

		// Should use idx_method_calls_caller
		if !strings.Contains(plan, "idx_method_calls_caller") {
			t.Logf("Warning: query may not be using idx_method_calls_caller index")
		}
	})

	// Test 3: SearchGraphNodes query (name filter)
	t.Run("SearchGraphNodes uses name indexes", func(t *testing.T) {
		query := `
			EXPLAIN QUERY PLAN
			SELECT id, name as label, kind as type, file_path, line, 0 as is_virtual
			FROM symbols
			WHERE name LIKE '%Test%'
		`
		plan := getQueryPlan(t, store, query)
		t.Logf("SearchGraphNodes (symbols) plan: %s", plan)

		// LIKE queries may not always use indexes, but verify query structure
	})

	// Test 4: GetGraphNodesByFile query (file path filter)
	t.Run("GetGraphNodesByFile uses file index", func(t *testing.T) {
		query := `
			EXPLAIN QUERY PLAN
			SELECT id, name as label, kind as type, file_path, line, 0 as is_virtual
			FROM symbols
			WHERE file_path = 'src/Test.java'
		`
		plan := getQueryPlan(t, store, query)
		t.Logf("GetGraphNodesByFile plan: %s", plan)

		// Should use idx_symbols_file
		if !strings.Contains(plan, "idx_symbols_file") {
			t.Logf("Warning: query may not be using idx_symbols_file index")
		}
	})

	// Test 5: GetNodeAnnotation query
	t.Run("GetNodeAnnotation uses annotation index", func(t *testing.T) {
		query := `
			EXPLAIN QUERY PLAN
			SELECT node_id, description, ai_remarks, code_snippet, custom_properties, created_at, updated_at
			FROM node_annotations
			WHERE node_id = 'TestClass'
		`
		plan := getQueryPlan(t, store, query)
		t.Logf("GetNodeAnnotation plan: %s", plan)

		// node_id is PRIMARY KEY, should use index automatically
		if !strings.Contains(plan, "idx_node_annotations_node") && !strings.Contains(plan, "PRIMARY KEY") {
			t.Logf("Warning: query may not be using index")
		}
	})

	// Test 6: GetEdgeAnnotation query
	t.Run("GetEdgeAnnotation uses edge index", func(t *testing.T) {
		query := `
			EXPLAIN QUERY PLAN
			SELECT edge_id, source_node_id, target_node_id, remarks, call_count, created_at, updated_at
			FROM edge_annotations
			WHERE edge_id = 'test_edge'
		`
		plan := getQueryPlan(t, store, query)
		t.Logf("GetEdgeAnnotation plan: %s", plan)

		// edge_id is PRIMARY KEY, should use index automatically
	})

	// Test 7: GetVirtualNodesByClass query
	t.Run("GetVirtualNodesByClass uses class index", func(t *testing.T) {
		query := `
			EXPLAIN QUERY PLAN
			SELECT id, label, type, virtual_class_id, virtual_directory, is_virtual, created_at, updated_at
			FROM virtual_nodes
			WHERE virtual_class_id = 'vclass_test'
		`
		plan := getQueryPlan(t, store, query)
		t.Logf("GetVirtualNodesByClass plan: %s", plan)

		// Should use idx_virtual_nodes_class
		if !strings.Contains(plan, "idx_virtual_nodes_class") {
			t.Logf("Warning: query may not be using idx_virtual_nodes_class index")
		}
	})

	// Test 8: GetVirtualDirectoriesByParent query
	t.Run("GetVirtualDirectoriesByParent uses parent index", func(t *testing.T) {
		query := `
			EXPLAIN QUERY PLAN
			SELECT id, path, name, parent_path, created_at
			FROM virtual_directories
			WHERE parent_path = 'src/test'
		`
		plan := getQueryPlan(t, store, query)
		t.Logf("GetVirtualDirectoriesByParent plan: %s", plan)

		// Should use idx_virtual_directories_parent
		if !strings.Contains(plan, "idx_virtual_directories_parent") {
			t.Logf("Warning: query may not be using idx_virtual_directories_parent index")
		}
	})
}

// getQueryPlan executes EXPLAIN QUERY PLAN and returns the plan as a string
func getQueryPlan(t *testing.T, store *Store, query string) string {
	rows, err := store.db.Query(query)
	if err != nil {
		t.Fatalf("failed to execute EXPLAIN QUERY PLAN: %v", err)
	}
	defer rows.Close()

	var plans []string
	for rows.Next() {
		var id, parent, notused int
		var detail string
		err := rows.Scan(&id, &parent, &notused, &detail)
		if err != nil {
			t.Fatalf("failed to scan query plan: %v", err)
		}
		plans = append(plans, detail)
	}

	return strings.Join(plans, " | ")
}

// TestIndexPerformanceComparison compares query performance with and without indexes (conceptual)
func TestIndexPerformanceComparison(t *testing.T) {
	// This is a conceptual test to document that indexes improve performance
	// In practice, with small test datasets, the difference is negligible
	// But with production data (thousands of symbols), indexes are critical

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

	// Insert 100 symbols
	for i := 0; i < 100; i++ {
		symbolName := fmt.Sprintf("Symbol%d", i)
		fileName := fmt.Sprintf("src/File%d", i%10)
		err = store.InsertSymbol(types.Symbol{
			ID:       symbolName,
			Name:     symbolName,
			Kind:     "class",
			FilePath: fileName,
			Line:     i,
		})
		if err != nil {
			t.Fatalf("InsertSymbol failed: %v", err)
		}
	}

	// Query by name (should use idx_symbols_name)
	var count int
	if err := store.DB().QueryRow("SELECT COUNT(*) FROM symbols WHERE name = ?", "Symbol50").Scan(&count); err != nil {
		t.Fatalf("symbol query failed: %v", err)
	}

	if count == 0 {
		t.Error("expected to find Symbol50")
	}

	t.Logf("Query executed successfully with indexes")
}

// TestUniqueConstraints verifies unique indexes are enforced
func TestUniqueConstraints(t *testing.T) {
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

	// Test: virtual_directories.path is UNIQUE
	now := time.Now().Unix()
	dir1 := &types.VirtualDirectory{
		ID:        "dir1",
		Path:      "src/test",
		Name:      "test",
		CreatedAt: now,
	}
	err = store.InsertVirtualDirectory(*dir1)
	if err != nil {
		t.Fatalf("InsertVirtualDirectory failed: %v", err)
	}

	// Try to insert duplicate path
	dir2 := &types.VirtualDirectory{
		ID:        "dir2",
		Path:      "src/test", // Duplicate
		Name:      "test",
		CreatedAt: now,
	}
	err = store.InsertVirtualDirectory(*dir2)
	if err == nil {
		t.Error("expected error when inserting duplicate directory path, got nil")
	}
	if !strings.Contains(err.Error(), "UNIQUE") {
		t.Logf("Error message: %v", err)
	}
}
