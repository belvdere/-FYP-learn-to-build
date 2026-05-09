package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"example.com/fyp/pkg/types"
)

// edgeSeparator is duplicated here to avoid a cross-package import cycle.
// Must stay in sync with api.createDirectedEdgeID.
const snapshotEdgeSep = "::"

func isSymbolBackedContextNode(n types.SnapshotNode) bool {
	if n.Type == "directory" {
		return false
	}
	if n.Type == "file" {
		return false
	}
	// Legacy file placeholders were serialized as class/interface nodes with line=0.
	if (n.Type == "class" || n.Type == "interface") && n.FilePath != "" && n.Line == 0 {
		return false
	}
	return true
}

// CreateSnapshot creates a new snapshot in the database.
func (s *Store) CreateSnapshot(snapshot *types.Snapshot) error {
	now := time.Now().Unix()

	virtualNodesJSON, err := json.Marshal(snapshot.VirtualNodes)
	if err != nil {
		return fmt.Errorf("marshal virtual nodes: %w", err)
	}

	contextNodesJSON, err := json.Marshal(snapshot.ContextNodes)
	if err != nil {
		return fmt.Errorf("marshal context nodes: %w", err)
	}

	edgesJSON, err := json.Marshal(snapshot.Edges)
	if err != nil {
		return fmt.Errorf("marshal edges: %w", err)
	}

	query := `
		INSERT INTO snapshots (id, name, virtual_nodes, context_nodes, edges, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err = s.db.Exec(query,
		snapshot.ID,
		snapshot.Name,
		string(virtualNodesJSON),
		string(contextNodesJSON),
		string(edgesJSON),
		now,
		now,
	)

	if err != nil {
		return fmt.Errorf("insert snapshot: %w", err)
	}

	snapshot.CreatedAt = now
	snapshot.UpdatedAt = now

	return nil
}

// GetSnapshot retrieves a snapshot by ID.
func (s *Store) GetSnapshot(id string) (*types.Snapshot, error) {
	query := `
		SELECT id, name, virtual_nodes, context_nodes, edges, custom_prompt, created_at, updated_at
		FROM snapshots
		WHERE id = ?
	`

	var snapshot types.Snapshot
	var name, customPrompt sql.NullString
	var virtualNodesJSON, contextNodesJSON, edgesJSON string

	err := s.db.QueryRow(query, id).Scan(
		&snapshot.ID,
		&name,
		&virtualNodesJSON,
		&contextNodesJSON,
		&edgesJSON,
		&customPrompt,
		&snapshot.CreatedAt,
		&snapshot.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("snapshot not found: %s", id)
		}
		return nil, fmt.Errorf("query snapshot: %w", err)
	}

	if name.Valid {
		snapshot.Name = name.String
	}

	if customPrompt.Valid {
		snapshot.CustomPrompt = customPrompt.String
	}

	if err := json.Unmarshal([]byte(virtualNodesJSON), &snapshot.VirtualNodes); err != nil {
		return nil, fmt.Errorf("unmarshal virtual nodes: %w", err)
	}

	if err := json.Unmarshal([]byte(contextNodesJSON), &snapshot.ContextNodes); err != nil {
		return nil, fmt.Errorf("unmarshal context nodes: %w", err)
	}

	if err := json.Unmarshal([]byte(edgesJSON), &snapshot.Edges); err != nil {
		return nil, fmt.Errorf("unmarshal edges: %w", err)
	}

	return &snapshot, nil
}

// ListSnapshots retrieves all snapshots, ordered by creation time (newest first).
func (s *Store) ListSnapshots() ([]*types.Snapshot, error) {
	query := `
		SELECT id, name, virtual_nodes, context_nodes, edges, custom_prompt, created_at, updated_at
		FROM snapshots
		ORDER BY created_at DESC
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query snapshots: %w", err)
	}
	defer rows.Close()

	var snapshots []*types.Snapshot
	for rows.Next() {
		var snapshot types.Snapshot
		var name, customPrompt sql.NullString
		var virtualNodesJSON, contextNodesJSON, edgesJSON string

		err := rows.Scan(
			&snapshot.ID,
			&name,
			&virtualNodesJSON,
			&contextNodesJSON,
			&edgesJSON,
			&customPrompt,
			&snapshot.CreatedAt,
			&snapshot.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan snapshot: %w", err)
		}

		if name.Valid {
			snapshot.Name = name.String
		}

		if customPrompt.Valid {
			snapshot.CustomPrompt = customPrompt.String
		}

		if err := json.Unmarshal([]byte(virtualNodesJSON), &snapshot.VirtualNodes); err != nil {
			return nil, fmt.Errorf("unmarshal virtual nodes: %w", err)
		}

		if err := json.Unmarshal([]byte(contextNodesJSON), &snapshot.ContextNodes); err != nil {
			return nil, fmt.Errorf("unmarshal context nodes: %w", err)
		}

		if err := json.Unmarshal([]byte(edgesJSON), &snapshot.Edges); err != nil {
			return nil, fmt.Errorf("unmarshal edges: %w", err)
		}

		snapshots = append(snapshots, &snapshot)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate snapshots: %w", err)
	}

	return snapshots, nil
}

// DeleteSnapshot deletes a snapshot by ID.
func (s *Store) DeleteSnapshot(id string) error {
	query := `DELETE FROM snapshots WHERE id = ?`

	result, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("delete snapshot: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("snapshot not found: %s", id)
	}

	return nil
}

// LoadSnapshotState materializes snapshot virtual nodes and manual edges into the DB
// inside a single transaction. Any write failure rolls back the entire load.
// Call ClearAllVirtualState first for a clean round-trip.
// Returns counts of restored nodes, edges, and missing context nodes (stale real symbols).
func (s *Store) LoadSnapshotState(snapshot *types.Snapshot) (restoredNodes, restoredEdges, missingContextNodes int, err error) {
	// Build set of virtual node IDs for parent resolution.
	virtualNodeIDs := make(map[string]bool, len(snapshot.VirtualNodes))
	for _, n := range snapshot.VirtualNodes {
		virtualNodeIDs[n.ID] = true
	}

	// Pre-resolve real context symbol existence before opening the write tx.
	// This serves two goals:
	// 1) missingContextNodes is a unique node count (not edge count)
	// 2) no s.db reads are performed while a tx is open (important with max-open-conns=1)
	contextCandidateIDs := make(map[string]struct{})
	nonSymbolContextIDs := make(map[string]struct{})
	for _, n := range snapshot.ContextNodes {
		if n.ID == "" || virtualNodeIDs[n.ID] {
			continue
		}
		if !isSymbolBackedContextNode(n) {
			nonSymbolContextIDs[n.ID] = struct{}{}
			continue
		}
		contextCandidateIDs[n.ID] = struct{}{}
	}
	for _, e := range snapshot.Edges {
		if !virtualNodeIDs[e.SourceID] && e.SourceID != "" {
			if _, skip := nonSymbolContextIDs[e.SourceID]; !skip {
				contextCandidateIDs[e.SourceID] = struct{}{}
			}
		}
		if !virtualNodeIDs[e.TargetID] && e.TargetID != "" {
			if _, skip := nonSymbolContextIDs[e.TargetID]; !skip {
				contextCandidateIDs[e.TargetID] = struct{}{}
			}
		}
	}

	symbolExists := make(map[string]bool, len(contextCandidateIDs))
	staleContextIDs := make(map[string]struct{})
	for id := range contextCandidateIDs {
		var count int
		if qErr := s.db.QueryRow(`SELECT COUNT(*) FROM symbols WHERE id = ?`, id).Scan(&count); qErr != nil {
			return 0, 0, 0, fmt.Errorf("check context symbol %q: %w", id, qErr)
		}
		exists := count > 0
		symbolExists[id] = exists
		if !exists {
			staleContextIDs[id] = struct{}{}
		}
	}
	missingContextNodes = len(staleContextIDs)

	// Fix P1/P2: wrap all writes in a single transaction; any failure rolls back entirely.
	tx, err := s.db.Begin()
	if err != nil {
		return 0, 0, missingContextNodes, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	now := time.Now().Unix()

	for _, n := range snapshot.VirtualNodes {
		virtualClassID := ""
		if n.ParentID != "" && virtualNodeIDs[n.ParentID] {
			virtualClassID = n.ParentID
		}

		if _, err = tx.Exec(`
			INSERT OR REPLACE INTO virtual_nodes
			(id, label, type, virtual_class_id, virtual_directory, is_virtual, materialized, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, 1, 0, ?, ?)
		`, n.ID, n.Label, n.Type, nullableString(virtualClassID), nullableString(""), now, now); err != nil {
			return 0, 0, missingContextNodes, fmt.Errorf("insert virtual node %q: %w", n.ID, err)
		}

		if n.Description != "" || n.AIRemarks != "" {
			if _, err = tx.Exec(`
				INSERT INTO node_annotations (node_id, description, ai_remarks, code_snippet, custom_properties, created_at, updated_at)
				VALUES (?, ?, ?, '', '', ?, ?)
				ON CONFLICT(node_id) DO UPDATE SET
					description = excluded.description,
					ai_remarks  = excluded.ai_remarks,
					updated_at  = excluded.updated_at
			`, n.ID, n.Description, n.AIRemarks, now, now); err != nil {
				return 0, 0, missingContextNodes, fmt.Errorf("upsert annotation for %q: %w", n.ID, err)
			}
		}
		restoredNodes++
	}

	// Restore annotations for context nodes (real symbols) that have description/aiRemarks
	// and still exist in the symbols table. Stale context nodes are skipped — they're
	// rendered in-memory only and must not pollute the annotation store.
	for _, n := range snapshot.ContextNodes {
		if n.Description == "" && n.AIRemarks == "" {
			continue
		}
		if !symbolExists[n.ID] {
			continue // stale — shown in-memory only
		}
		if _, err = tx.Exec(`
			INSERT INTO node_annotations (node_id, description, ai_remarks, code_snippet, custom_properties, created_at, updated_at)
			VALUES (?, ?, ?, '', '', ?, ?)
			ON CONFLICT(node_id) DO UPDATE SET
				description = excluded.description,
				ai_remarks  = excluded.ai_remarks,
				updated_at  = excluded.updated_at
		`, n.ID, n.Description, n.AIRemarks, now, now); err != nil {
			return 0, 0, missingContextNodes, fmt.Errorf("upsert context annotation for %q: %w", n.ID, err)
		}
	}

	for _, e := range snapshot.Edges {
		edgeID := e.SourceID + snapshotEdgeSep + e.TargetID

		if !virtualNodeIDs[e.SourceID] && !virtualNodeIDs[e.TargetID] {
			// Real-real edge: both endpoints must still exist to be restorable.
			if !symbolExists[e.SourceID] || !symbolExists[e.TargetID] {
				continue
			}

			// Materialize missing real-real relationships as manual edges so
			// snapshot round-trip preserves user-visible graph structure even
			// when indexed edges are currently absent.
			var existingCount int
			if err = tx.QueryRow(`
				SELECT COUNT(*) FROM method_calls
				WHERE caller_id = ? AND callee_id = ?
			`, e.SourceID, e.TargetID).Scan(&existingCount); err != nil {
				return 0, 0, missingContextNodes, fmt.Errorf("check existing edge %q→%q: %w", e.SourceID, e.TargetID, err)
			}
			if existingCount == 0 {
				if _, err = tx.Exec(`
					INSERT INTO method_calls (caller_id, callee_id, call_type, file_path, line, created_at)
					VALUES (?, ?, 'manual', '', 0, ?)
				`, e.SourceID, e.TargetID, now); err != nil {
					return 0, 0, missingContextNodes, fmt.Errorf("insert real-real manual edge %q→%q: %w", e.SourceID, e.TargetID, err)
				}
			}

			// Preserve edge remarks for real-real edges as well.
			if e.Remarks != "" {
				if _, err = tx.Exec(`
					INSERT INTO edge_annotations (edge_id, source_node_id, target_node_id, remarks, call_count, created_at, updated_at)
					VALUES (?, ?, ?, ?, 0, ?, ?)
					ON CONFLICT(edge_id) DO UPDATE SET
						remarks    = excluded.remarks,
						updated_at = excluded.updated_at
				`, edgeID, e.SourceID, e.TargetID, e.Remarks, now, now); err != nil {
					return 0, 0, missingContextNodes, fmt.Errorf("upsert real-real edge annotation %q: %w", edgeID, err)
				}
			}
			restoredEdges++
			continue
		}

		// Virtual-involved edge: keep idempotent by inserting only when absent.
		var existingManual int
		if err = tx.QueryRow(`
			SELECT COUNT(*) FROM method_calls
			WHERE caller_id = ? AND callee_id = ? AND call_type = 'manual'
		`, e.SourceID, e.TargetID).Scan(&existingManual); err != nil {
			return 0, 0, missingContextNodes, fmt.Errorf("check existing manual edge %q→%q: %w", e.SourceID, e.TargetID, err)
		}
		if existingManual == 0 {
			if _, err = tx.Exec(`
				INSERT INTO method_calls (caller_id, callee_id, call_type, file_path, line, created_at)
				VALUES (?, ?, 'manual', '', 0, ?)
			`, e.SourceID, e.TargetID, now); err != nil {
				return 0, 0, missingContextNodes, fmt.Errorf("insert manual edge %q→%q: %w", e.SourceID, e.TargetID, err)
			}
		}

		if e.Remarks != "" {
			if _, err = tx.Exec(`
				INSERT INTO edge_annotations (edge_id, source_node_id, target_node_id, remarks, call_count, created_at, updated_at)
				VALUES (?, ?, ?, ?, 0, ?, ?)
				ON CONFLICT(edge_id) DO UPDATE SET
					remarks    = excluded.remarks,
					updated_at = excluded.updated_at
			`, edgeID, e.SourceID, e.TargetID, e.Remarks, now, now); err != nil {
				return 0, 0, missingContextNodes, fmt.Errorf("upsert edge annotation %q: %w", edgeID, err)
			}
		}
		restoredEdges++
	}

	if err = tx.Commit(); err != nil {
		return 0, 0, missingContextNodes, fmt.Errorf("commit: %w", err)
	}
	return restoredNodes, restoredEdges, missingContextNodes, nil
}

// UpdateSnapshotCustomPrompt updates the custom prompt for a snapshot.
func (s *Store) UpdateSnapshotCustomPrompt(id string, customPrompt string) error {
	now := time.Now().Unix()
	query := `UPDATE snapshots SET custom_prompt = ?, updated_at = ? WHERE id = ?`

	result, err := s.db.Exec(query, customPrompt, now, id)
	if err != nil {
		return fmt.Errorf("update snapshot custom prompt: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("snapshot not found: %s", id)
	}

	return nil
}

// ClearSnapshotCustomPrompt clears the custom prompt for a snapshot (resets to generated).
func (s *Store) ClearSnapshotCustomPrompt(id string) error {
	now := time.Now().Unix()
	query := `UPDATE snapshots SET custom_prompt = NULL, updated_at = ? WHERE id = ?`

	result, err := s.db.Exec(query, now, id)
	if err != nil {
		return fmt.Errorf("clear snapshot custom prompt: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("snapshot not found: %s", id)
	}

	return nil
}
