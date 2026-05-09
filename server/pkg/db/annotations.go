package db

import (
	"database/sql"
	"fmt"
	"time"

	"example.com/fyp/pkg/types"
)

// UpsertNodeAnnotation inserts or updates a node annotation.
func (s *Store) UpsertNodeAnnotation(annotation types.NodeAnnotation) error {
	now := time.Now().Unix()

	query := `
		INSERT INTO node_annotations
		(node_id, description, ai_remarks, code_snippet, custom_properties, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(node_id) DO UPDATE SET
			description = excluded.description,
			ai_remarks = excluded.ai_remarks,
			code_snippet = excluded.code_snippet,
			custom_properties = excluded.custom_properties,
			updated_at = excluded.updated_at
	`

	_, err := s.db.Exec(query,
		annotation.NodeID,
		annotation.Description,
		annotation.AIRemarks,
		annotation.CodeSnippet,
		annotation.CustomProperties,
		now,
		now,
	)

	return err
}

// GetNodeAnnotation retrieves an annotation for a node.
func (s *Store) GetNodeAnnotation(nodeID string) (*types.NodeAnnotation, error) {
	query := `
		SELECT node_id, description, ai_remarks, code_snippet, custom_properties, created_at, updated_at
		FROM node_annotations
		WHERE node_id = ?
	`

	var annotation types.NodeAnnotation
	var description sql.NullString
	var aiRemarks sql.NullString
	var codeSnippet sql.NullString
	var customProps sql.NullString

	err := s.db.QueryRow(query, nodeID).Scan(
		&annotation.NodeID,
		&description,
		&aiRemarks,
		&codeSnippet,
		&customProps,
		&annotation.CreatedAt,
		&annotation.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No annotation found, not an error
		}
		return nil, err
	}

	if description.Valid {
		annotation.Description = description.String
	}
	if aiRemarks.Valid {
		annotation.AIRemarks = aiRemarks.String
	}
	if codeSnippet.Valid {
		annotation.CodeSnippet = codeSnippet.String
	}
	if customProps.Valid {
		annotation.CustomProperties = customProps.String
	}

	return &annotation, nil
}

// GetAllNodeAnnotations retrieves all node annotations.
func (s *Store) GetAllNodeAnnotations() ([]types.NodeAnnotation, error) {
	query := `
		SELECT node_id, description, ai_remarks, code_snippet, custom_properties, created_at, updated_at
		FROM node_annotations
		ORDER BY updated_at DESC
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query node annotations: %w", err)
	}
	defer rows.Close()

	var annotations []types.NodeAnnotation
	for rows.Next() {
		var annotation types.NodeAnnotation
		var description sql.NullString
		var aiRemarks sql.NullString
		var codeSnippet sql.NullString
		var customProps sql.NullString

		err := rows.Scan(
			&annotation.NodeID,
			&description,
			&aiRemarks,
			&codeSnippet,
			&customProps,
			&annotation.CreatedAt,
			&annotation.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan node annotation: %w", err)
		}

		if description.Valid {
			annotation.Description = description.String
		}
		if aiRemarks.Valid {
			annotation.AIRemarks = aiRemarks.String
		}
		if codeSnippet.Valid {
			annotation.CodeSnippet = codeSnippet.String
		}
		if customProps.Valid {
			annotation.CustomProperties = customProps.String
		}

		annotations = append(annotations, annotation)
	}

	return annotations, rows.Err()
}

// GetAllNodeAnnotationsMap retrieves all node annotations as a map keyed by node ID.
// Use this for bulk lookups to avoid N+1 queries.
func (s *Store) GetAllNodeAnnotationsMap() (map[string]*types.NodeAnnotation, error) {
	annotations, err := s.GetAllNodeAnnotations()
	if err != nil {
		return nil, err
	}
	m := make(map[string]*types.NodeAnnotation, len(annotations))
	for i := range annotations {
		m[annotations[i].NodeID] = &annotations[i]
	}
	return m, nil
}

// DeleteNodeAnnotation deletes an annotation for a node.
// This operation is idempotent - it succeeds even if the annotation doesn't exist.
func (s *Store) DeleteNodeAnnotation(nodeID string) error {
	query := `DELETE FROM node_annotations WHERE node_id = ?`

	_, err := s.db.Exec(query, nodeID)
	return err
}

// UpsertEdgeAnnotation inserts or updates an edge annotation.
func (s *Store) UpsertEdgeAnnotation(annotation types.EdgeAnnotation) error {
	now := time.Now().Unix()

	query := `
		INSERT INTO edge_annotations
		(edge_id, source_node_id, target_node_id, remarks, call_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(edge_id) DO UPDATE SET
			remarks = excluded.remarks,
			call_count = excluded.call_count,
			updated_at = excluded.updated_at
	`

	_, err := s.db.Exec(query,
		annotation.EdgeID,
		annotation.SourceNodeID,
		annotation.TargetNodeID,
		annotation.Remarks,
		annotation.CallCount,
		now,
		now,
	)

	return err
}

// GetEdgeAnnotation retrieves an annotation for an edge.
func (s *Store) GetEdgeAnnotation(edgeID string) (*types.EdgeAnnotation, error) {
	query := `
		SELECT edge_id, source_node_id, target_node_id, remarks, call_count, created_at, updated_at
		FROM edge_annotations
		WHERE edge_id = ?
	`

	var annotation types.EdgeAnnotation
	var remarks sql.NullString

	err := s.db.QueryRow(query, edgeID).Scan(
		&annotation.EdgeID,
		&annotation.SourceNodeID,
		&annotation.TargetNodeID,
		&remarks,
		&annotation.CallCount,
		&annotation.CreatedAt,
		&annotation.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No annotation found, not an error
		}
		return nil, err
	}

	if remarks.Valid {
		annotation.Remarks = remarks.String
	}

	return &annotation, nil
}

// GetEdgeAnnotationByNodes retrieves an annotation for an edge by source and target nodes.
func (s *Store) GetEdgeAnnotationByNodes(sourceNodeID, targetNodeID string) (*types.EdgeAnnotation, error) {
	query := `
		SELECT edge_id, source_node_id, target_node_id, remarks, call_count, created_at, updated_at
		FROM edge_annotations
		WHERE (source_node_id = ? AND target_node_id = ?)
		   OR (source_node_id = ? AND target_node_id = ?)
	`

	var annotation types.EdgeAnnotation
	var remarks sql.NullString

	err := s.db.QueryRow(query, sourceNodeID, targetNodeID, targetNodeID, sourceNodeID).Scan(
		&annotation.EdgeID,
		&annotation.SourceNodeID,
		&annotation.TargetNodeID,
		&remarks,
		&annotation.CallCount,
		&annotation.CreatedAt,
		&annotation.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No annotation found, not an error
		}
		return nil, err
	}

	if remarks.Valid {
		annotation.Remarks = remarks.String
	}

	return &annotation, nil
}

// GetAllEdgeAnnotations retrieves all edge annotations.
func (s *Store) GetAllEdgeAnnotations() ([]types.EdgeAnnotation, error) {
	query := `
		SELECT edge_id, source_node_id, target_node_id, remarks, call_count, created_at, updated_at
		FROM edge_annotations
		ORDER BY updated_at DESC
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query edge annotations: %w", err)
	}
	defer rows.Close()

	var annotations []types.EdgeAnnotation
	for rows.Next() {
		var annotation types.EdgeAnnotation
		var remarks sql.NullString

		err := rows.Scan(
			&annotation.EdgeID,
			&annotation.SourceNodeID,
			&annotation.TargetNodeID,
			&remarks,
			&annotation.CallCount,
			&annotation.CreatedAt,
			&annotation.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan edge annotation: %w", err)
		}

		if remarks.Valid {
			annotation.Remarks = remarks.String
		}

		annotations = append(annotations, annotation)
	}

	return annotations, rows.Err()
}

// DeleteEdgeAnnotation deletes an annotation for an edge.
// This operation is idempotent - it succeeds even if the annotation doesn't exist.
func (s *Store) DeleteEdgeAnnotation(edgeID string) error {
	query := `DELETE FROM edge_annotations WHERE edge_id = ?`

	_, err := s.db.Exec(query, edgeID)
	return err
}
