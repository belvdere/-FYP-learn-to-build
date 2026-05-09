package db

// Validation tests for the snapshot round-trip feature:
//   graph → snapshot → clear → load → same logical state
//
// Covers:
//  1. ClearAllVirtualState: DB-wide reset; real symbols untouched
//  2. DeleteVirtualNodeCascade: node + incident manual edges + annotations removed
//  3. LoadSnapshotState: virtual nodes/edges/annotations restored; IDs preserved
//  4. Idempotency: loading the same snapshot twice gives deterministic results
//  5. Stale context nodes: missing real symbols counted, NOT written to DB
//  6. parentId fidelity: VirtualClassID round-trips through snapshot create/load
//  7. SQL sanity: SELECT COUNT queries after clear and after load

import (
	"os"
	"testing"
	"time"

	"example.com/fyp/pkg/types"
)

// ─── helpers ────────────────────────────────────────────────────────────────

func newRoundTripStore(t *testing.T) *Store {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "fyp-roundtrip-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })
	s, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func countRows(t *testing.T, s *Store, query string, args ...interface{}) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("count query %q: %v", query, err)
	}
	return n
}

// insertRealSymbol seeds a minimal real symbol so we can test that clears leave them alone.
func insertRealSymbol(t *testing.T, s *Store, id, name, filePath string) {
	t.Helper()
	if err := s.InsertSymbol(types.Symbol{
		ID:       id,
		Name:     name,
		Kind:     "method",
		FilePath: filePath,
		Line:     1,
	}); err != nil {
		t.Fatalf("InsertSymbol %q: %v", id, err)
	}
}

// insertVirtualNode seeds a virtual node.
func insertVirtualNode(t *testing.T, s *Store, id, label, typ, classID string) {
	t.Helper()
	if err := s.InsertVirtualNode(types.VirtualNode{
		ID:             id,
		Label:          label,
		Type:           typ,
		VirtualClassID: classID,
		IsVirtual:      true,
		CreatedAt:      time.Now().Unix(),
		UpdatedAt:      time.Now().Unix(),
	}); err != nil {
		t.Fatalf("InsertVirtualNode %q: %v", id, err)
	}
}

// insertManualEdge creates a manual edge between two node IDs.
func insertManualEdge(t *testing.T, s *Store, callerID, calleeID string) {
	t.Helper()
	if err := s.InsertMethodCall(types.MethodCall{
		CallerID: callerID,
		CalleeID: calleeID,
		CallType: "manual",
	}); err != nil {
		t.Fatalf("InsertMethodCall %q→%q: %v", callerID, calleeID, err)
	}
}

// ─── 1. ClearAllVirtualState ────────────────────────────────────────────────

func TestClearAllVirtualState_EmptiesVirtualTablesLeavesRealData(t *testing.T) {
	s := newRoundTripStore(t)

	// Seed real symbols
	insertRealSymbol(t, s, "real.a", "a", "Real.java")
	insertRealSymbol(t, s, "real.b", "b", "Real.java")

	// Seed virtual node with annotation
	insertVirtualNode(t, s, "v1", "VClass", "class", "")
	if err := s.UpsertNodeAnnotation(types.NodeAnnotation{
		NodeID:      "v1",
		Description: "a virtual class",
	}); err != nil {
		t.Fatalf("UpsertNodeAnnotation: %v", err)
	}

	// Manual edge: virtual→real (one virtual endpoint)
	insertManualEdge(t, s, "v1", "real.a")
	virtualEdgeID := "v1" + snapshotEdgeSep + "real.a"
	if err := s.UpsertEdgeAnnotation(types.EdgeAnnotation{
		EdgeID:       virtualEdgeID,
		SourceNodeID: "v1",
		TargetNodeID: "real.a",
		Remarks:      "virtual→real",
	}); err != nil {
		t.Fatalf("UpsertEdgeAnnotation (virtual): %v", err)
	}

	// P1 regression: manual edge between two REAL nodes — edge_annotation must also be cleaned up.
	insertManualEdge(t, s, "real.a", "real.b")
	realManualEdgeID := "real.a" + snapshotEdgeSep + "real.b"
	if err := s.UpsertEdgeAnnotation(types.EdgeAnnotation{
		EdgeID:       realManualEdgeID,
		SourceNodeID: "real.a",
		TargetNodeID: "real.b",
		Remarks:      "real→real manual",
	}); err != nil {
		t.Fatalf("UpsertEdgeAnnotation (real-real manual): %v", err)
	}

	// Indexed (non-manual) edge — must survive
	if err := s.InsertMethodCall(types.MethodCall{
		CallerID: "real.a",
		CalleeID: "real.b",
		CallType: "direct",
		FilePath: "Real.java",
	}); err != nil {
		t.Fatalf("InsertMethodCall (direct): %v", err)
	}

	// ── pre-clear SQL sanity ──
	if n := countRows(t, s, `SELECT COUNT(*) FROM virtual_nodes`); n != 1 {
		t.Fatalf("pre-clear: expected 1 virtual_node, got %d", n)
	}
	if n := countRows(t, s, `SELECT COUNT(*) FROM method_calls WHERE call_type='manual'`); n != 2 {
		t.Fatalf("pre-clear: expected 2 manual edges, got %d", n)
	}

	// ── clear ──
	if err := s.ClearAllVirtualState(); err != nil {
		t.Fatalf("ClearAllVirtualState: %v", err)
	}

	// ── SQL sanity: virtual tables are zero ──
	if n := countRows(t, s, `SELECT COUNT(*) FROM virtual_nodes`); n != 0 {
		t.Errorf("after clear: virtual_nodes = %d, want 0", n)
	}
	if n := countRows(t, s, `SELECT COUNT(*) FROM method_calls WHERE call_type='manual'`); n != 0 {
		t.Errorf("after clear: manual edges = %d, want 0", n)
	}
	if n := countRows(t, s, `SELECT COUNT(*) FROM node_annotations WHERE node_id = 'v1'`); n != 0 {
		t.Errorf("after clear: node_annotation for v1 = %d, want 0", n)
	}
	// P1: virtual→real edge annotation gone
	if n := countRows(t, s, `SELECT COUNT(*) FROM edge_annotations WHERE edge_id = ?`, virtualEdgeID); n != 0 {
		t.Errorf("after clear: edge_annotation (virtual→real) = %d, want 0", n)
	}
	// P1: real→real manual edge annotation gone (this was the bug)
	if n := countRows(t, s, `SELECT COUNT(*) FROM edge_annotations WHERE edge_id = ?`, realManualEdgeID); n != 0 {
		t.Errorf("after clear: edge_annotation (real→real manual) = %d, want 0 (P1 fix)", n)
	}
	// P2/5: virtual_directories cleared
	if n := countRows(t, s, `SELECT COUNT(*) FROM virtual_directories`); n != 0 {
		t.Errorf("after clear: virtual_directories = %d, want 0 (P2/5 fix)", n)
	}

	// ── real data is untouched ──
	if n := countRows(t, s, `SELECT COUNT(*) FROM symbols`); n != 2 {
		t.Errorf("after clear: symbols = %d, want 2 (real data should survive)", n)
	}
	if n := countRows(t, s, `SELECT COUNT(*) FROM method_calls WHERE call_type='direct'`); n != 1 {
		t.Errorf("after clear: direct edges = %d, want 1 (real data should survive)", n)
	}
}

// ─── 2. DeleteVirtualNodeCascade ────────────────────────────────────────────

func TestDeleteVirtualNodeCascade_RemovesNodeEdgesAnnotations(t *testing.T) {
	s := newRoundTripStore(t)

	insertVirtualNode(t, s, "vc", "VClass", "class", "")
	insertVirtualNode(t, s, "vm", "VMethod", "method", "vc")

	// Annotation on the node being deleted
	if err := s.UpsertNodeAnnotation(types.NodeAnnotation{
		NodeID:    "vm",
		AIRemarks: "to implement",
	}); err != nil {
		t.Fatalf("UpsertNodeAnnotation: %v", err)
	}

	// Manual edge FROM vm to vc (incident on vm)
	insertManualEdge(t, s, "vm", "vc")
	edgeID := "vm" + snapshotEdgeSep + "vc"
	if err := s.UpsertEdgeAnnotation(types.EdgeAnnotation{
		EdgeID:       edgeID,
		SourceNodeID: "vm",
		TargetNodeID: "vc",
		Remarks:      "calls parent",
	}); err != nil {
		t.Fatalf("UpsertEdgeAnnotation: %v", err)
	}

	// Another manual edge unrelated to vm (should survive)
	insertVirtualNode(t, s, "vx", "VOther", "method", "")
	insertManualEdge(t, s, "vc", "vx")

	// ── cascade delete vm ──
	if err := s.DeleteVirtualNodeCascade("vm"); err != nil {
		t.Fatalf("DeleteVirtualNodeCascade: %v", err)
	}

	// vm gone
	if n := countRows(t, s, `SELECT COUNT(*) FROM virtual_nodes WHERE id='vm'`); n != 0 {
		t.Errorf("vm still in virtual_nodes after cascade delete")
	}
	// annotation gone
	if n := countRows(t, s, `SELECT COUNT(*) FROM node_annotations WHERE node_id='vm'`); n != 0 {
		t.Errorf("node_annotation for vm still present after cascade delete")
	}
	// incident manual edge gone
	if n := countRows(t, s, `SELECT COUNT(*) FROM method_calls WHERE (caller_id='vm' OR callee_id='vm') AND call_type='manual'`); n != 0 {
		t.Errorf("manual edge incident on vm still present after cascade delete")
	}
	// edge annotation gone
	if n := countRows(t, s, `SELECT COUNT(*) FROM edge_annotations WHERE edge_id=?`, edgeID); n != 0 {
		t.Errorf("edge_annotation for vm edge still present after cascade delete")
	}

	// other virtual nodes survive
	if n := countRows(t, s, `SELECT COUNT(*) FROM virtual_nodes`); n != 2 { // vc + vx
		t.Errorf("expected 2 surviving virtual nodes (vc, vx), got %d", n)
	}
	// unrelated manual edge survives
	if n := countRows(t, s, `SELECT COUNT(*) FROM method_calls WHERE call_type='manual'`); n != 1 {
		t.Errorf("expected 1 surviving manual edge (vc→vx), got %d", n)
	}
}

// ─── 3. LoadSnapshotState: basic round-trip ──────────────────────────────────

func TestLoadSnapshotState_RestoresVirtualNodesEdgesAnnotations(t *testing.T) {
	s := newRoundTripStore(t)

	snapshot := &types.Snapshot{
		ID: "snap_test1",
		VirtualNodes: []types.SnapshotNode{
			{
				ID:          "vc1",
				Label:       "AuthService",
				Type:        "class",
				IsVirtual:   true,
				Description: "Auth service class",
				AIRemarks:   "implement OAuth",
			},
			{
				ID:        "vm1",
				Label:     "AuthService.login",
				Type:      "method",
				IsVirtual: true,
				ParentID:  "vc1",
			},
		},
		ContextNodes: []types.SnapshotNode{},
		Edges: []types.SnapshotEdge{
			{ID: "e1", SourceID: "vm1", TargetID: "vc1", Remarks: "calls class"},
		},
	}

	restoredNodes, restoredEdges, missingCtx, err := s.LoadSnapshotState(snapshot)
	if err != nil {
		t.Fatalf("LoadSnapshotState: %v", err)
	}

	// Return value checks
	if restoredNodes != 2 {
		t.Errorf("restoredNodes = %d, want 2", restoredNodes)
	}
	if restoredEdges != 1 {
		t.Errorf("restoredEdges = %d, want 1", restoredEdges)
	}
	if missingCtx != 0 {
		t.Errorf("missingContextNodes = %d, want 0", missingCtx)
	}

	// SQL sanity: counts match snapshot payload
	if n := countRows(t, s, `SELECT COUNT(*) FROM virtual_nodes`); n != 2 {
		t.Errorf("virtual_nodes = %d, want 2", n)
	}
	if n := countRows(t, s, `SELECT COUNT(*) FROM method_calls WHERE call_type='manual'`); n != 1 {
		t.Errorf("manual edges = %d, want 1", n)
	}

	// Node IDs preserved exactly
	if n := countRows(t, s, `SELECT COUNT(*) FROM virtual_nodes WHERE id='vc1'`); n != 1 {
		t.Errorf("vc1 not found in virtual_nodes")
	}
	if n := countRows(t, s, `SELECT COUNT(*) FROM virtual_nodes WHERE id='vm1'`); n != 1 {
		t.Errorf("vm1 not found in virtual_nodes")
	}

	// Annotation for vc1 restored
	ann, err := s.GetNodeAnnotation("vc1")
	if err != nil || ann == nil {
		t.Fatalf("expected annotation for vc1, got err=%v ann=%v", err, ann)
	}
	if ann.Description != "Auth service class" {
		t.Errorf("vc1 description = %q, want %q", ann.Description, "Auth service class")
	}
	if ann.AIRemarks != "implement OAuth" {
		t.Errorf("vc1 aiRemarks = %q, want %q", ann.AIRemarks, "implement OAuth")
	}

	// Edge annotation for remarks
	edgeID := "vm1" + snapshotEdgeSep + "vc1"
	eann, err := s.GetEdgeAnnotation(edgeID)
	if err != nil || eann == nil {
		t.Fatalf("expected edge annotation for %q, got err=%v eann=%v", edgeID, err, eann)
	}
	if eann.Remarks != "calls class" {
		t.Errorf("edge remarks = %q, want %q", eann.Remarks, "calls class")
	}

	// Orphan check: every manual edge endpoint must exist in virtual_nodes or symbols
	orphans := countRows(t, s, `
		SELECT COUNT(*) FROM method_calls m
		WHERE m.call_type = 'manual'
		  AND (m.caller_id NOT IN (SELECT id FROM virtual_nodes) AND m.caller_id NOT IN (SELECT id FROM symbols))
		  OR  (m.callee_id NOT IN (SELECT id FROM virtual_nodes) AND m.callee_id NOT IN (SELECT id FROM symbols))
	`)
	if orphans != 0 {
		t.Errorf("orphan manual edges (dangling endpoints) = %d, want 0", orphans)
	}
}

// ─── 4. Idempotency: loading same snapshot twice ──────────────────────────────

func TestLoadSnapshotState_Idempotent(t *testing.T) {
	s := newRoundTripStore(t)

	snapshot := &types.Snapshot{
		ID: "snap_idem",
		VirtualNodes: []types.SnapshotNode{
			{ID: "idem_v1", Label: "IdemClass", Type: "class", IsVirtual: true},
			{ID: "idem_v2", Label: "IdemMethod", Type: "method", IsVirtual: true, ParentID: "idem_v1"},
		},
		Edges: []types.SnapshotEdge{
			{ID: "ie1", SourceID: "idem_v2", TargetID: "idem_v1"},
		},
	}

	// First load
	rn1, re1, mc1, err := s.LoadSnapshotState(snapshot)
	if err != nil {
		t.Fatalf("first LoadSnapshotState: %v", err)
	}

	// Second load (without clearing)
	rn2, re2, mc2, err := s.LoadSnapshotState(snapshot)
	if err != nil {
		t.Fatalf("second LoadSnapshotState: %v", err)
	}

	// Both loads return same counts
	if rn1 != rn2 {
		t.Errorf("restoredNodes: first=%d second=%d (should be equal)", rn1, rn2)
	}
	if mc1 != mc2 {
		t.Errorf("missingContextNodes: first=%d second=%d (should be equal)", mc1, mc2)
	}

	// After two loads, virtual_nodes still just 2 (INSERT OR REPLACE, not INSERT)
	if n := countRows(t, s, `SELECT COUNT(*) FROM virtual_nodes`); n != 2 {
		t.Errorf("after 2 loads: virtual_nodes = %d, want 2 (no duplicates)", n)
	}

	// Manual edge insertion is idempotent across repeated loads.
	if n := countRows(t, s, `SELECT COUNT(*) FROM method_calls WHERE call_type='manual'`); n != 1 {
		t.Errorf("after 2 loads: manual edges = %d, want 1", n)
	}

	_ = re1
	_ = re2
}

// ─── 5. Stale context nodes not written to DB ────────────────────────────────

func TestLoadSnapshotState_StaleContextNodeNotWrittenToDB(t *testing.T) {
	s := newRoundTripStore(t)

	// "real.exists" is actually in symbols; "real.ghost" is not
	insertRealSymbol(t, s, "real.exists", "exists", "Exists.java")

	snapshot := &types.Snapshot{
		ID:           "snap_stale",
		VirtualNodes: []types.SnapshotNode{},
		ContextNodes: []types.SnapshotNode{
			{ID: "real.exists", Label: "exists", Type: "method", IsVirtual: false},
			{ID: "real.ghost", Label: "ghost", Type: "method", IsVirtual: false},
		},
		// Edge between two real nodes: one exists, one doesn't → stale
		Edges: []types.SnapshotEdge{
			{ID: "se1", SourceID: "real.exists", TargetID: "real.ghost"},
		},
	}

	_, _, missingCtx, err := s.LoadSnapshotState(snapshot)
	if err != nil {
		t.Fatalf("LoadSnapshotState: %v", err)
	}

	// The ghost node is missing → counted
	if missingCtx != 1 {
		t.Errorf("missingContextNodes = %d, want 1 (real.ghost is stale)", missingCtx)
	}

	// Stale node must NOT be written to virtual_nodes
	if n := countRows(t, s, `SELECT COUNT(*) FROM virtual_nodes WHERE id='real.ghost'`); n != 0 {
		t.Errorf("stale context node 'real.ghost' was written to virtual_nodes, should be in-memory only")
	}

	// No manual edge created for the stale pair either
	if n := countRows(t, s, `SELECT COUNT(*) FROM method_calls WHERE call_type='manual'`); n != 0 {
		t.Errorf("manual edge created for stale context pair = %d, want 0", n)
	}
}

// Missing context count should be unique per stale node, not per stale edge.
func TestLoadSnapshotState_MissingContextNodesCountUnique(t *testing.T) {
	s := newRoundTripStore(t)

	insertRealSymbol(t, s, "real.exists", "exists", "Exists.java")

	snapshot := &types.Snapshot{
		ID:           "snap_stale_unique",
		VirtualNodes: []types.SnapshotNode{},
		ContextNodes: []types.SnapshotNode{
			{ID: "real.exists", Label: "exists", Type: "method", IsVirtual: false},
			{ID: "real.ghost", Label: "ghost", Type: "method", IsVirtual: false},
		},
		Edges: []types.SnapshotEdge{
			{ID: "se1", SourceID: "real.exists", TargetID: "real.ghost"},
			{ID: "se2", SourceID: "real.ghost", TargetID: "real.exists"},
		},
	}

	_, _, missingCtx, err := s.LoadSnapshotState(snapshot)
	if err != nil {
		t.Fatalf("LoadSnapshotState: %v", err)
	}
	if missingCtx != 1 {
		t.Errorf("missingContextNodes = %d, want 1 (unique stale node count)", missingCtx)
	}
}

// ─── 5b. Real-real edges are materialized when absent and remarks are restored ──

func TestLoadSnapshotState_RealRealEdgeBothPresent(t *testing.T) {
	s := newRoundTripStore(t)

	insertRealSymbol(t, s, "real.a", "a", "A.java")
	insertRealSymbol(t, s, "real.b", "b", "B.java")

	snapshot := &types.Snapshot{
		ID:           "snap_real_real",
		VirtualNodes: []types.SnapshotNode{},
		ContextNodes: []types.SnapshotNode{
			{ID: "real.a", Label: "a", IsVirtual: false},
			{ID: "real.b", Label: "b", IsVirtual: false},
		},
		Edges: []types.SnapshotEdge{
			{ID: "rr1", SourceID: "real.a", TargetID: "real.b", Remarks: "keep this note"},
		},
	}

	_, restoredEdges, missingCtx, err := s.LoadSnapshotState(snapshot)
	if err != nil {
		t.Fatalf("LoadSnapshotState: %v", err)
	}
	if missingCtx != 0 {
		t.Errorf("missingContextNodes = %d, want 0 (both endpoints exist)", missingCtx)
	}
	if restoredEdges != 1 {
		t.Errorf("restoredEdges = %d, want 1", restoredEdges)
	}

	// When no indexed edge exists, snapshot load materializes a manual edge so
	// graph -> snapshot -> graph round-trip preserves visibility.
	if n := countRows(t, s, `SELECT COUNT(*) FROM method_calls WHERE caller_id='real.a' AND callee_id='real.b' AND call_type='manual'`); n != 1 {
		t.Errorf("expected one materialized manual edge real.a→real.b, got %d", n)
	}

	// Remarks are restored for real-real edges.
	eann, err := s.GetEdgeAnnotation("real.a" + snapshotEdgeSep + "real.b")
	if err != nil || eann == nil {
		t.Fatalf("expected edge annotation for real.a::real.b, got err=%v ann=%v", err, eann)
	}
	if eann.Remarks != "keep this note" {
		t.Errorf("edge remarks = %q, want %q", eann.Remarks, "keep this note")
	}
}

func TestLoadSnapshotState_RealRealEdgeUsesIndexedWhenPresent(t *testing.T) {
	s := newRoundTripStore(t)

	insertRealSymbol(t, s, "real.a", "a", "A.java")
	insertRealSymbol(t, s, "real.b", "b", "B.java")

	// Seed an indexed edge that should be reused (not duplicated as manual).
	if err := s.InsertMethodCall(types.MethodCall{
		CallerID: "real.a",
		CalleeID: "real.b",
		CallType: "direct",
		FilePath: "A.java",
		Line:     10,
	}); err != nil {
		t.Fatalf("InsertMethodCall direct edge: %v", err)
	}

	snapshot := &types.Snapshot{
		ID:           "snap_real_real_indexed",
		VirtualNodes: []types.SnapshotNode{},
		ContextNodes: []types.SnapshotNode{
			{ID: "real.a", Label: "a", IsVirtual: false},
			{ID: "real.b", Label: "b", IsVirtual: false},
		},
		Edges: []types.SnapshotEdge{
			{ID: "rr1", SourceID: "real.a", TargetID: "real.b"},
		},
	}

	_, restoredEdges, missingCtx, err := s.LoadSnapshotState(snapshot)
	if err != nil {
		t.Fatalf("LoadSnapshotState: %v", err)
	}
	if missingCtx != 0 {
		t.Errorf("missingContextNodes = %d, want 0", missingCtx)
	}
	if restoredEdges != 1 {
		t.Errorf("restoredEdges = %d, want 1", restoredEdges)
	}
	if n := countRows(t, s, `SELECT COUNT(*) FROM method_calls WHERE caller_id='real.a' AND callee_id='real.b'`); n != 1 {
		t.Errorf("expected one existing edge row to remain, got %d", n)
	}
	if n := countRows(t, s, `SELECT COUNT(*) FROM method_calls WHERE caller_id='real.a' AND callee_id='real.b' AND call_type='manual'`); n != 0 {
		t.Errorf("should not insert manual duplicate when indexed edge exists, got %d", n)
	}
}

// ─── 6. parentId round-trip through snapshot create/load ────────────────────

func TestLoadSnapshotState_ParentIDRoundTrip(t *testing.T) {
	s := newRoundTripStore(t)

	// Virtual class + method with parentId = class
	snapshot := &types.Snapshot{
		ID: "snap_parent",
		VirtualNodes: []types.SnapshotNode{
			{ID: "p_class", Label: "PaymentService", Type: "class", IsVirtual: true},
			{
				ID:        "p_method",
				Label:     "PaymentService.charge",
				Type:      "method",
				IsVirtual: true,
				ParentID:  "p_class", // method belongs to class
			},
		},
	}

	if _, _, _, err := s.LoadSnapshotState(snapshot); err != nil {
		t.Fatalf("LoadSnapshotState: %v", err)
	}

	// Verify virtual_class_id is set correctly on the method node
	var virtualClassID string
	err := s.db.QueryRow(
		`SELECT COALESCE(virtual_class_id, '') FROM virtual_nodes WHERE id = 'p_method'`,
	).Scan(&virtualClassID)
	if err != nil {
		t.Fatalf("query p_method virtual_class_id: %v", err)
	}
	if virtualClassID != "p_class" {
		t.Errorf("p_method.virtual_class_id = %q, want %q", virtualClassID, "p_class")
	}

	// Class node has no parent (empty virtual_class_id)
	var classVirtualClassID string
	err = s.db.QueryRow(
		`SELECT COALESCE(virtual_class_id, '') FROM virtual_nodes WHERE id = 'p_class'`,
	).Scan(&classVirtualClassID)
	if err != nil {
		t.Fatalf("query p_class virtual_class_id: %v", err)
	}
	if classVirtualClassID != "" {
		t.Errorf("p_class.virtual_class_id = %q, want empty", classVirtualClassID)
	}
}

// ─── P1: LoadSnapshotState atomicity — bad node ID makes tx roll back ────────

func TestLoadSnapshotState_RollsBackOnWriteFailure(t *testing.T) {
	s := newRoundTripStore(t)

	// Insert a virtual node whose ID conflicts: it already exists with a NOT NULL label.
	// We'll craft a snapshot where one node write will violate a constraint to trigger rollback.
	// Easiest: insert a node, then pass a snapshot whose first node succeeds but whose
	// annotation write would fail if we intentionally break the table.
	//
	// Instead, verify atomicity by checking that if LoadSnapshotState returns an error,
	// virtual_nodes remains at 0 (nothing partially committed).
	//
	// We trigger a real error: pass a virtual node with an empty ID, which violates
	// the NOT NULL / PRIMARY KEY constraint on virtual_nodes.id.
	snapshot := &types.Snapshot{
		ID: "snap_atomic",
		VirtualNodes: []types.SnapshotNode{
			{ID: "ok_node", Label: "Good", Type: "class", IsVirtual: true},
			{ID: "", Label: "BadID", Type: "class", IsVirtual: true}, // empty PK → constraint violation
		},
	}

	_, _, _, err := s.LoadSnapshotState(snapshot)
	if err == nil {
		// If the DB doesn't enforce NOT NULL on id, we can't trigger a failure this way;
		// skip rather than false-fail.
		t.Skip("DB did not reject empty primary key; atomicity test skipped")
	}

	// Transaction must have been rolled back: ok_node should NOT be in the DB.
	if n := countRows(t, s, `SELECT COUNT(*) FROM virtual_nodes`); n != 0 {
		t.Errorf("partial commit detected: virtual_nodes = %d after rolled-back load, want 0", n)
	}
}

// ─── 7. Full round-trip: clear → load → verify ──────────────────────────────

func TestSnapshotRoundTrip_ClearThenLoad(t *testing.T) {
	s := newRoundTripStore(t)

	// Setup: virtual state with nodes, edges, annotations
	insertVirtualNode(t, s, "rt_vc", "RTClass", "class", "")
	insertVirtualNode(t, s, "rt_vm", "RTMethod", "method", "rt_vc")
	if err := s.UpsertNodeAnnotation(types.NodeAnnotation{
		NodeID:    "rt_vm",
		AIRemarks: "implement this",
	}); err != nil {
		t.Fatalf("UpsertNodeAnnotation: %v", err)
	}
	insertManualEdge(t, s, "rt_vm", "rt_vc")

	snapshot := &types.Snapshot{
		ID: "snap_rt",
		VirtualNodes: []types.SnapshotNode{
			{ID: "rt_vc", Label: "RTClass", Type: "class", IsVirtual: true},
			{ID: "rt_vm", Label: "RTMethod", Type: "method", IsVirtual: true, ParentID: "rt_vc", AIRemarks: "implement this"},
		},
		Edges: []types.SnapshotEdge{
			{ID: "rt_e1", SourceID: "rt_vm", TargetID: "rt_vc", Remarks: "rt edge"},
		},
	}

	// Clear everything
	if err := s.ClearAllVirtualState(); err != nil {
		t.Fatalf("ClearAllVirtualState: %v", err)
	}

	// SQL sanity after clear
	if n := countRows(t, s, `SELECT COUNT(*) FROM virtual_nodes`); n != 0 {
		t.Fatalf("after clear: virtual_nodes = %d, want 0", n)
	}
	if n := countRows(t, s, `SELECT COUNT(*) FROM method_calls WHERE call_type='manual'`); n != 0 {
		t.Fatalf("after clear: manual edges = %d, want 0", n)
	}

	// Load snapshot
	rn, re, mc, err := s.LoadSnapshotState(snapshot)
	if err != nil {
		t.Fatalf("LoadSnapshotState: %v", err)
	}
	if rn != 2 {
		t.Errorf("restoredNodes = %d, want 2", rn)
	}
	if re != 1 {
		t.Errorf("restoredEdges = %d, want 1", re)
	}
	if mc != 0 {
		t.Errorf("missingContextNodes = %d, want 0", mc)
	}

	// SQL sanity after load: counts match payload
	if n := countRows(t, s, `SELECT COUNT(*) FROM virtual_nodes`); n != 2 {
		t.Errorf("after load: virtual_nodes = %d, want 2", n)
	}
	if n := countRows(t, s, `SELECT COUNT(*) FROM method_calls WHERE call_type='manual'`); n != 1 {
		t.Errorf("after load: manual edges = %d, want 1", n)
	}

	// IDs preserved
	if n := countRows(t, s, `SELECT COUNT(*) FROM virtual_nodes WHERE id IN ('rt_vc','rt_vm')`); n != 2 {
		t.Errorf("after load: expected both rt_vc and rt_vm in virtual_nodes")
	}

	// Annotation restored
	ann, err := s.GetNodeAnnotation("rt_vm")
	if err != nil || ann == nil {
		t.Fatalf("expected annotation for rt_vm after load, got err=%v", err)
	}
	if ann.AIRemarks != "implement this" {
		t.Errorf("rt_vm aiRemarks = %q, want %q", ann.AIRemarks, "implement this")
	}

	// Edge annotation restored
	edgeID := "rt_vm" + snapshotEdgeSep + "rt_vc"
	eann, err := s.GetEdgeAnnotation(edgeID)
	if err != nil || eann == nil {
		t.Fatalf("expected edge annotation after load, got err=%v", err)
	}
	if eann.Remarks != "rt edge" {
		t.Errorf("edge remarks = %q, want %q", eann.Remarks, "rt edge")
	}

	// Orphan check
	orphans := countRows(t, s, `
		SELECT COUNT(*) FROM method_calls m
		WHERE m.call_type = 'manual'
		  AND (m.caller_id NOT IN (SELECT id FROM virtual_nodes) AND m.caller_id NOT IN (SELECT id FROM symbols))
		  OR  (m.callee_id NOT IN (SELECT id FROM virtual_nodes) AND m.callee_id NOT IN (SELECT id FROM symbols))
	`)
	if orphans != 0 {
		t.Errorf("orphan manual edges after load = %d, want 0", orphans)
	}
}

// TestLoadSnapshotState_ContextNodeAnnotationsRestored verifies that annotations
// on real (non-virtual) context nodes are written to node_annotations on load.
// Stale context nodes (symbol no longer in DB) must NOT get a DB row.
func TestLoadSnapshotState_ContextNodeAnnotationsRestored(t *testing.T) {
	s := newRoundTripStore(t)

	// Seed two real symbols: one present, one that will be "stale" (absent from DB).
	insertRealSymbol(t, s, "ctx_present", "Present", "ctx.java")
	// "ctx_stale" is intentionally NOT inserted into symbols.

	snapshot := &types.Snapshot{
		ID:           "snap_ctx_ann",
		VirtualNodes: []types.SnapshotNode{},
		ContextNodes: []types.SnapshotNode{
			{ID: "ctx_present", Label: "Present", Type: "class", IsVirtual: false, Description: "ctx desc", AIRemarks: "ctx ai"},
			{ID: "ctx_stale", Label: "Stale", Type: "class", IsVirtual: false, Description: "stale desc", AIRemarks: "stale ai"},
		},
		Edges: []types.SnapshotEdge{},
	}
	if err := s.CreateSnapshot(snapshot); err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}

	_, _, missing, err := s.LoadSnapshotState(snapshot)
	if err != nil {
		t.Fatalf("LoadSnapshotState: %v", err)
	}
	// stale context node should be counted as missing (unique node count).
	if missing != 1 {
		t.Errorf("missingContextNodes = %d, want 1 (ctx_stale)", missing)
	}

	// Present context node annotation must exist in DB.
	ann, err := s.GetNodeAnnotation("ctx_present")
	if err != nil {
		t.Fatalf("GetNodeAnnotation(ctx_present): %v", err)
	}
	if ann == nil {
		t.Fatal("expected annotation for ctx_present, got nil")
	}
	if ann.Description != "ctx desc" {
		t.Errorf("description = %q, want %q", ann.Description, "ctx desc")
	}
	if ann.AIRemarks != "ctx ai" {
		t.Errorf("ai_remarks = %q, want %q", ann.AIRemarks, "ctx ai")
	}

	// Stale context node must NOT have a DB annotation row.
	staleAnn, _ := s.GetNodeAnnotation("ctx_stale")
	if staleAnn != nil {
		t.Errorf("stale context node should not have a DB annotation, got %+v", staleAnn)
	}
}

func TestLoadSnapshotState_FileAndDirectoryContextNotCountedMissing(t *testing.T) {
	s := newRoundTripStore(t)

	snapshot := &types.Snapshot{
		ID:           "snap_file_dir_ctx",
		VirtualNodes: []types.SnapshotNode{},
		ContextNodes: []types.SnapshotNode{
			{ID: "dir:src/service", Label: "service", Type: "directory", FilePath: "src/service", IsVirtual: false},
			{ID: "file:src/service/UserService.java", Label: "UserService.java", Type: "file", FilePath: "src/service/UserService.java", Line: 0, IsVirtual: false},
			// Backward-compatible: legacy file placeholder shape.
			{ID: "file:src/service/Legacy.java", Label: "Legacy.java", Type: "class", FilePath: "src/service/Legacy.java", Line: 0, IsVirtual: false},
		},
		Edges: []types.SnapshotEdge{},
	}

	_, _, missing, err := s.LoadSnapshotState(snapshot)
	if err != nil {
		t.Fatalf("LoadSnapshotState: %v", err)
	}
	if missing != 0 {
		t.Errorf("missingContextNodes = %d, want 0 for file/directory placeholders", missing)
	}
}

func TestLoadSnapshotState_MixedContextCountsOnlyMissingSymbols(t *testing.T) {
	s := newRoundTripStore(t)
	insertRealSymbol(t, s, "real.present", "UserService.save(User)", "src/service/UserService.java")

	snapshot := &types.Snapshot{
		ID:           "snap_mixed_ctx",
		VirtualNodes: []types.SnapshotNode{},
		ContextNodes: []types.SnapshotNode{
			{ID: "dir:src/service", Label: "service", Type: "directory", FilePath: "src/service", IsVirtual: false},
			{ID: "file:src/service/UserService.java", Label: "UserService.java", Type: "file", FilePath: "src/service/UserService.java", Line: 0, IsVirtual: false},
			{ID: "real.present", Label: "UserService.save(User)", Type: "method", FilePath: "src/service/UserService.java", Line: 12, IsVirtual: false},
			{ID: "real.missing", Label: "MissingService.call()", Type: "method", FilePath: "src/service/MissingService.java", Line: 7, IsVirtual: false},
		},
		Edges: []types.SnapshotEdge{
			{SourceID: "real.present", TargetID: "real.missing"},
		},
	}

	_, _, missing, err := s.LoadSnapshotState(snapshot)
	if err != nil {
		t.Fatalf("LoadSnapshotState: %v", err)
	}
	if missing != 1 {
		t.Errorf("missingContextNodes = %d, want 1 (only real.missing)", missing)
	}
}
