package db

import (
	"database/sql"
	"fmt"
	"time"

	"example.com/fyp/pkg/types"
)

// nullableString returns a sql.NullString so empty Go strings are stored as SQL NULL.
func nullableString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

// InsertVirtualNode inserts a new virtual node into the database.
func (s *Store) InsertVirtualNode(node types.VirtualNode) error {
	now := time.Now().Unix()

	query := `
		INSERT INTO virtual_nodes
		(id, label, type, virtual_class_id, virtual_directory, is_virtual, materialized, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(query,
		node.ID,
		node.Label,
		node.Type,
		nullableString(node.VirtualClassID),
		nullableString(node.VirtualDirectory),
		1, // is_virtual always true
		0, // materialized defaults to false on create
		now,
		now,
	)

	return err
}

// GetVirtualNode retrieves a virtual node by ID.
func (s *Store) GetVirtualNode(id string) (*types.VirtualNode, error) {
	query := `
		SELECT id, label, type, virtual_class_id, virtual_directory, materialized, created_at, updated_at
		FROM virtual_nodes
		WHERE id = ?
	`

	var node types.VirtualNode
	var virtualClassID sql.NullString
	var virtualDir sql.NullString
	var materialized int

	err := s.db.QueryRow(query, id).Scan(
		&node.ID,
		&node.Label,
		&node.Type,
		&virtualClassID,
		&virtualDir,
		&materialized,
		&node.CreatedAt,
		&node.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	node.IsVirtual = true
	node.Materialized = materialized == 1
	if virtualClassID.Valid {
		node.VirtualClassID = virtualClassID.String
	}
	if virtualDir.Valid {
		node.VirtualDirectory = virtualDir.String
	}

	return &node, nil
}

// GetAllVirtualNodes retrieves all virtual nodes from the database.
func (s *Store) GetAllVirtualNodes() ([]types.VirtualNode, error) {
	query := `
		SELECT id, label, type, virtual_class_id, virtual_directory, materialized, created_at, updated_at
		FROM virtual_nodes
		ORDER BY created_at DESC
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query virtual nodes: %w", err)
	}
	defer rows.Close()

	var nodes []types.VirtualNode
	for rows.Next() {
		var node types.VirtualNode
		var virtualClassID sql.NullString
		var virtualDir sql.NullString
		var materialized int

		err := rows.Scan(
			&node.ID,
			&node.Label,
			&node.Type,
			&virtualClassID,
			&virtualDir,
			&materialized,
			&node.CreatedAt,
			&node.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan virtual node: %w", err)
		}

		node.IsVirtual = true
		node.Materialized = materialized == 1
		if virtualClassID.Valid {
			node.VirtualClassID = virtualClassID.String
		}
		if virtualDir.Valid {
			node.VirtualDirectory = virtualDir.String
		}

		nodes = append(nodes, node)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate virtual nodes: %w", err)
	}

	return nodes, nil
}

// UpdateVirtualNode updates an existing virtual node.
func (s *Store) UpdateVirtualNode(node types.VirtualNode) error {
	now := time.Now().Unix()

	query := `
		UPDATE virtual_nodes
		SET label = ?, type = ?, virtual_class_id = ?, virtual_directory = ?, materialized = ?, updated_at = ?
		WHERE id = ?
	`

	materializedInt := 0
	if node.Materialized {
		materializedInt = 1
	}

	result, err := s.db.Exec(query,
		node.Label,
		node.Type,
		nullableString(node.VirtualClassID),
		nullableString(node.VirtualDirectory),
		materializedInt,
		now,
		node.ID,
	)

	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return fmt.Errorf("virtual node not found: %s", node.ID)
	}

	return nil
}

// DeleteVirtualNode deletes a virtual node by ID.
// This operation is idempotent - it succeeds even if the node doesn't exist.
func (s *Store) DeleteVirtualNode(id string) error {
	query := `DELETE FROM virtual_nodes WHERE id = ?`

	_, err := s.db.Exec(query, id)
	return err
}

// DeleteVirtualNodeCascade deletes a virtual node and all its dependent rows
// (annotations, manual edges) in a single transaction.
func (s *Store) DeleteVirtualNodeCascade(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// Fix P2/3: check every preparatory exec so partial cleanup can't be committed.
	if _, err = tx.Exec(`DELETE FROM node_annotations WHERE node_id = ?`, id); err != nil {
		return fmt.Errorf("delete node_annotations: %w", err)
	}
	if _, err = tx.Exec(`DELETE FROM edge_annotations WHERE source_node_id = ? OR target_node_id = ?`, id, id); err != nil {
		return fmt.Errorf("delete edge_annotations: %w", err)
	}
	if _, err = tx.Exec(`DELETE FROM method_calls WHERE (caller_id = ? OR callee_id = ?) AND call_type = 'manual'`, id, id); err != nil {
		return fmt.Errorf("delete manual edges: %w", err)
	}
	if _, err = tx.Exec(`DELETE FROM virtual_nodes WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete virtual node: %w", err)
	}
	return tx.Commit()
}

// ClearAllVirtualState deletes all virtual nodes, manual edges, virtual directories,
// and their annotations in a single transaction.
// Used by the Clear action and before snapshot load to give a clean slate.
func (s *Store) ClearAllVirtualState() error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// Fix P2/3: check every exec so partial cleanup cannot be committed.
	// Fix P1: delete edge_annotations for ALL manual edges (not just virtual-endpoint ones)
	//         so real-real manual edges don't leak remarks onto future indexed edges.
	// Fix P2/5: also clear virtual_directories so the graph is fully reset.

	if _, err = tx.Exec(`DELETE FROM node_annotations WHERE node_id IN (SELECT id FROM virtual_nodes)`); err != nil {
		return fmt.Errorf("delete node_annotations: %w", err)
	}
	// Delete edge_annotations for all manual method_calls, then for any remaining
	// virtual-endpoint edges (e.g. annotations added outside the manual-call path).
	if _, err = tx.Exec(`
		DELETE FROM edge_annotations
		WHERE edge_id IN (
			SELECT caller_id || '::' || callee_id FROM method_calls WHERE call_type = 'manual'
		)
		OR source_node_id IN (SELECT id FROM virtual_nodes)
		OR target_node_id IN (SELECT id FROM virtual_nodes)
	`); err != nil {
		return fmt.Errorf("delete edge_annotations: %w", err)
	}
	if _, err = tx.Exec(`DELETE FROM method_calls WHERE call_type = 'manual'`); err != nil {
		return fmt.Errorf("delete manual edges: %w", err)
	}
	if _, err = tx.Exec(`DELETE FROM virtual_nodes`); err != nil {
		return fmt.Errorf("delete virtual nodes: %w", err)
	}
	// Fix P2/5: clear virtual directories (they show up as graph nodes too).
	if _, err = tx.Exec(`DELETE FROM virtual_directories`); err != nil {
		return fmt.Errorf("delete virtual directories: %w", err)
	}
	return tx.Commit()
}

// GetVirtualNodesByDirectory retrieves all virtual nodes in a virtual directory.
func (s *Store) GetVirtualNodesByDirectory(virtualDirectory string) ([]types.VirtualNode, error) {
	query := `
		SELECT id, label, type, virtual_class_id, virtual_directory, materialized, created_at, updated_at
		FROM virtual_nodes
		WHERE virtual_directory = ?
		ORDER BY label
	`

	rows, err := s.db.Query(query, virtualDirectory)
	if err != nil {
		return nil, fmt.Errorf("query virtual nodes by directory: %w", err)
	}
	defer rows.Close()

	var nodes []types.VirtualNode
	for rows.Next() {
		var node types.VirtualNode
		var virtualClassID sql.NullString
		var virtualDir sql.NullString
		var materialized int

		err := rows.Scan(
			&node.ID,
			&node.Label,
			&node.Type,
			&virtualClassID,
			&virtualDir,
			&materialized,
			&node.CreatedAt,
			&node.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan virtual node: %w", err)
		}

		node.IsVirtual = true
		node.Materialized = materialized == 1
		if virtualClassID.Valid {
			node.VirtualClassID = virtualClassID.String
		}
		if virtualDir.Valid {
			node.VirtualDirectory = virtualDir.String
		}

		nodes = append(nodes, node)
	}

	return nodes, rows.Err()
}

// MarkVirtualNodeMaterialized marks a single virtual node as materialized.
func (s *Store) MarkVirtualNodeMaterialized(id string) error {
	_, err := s.db.Exec(
		"UPDATE virtual_nodes SET materialized = 1, updated_at = ? WHERE id = ?",
		time.Now().Unix(), id,
	)
	return err
}

// MarkMaterializedByFile marks all virtual nodes as materialized whose label
// matches a symbol that was just indexed from filePath.
func (s *Store) MarkMaterializedByFile(filePath string) error {
	_, err := s.db.Exec(`
		UPDATE virtual_nodes
		SET materialized = 1, updated_at = ?
		WHERE materialized = 0
		  AND label IN (SELECT name FROM symbols WHERE file_path = ?)
	`, time.Now().Unix(), filePath)
	return err
}
