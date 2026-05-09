package db

import (
	"fmt"
	"time"

	"example.com/fyp/pkg/types"
)

// InsertMethodCall stores a method call relationship.
func (s *Store) InsertMethodCall(call types.MethodCall) error {
	query := `
		INSERT INTO method_calls 
		(caller_id, callee_id, call_type, file_path, line, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(query,
		call.CallerID,
		call.CalleeID,
		call.CallType,
		call.FilePath,
		call.Line,
		time.Now().Unix(),
	)

	return err
}

// GetAllMethodCalls retrieves all method calls from the database.
func (s *Store) GetAllMethodCalls() ([]types.MethodCall, error) {
	query := `
		SELECT id, caller_id, callee_id, call_type, file_path, line
		FROM method_calls
		ORDER BY file_path, line
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query method calls: %w", err)
	}
	defer rows.Close()

	var calls []types.MethodCall
	for rows.Next() {
		var call types.MethodCall
		err := rows.Scan(
			&call.ID,
			&call.CallerID,
			&call.CalleeID,
			&call.CallType,
			&call.FilePath,
			&call.Line,
		)
		if err != nil {
			return nil, fmt.Errorf("scan method call: %w", err)
		}
		calls = append(calls, call)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate method calls: %w", err)
	}

	return calls, nil
}

// DeleteMethodCallsByFile removes all method calls from a specific file.
// Used when re-indexing a file.
func (s *Store) DeleteMethodCallsByFile(filePath string) error {
	query := `DELETE FROM method_calls WHERE file_path = ?`
	_, err := s.db.Exec(query, filePath)
	return err
}

// DeleteManualMethodCall removes a user-created (manual) edge from method_calls.
// Used when the user deletes an edge they created via the graph UI.
// Only deletes manual edges; indexed edges from code analysis are preserved.
func (s *Store) DeleteManualMethodCall(callerSymbol, calleeSymbol string) error {
	query := `DELETE FROM method_calls WHERE caller_id = ? AND callee_id = ? AND call_type = 'manual'`
	_, err := s.db.Exec(query, callerSymbol, calleeSymbol)
	return err
}
