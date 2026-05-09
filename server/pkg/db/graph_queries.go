package db

import (
	"database/sql"
	"fmt"
	"strings"

	"example.com/fyp/pkg/types"
)

// virtualNodeFilePathSQL is the SQL expression that constructs a file_path for virtual nodes.
// It groups virtual nodes by class name (part before the first dot in label)
// so e.g. "Foo.bar" and "Foo.baz" share "virtual/Foo.virtual".
const virtualNodeFilePathSQL = `
	CASE
		WHEN COALESCE(virtual_directory, '') = '' THEN
			'virtual/' ||
			CASE WHEN INSTR(label, '.') > 0
				THEN SUBSTR(label, 1, INSTR(label, '.') - 1)
				ELSE label
			END || '.virtual'
		ELSE
			virtual_directory || '/' ||
			CASE WHEN INSTR(label, '.') > 0
				THEN SUBSTR(label, 1, INSTR(label, '.') - 1)
				ELSE label
			END || '.virtual'
	END`

// extractClassName returns the class name from a qualified label.
// e.g. "UserService.createUser" → "UserService", "helloWorld" → "helloWorld"
func extractClassName(label string) string {
	if idx := strings.Index(label, "."); idx > 0 {
		return label[:idx]
	}
	return label
}

// GetAllGraphNodes retrieves all nodes (real symbols + virtual nodes) for graph display.
// Virtual nodes are grouped by class name (part before the dot in the label).
// If a virtual node's class matches an existing real class, it is placed in that class's file.
func (s *Store) GetAllGraphNodes() ([]types.GraphNode, error) {
	query := `
		SELECT id, name as label, kind as type, file_path, line, 0 as is_virtual, 0 as materialized,
			'' as virtual_class_id, '' as virtual_directory
		FROM symbols
		UNION ALL
		SELECT id, label, type, ` + virtualNodeFilePathSQL + ` as file_path,
			0 as line, 1 as is_virtual, materialized,
			COALESCE(virtual_class_id, '') as virtual_class_id,
			COALESCE(virtual_directory, '') as virtual_directory
		FROM virtual_nodes
		WHERE NOT (materialized = 1 AND label IN (SELECT name FROM symbols))
		ORDER BY label
	`

	// Pre-fetch all annotations in one query to avoid N+1 per node
	annotationMap, err := s.GetAllNodeAnnotationsMap()
	if err != nil {
		annotationMap = map[string]*types.NodeAnnotation{} // non-fatal; continue without annotations
	}

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query graph nodes: %w", err)
	}
	defer rows.Close()

	var nodes []types.GraphNode
	for rows.Next() {
		var node types.GraphNode
		var filePath sql.NullString
		var line sql.NullInt64
		var isVirtual int
		var materialized int

		err := rows.Scan(
			&node.ID,
			&node.Label,
			&node.Type,
			&filePath,
			&line,
			&isVirtual,
			&materialized,
			&node.VirtualClassID,
			&node.VirtualDirectory,
		)
		if err != nil {
			return nil, fmt.Errorf("scan graph node: %w", err)
		}

		if filePath.Valid {
			node.FilePath = filePath.String
		}
		if line.Valid {
			node.Line = int(line.Int64)
		}
		node.IsVirtual = isVirtual == 1
		node.Materialized = materialized == 1
		node.Annotation = annotationMap[node.ID]

		nodes = append(nodes, node)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Post-process: if a virtual node's class matches a real class, use that file path.
	// This groups virtual methods like "AdminUserController.newEndpoint" under the real
	// AdminUserController.java file instead of a separate virtual file.
	classFileMap := make(map[string]string)
	for _, node := range nodes {
		if !node.IsVirtual && node.FilePath != "" {
			className := extractClassName(node.Label)
			if className != "" {
				classFileMap[className] = node.FilePath
			}
		}
	}
	for i := range nodes {
		if nodes[i].IsVirtual {
			className := extractClassName(nodes[i].Label)
			if realFilePath, exists := classFileMap[className]; exists {
				nodes[i].FilePath = realFilePath
			}
		}
	}

	return nodes, nil
}

// GetGraphNode retrieves a single node (real or virtual) by ID with annotation.
func (s *Store) GetGraphNode(nodeID string) (*types.GraphNode, error) {
	// Try to find in symbols first
	symbolQuery := `
		SELECT id, name as label, kind as type, file_path, line
		FROM symbols
		WHERE id = ?
	`

	var node types.GraphNode
	err := s.db.QueryRow(symbolQuery, nodeID).Scan(
		&node.ID,
		&node.Label,
		&node.Type,
		&node.FilePath,
		&node.Line,
	)

	if err == nil {
		node.IsVirtual = false
		// Get annotation if exists
		annotation, _ := s.GetNodeAnnotation(nodeID)
		node.Annotation = annotation
		return &node, nil
	}

	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("query symbol: %w", err)
	}

	// Not found in symbols, try virtual_nodes
	virtualQuery := `
		SELECT id, label, type, ` + virtualNodeFilePathSQL + ` as file_path, materialized
		FROM virtual_nodes
		WHERE id = ?
	`

	var filePath sql.NullString
	var materialized int
	err = s.db.QueryRow(virtualQuery, nodeID).Scan(
		&node.ID,
		&node.Label,
		&node.Type,
		&filePath,
		&materialized,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("node not found: %s", nodeID)
		}
		return nil, fmt.Errorf("query virtual node: %w", err)
	}

	if filePath.Valid {
		node.FilePath = filePath.String
	}
	node.IsVirtual = true
	node.Materialized = materialized == 1
	node.Line = 0

	// If the virtual node's class matches a real class, use that file path
	className := extractClassName(node.Label)
	var realFilePath string
	lookupErr := s.db.QueryRow(
		"SELECT file_path FROM symbols WHERE name LIKE ? LIMIT 1",
		className+".%",
	).Scan(&realFilePath)
	if lookupErr == nil && realFilePath != "" {
		node.FilePath = realFilePath
	}

	// Get annotation if exists
	annotation, _ := s.GetNodeAnnotation(nodeID)
	node.Annotation = annotation

	return &node, nil
}

// GetNodeNeighbors returns all nodes connected to a given node via edges.
func (s *Store) GetNodeNeighbors(nodeID string) ([]types.GraphNode, error) {
	// Get neighbors from both method_calls and edge_annotations tables
	query := `
		SELECT DISTINCT neighbor_id FROM (
			SELECT CASE 
				WHEN caller_id = ? THEN callee_id
				ELSE caller_id
			END as neighbor_id
			FROM method_calls
			WHERE caller_id = ? OR callee_id = ?
			UNION
			SELECT CASE 
				WHEN source_node_id = ? THEN target_node_id
				ELSE source_node_id
			END as neighbor_id
			FROM edge_annotations
			WHERE source_node_id = ? OR target_node_id = ?
		)
	`

	rows, err := s.db.Query(query, nodeID, nodeID, nodeID, nodeID, nodeID, nodeID)
	if err != nil {
		return nil, fmt.Errorf("query neighbors: %w", err)
	}

	var neighborIDs []string
	for rows.Next() {
		var neighborID string
		if err := rows.Scan(&neighborID); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan neighbor ID: %w", err)
		}
		neighborIDs = append(neighborIDs, neighborID)
	}

	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterate neighbors: %w", err)
	}
	// Explicitly close rows before nested DB lookups (GetGraphNode) to avoid
	// single-connection SQLite stalls when MaxOpenConns=1.
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close neighbor rows: %w", err)
	}

	// Fetch full node data for each neighbor
	var neighbors []types.GraphNode
	for _, neighborID := range neighborIDs {
		node, err := s.GetGraphNode(neighborID)
		if err != nil {
			// Skip nodes that can't be found (might be external/unindexed)
			continue
		}
		neighbors = append(neighbors, *node)
	}

	return neighbors, nil
}

// SearchGraphNodes searches for nodes by name/label (case-insensitive).
func (s *Store) SearchGraphNodes(query string) ([]types.GraphNode, error) {
	searchPattern := "%" + query + "%"

	sqlQuery := `
		SELECT id, name as label, kind as type, file_path, line, 0 as is_virtual,
			'' as virtual_class_id, '' as virtual_directory
		FROM symbols
		WHERE name LIKE ? COLLATE NOCASE
		UNION ALL
		SELECT id, label, type, ` + virtualNodeFilePathSQL + ` as file_path,
			0 as line, 1 as is_virtual,
			COALESCE(virtual_class_id, '') as virtual_class_id,
			COALESCE(virtual_directory, '') as virtual_directory
		FROM virtual_nodes
		WHERE label LIKE ? COLLATE NOCASE
		ORDER BY label
		LIMIT 50
	`

	// Pre-fetch all annotations in one query to avoid N+1 per result
	annotationMap, err := s.GetAllNodeAnnotationsMap()
	if err != nil {
		annotationMap = map[string]*types.NodeAnnotation{}
	}

	rows, err := s.db.Query(sqlQuery, searchPattern, searchPattern)
	if err != nil {
		return nil, fmt.Errorf("search graph nodes: %w", err)
	}
	defer rows.Close()

	var nodes []types.GraphNode
	for rows.Next() {
		var node types.GraphNode
		var filePath sql.NullString
		var line sql.NullInt64
		var isVirtual int

		err := rows.Scan(
			&node.ID,
			&node.Label,
			&node.Type,
			&filePath,
			&line,
			&isVirtual,
			&node.VirtualClassID,
			&node.VirtualDirectory,
		)
		if err != nil {
			return nil, fmt.Errorf("scan graph node: %w", err)
		}

		if filePath.Valid {
			node.FilePath = filePath.String
		}
		if line.Valid {
			node.Line = int(line.Int64)
		}
		node.IsVirtual = isVirtual == 1
		node.Annotation = annotationMap[node.ID]

		nodes = append(nodes, node)
	}

	return nodes, rows.Err()
}

// GetGraphNodesByFile retrieves all nodes (methods) in a specific file.
func (s *Store) GetGraphNodesByFile(filePath string) ([]types.GraphNode, error) {
	// Pre-fetch all annotations to avoid N+1 per symbol.
	// Must happen before opening rows to avoid nested-connection waits when
	// SQLite runs with a single pooled connection.
	annotationMap, err := s.GetAllNodeAnnotationsMap()
	if err != nil {
		annotationMap = map[string]*types.NodeAnnotation{}
	}

	// Get real symbols from file
	symbolQuery := `
		SELECT id, name as label, kind as type, file_path, line
		FROM symbols
		WHERE file_path = ?
		ORDER BY line
	`

	rows, err := s.db.Query(symbolQuery, filePath)
	if err != nil {
		return nil, fmt.Errorf("query symbols by file: %w", err)
	}
	defer rows.Close()

	var nodes []types.GraphNode
	for rows.Next() {
		var node types.GraphNode
		err := rows.Scan(
			&node.ID,
			&node.Label,
			&node.Type,
			&node.FilePath,
			&node.Line,
		)
		if err != nil {
			return nil, fmt.Errorf("scan symbol: %w", err)
		}

		node.IsVirtual = false
		node.Annotation = annotationMap[node.ID]

		nodes = append(nodes, node)
	}

	return nodes, rows.Err()
}

// CountGraphNodes returns the total count of nodes (real + virtual).
func (s *Store) CountGraphNodes() (int, error) {
	var count int

	query := `
		SELECT
			(SELECT COUNT(*) FROM symbols) +
			(SELECT COUNT(*) FROM virtual_nodes
			 WHERE NOT (materialized = 1 AND label IN (SELECT name FROM symbols))) as total
	`

	err := s.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count graph nodes: %w", err)
	}

	return count, nil
}
