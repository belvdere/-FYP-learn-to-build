package db

import (
	"fmt"
	"strings"
	"time"

	"example.com/fyp/pkg/types"
)

// ResolveSymbolIDsByLocations resolves source locations to canonical symbol IDs.
func (s *Store) ResolveSymbolIDsByLocations(locations []types.SymbolLocation) ([]types.ResolvedLocation, error) {
	results := make([]types.ResolvedLocation, 0, len(locations))
	for _, loc := range locations {
		resolved := types.ResolvedLocation{
			FilePath: loc.FilePath,
			Line:     loc.Line,
		}

		if loc.FilePath == "" || loc.Line <= 0 {
			results = append(results, resolved)
			continue
		}

		var symbolID string
		err := s.db.QueryRow(`
			SELECT id
			FROM symbols
			WHERE file_path = ?
			  AND line <= ?
			  AND (end_line = 0 OR end_line >= ?)
			ORDER BY CASE WHEN line = ? THEN 0 ELSE 1 END, ABS(line - ?) ASC
			LIMIT 1
		`, loc.FilePath, loc.Line, loc.Line, loc.Line, loc.Line).Scan(&symbolID)
		if err == nil {
			resolved.SymbolID = symbolID
		}

		results = append(results, resolved)
	}

	return results, nil
}

// ApplyFileDelta applies an on-save incremental update for a single file.
func (s *Store) ApplyFileDelta(
	filePath string,
	fileHash string,
	symbols []types.Symbol,
	changedCallerIDs []string,
	removedCallerIDs []string,
	calls []types.MethodCall,
) (err error) {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	now := time.Now().Unix()

	symbolStmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO symbols
		(id, name, kind, file_path, line, end_line, signature, body_hash, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare symbol upsert: %w", err)
	}
	defer symbolStmt.Close()

	newIDs := make([]string, 0, len(symbols))
	for _, sym := range symbols {
		symFilePath := sym.FilePath
		if symFilePath == "" {
			symFilePath = filePath
		}
		if _, err = symbolStmt.Exec(
			sym.ID,
			sym.Name,
			sym.Kind,
			symFilePath,
			sym.Line,
			sym.EndLine,
			sym.Signature,
			sym.BodyHash,
			now,
		); err != nil {
			return fmt.Errorf("upsert symbol: %w", err)
		}
		newIDs = append(newIDs, sym.ID)
	}

	if len(newIDs) == 0 {
		if _, err = tx.Exec(`DELETE FROM symbols WHERE file_path = ?`, filePath); err != nil {
			return fmt.Errorf("delete file symbols: %w", err)
		}
	} else {
		placeholders := make([]string, len(newIDs))
		args := make([]interface{}, 0, len(newIDs)+1)
		args = append(args, filePath)
		for i, id := range newIDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		q := fmt.Sprintf(`DELETE FROM symbols WHERE file_path = ? AND id NOT IN (%s)`, strings.Join(placeholders, ","))
		if _, err = tx.Exec(q, args...); err != nil {
			return fmt.Errorf("delete stale file symbols: %w", err)
		}
	}

	var where []string
	args := make([]interface{}, 0, len(changedCallerIDs)+len(removedCallerIDs)*2)
	if len(changedCallerIDs) > 0 {
		where = append(where, "caller_id IN ("+placeholderList(len(changedCallerIDs))+")")
		for _, id := range changedCallerIDs {
			args = append(args, id)
		}
	}
	if len(removedCallerIDs) > 0 {
		where = append(where, "caller_id IN ("+placeholderList(len(removedCallerIDs))+")")
		for _, id := range removedCallerIDs {
			args = append(args, id)
		}
		where = append(where, "callee_id IN ("+placeholderList(len(removedCallerIDs))+")")
		for _, id := range removedCallerIDs {
			args = append(args, id)
		}
	}

	if len(where) > 0 {
		deleteQuery := "DELETE FROM method_calls WHERE call_type != 'manual' AND (" + strings.Join(where, " OR ") + ")"
		if _, err = tx.Exec(deleteQuery, args...); err != nil {
			return fmt.Errorf("delete stale method calls: %w", err)
		}
	}

	callStmt, err := tx.Prepare(`
		INSERT INTO method_calls
		(caller_id, callee_id, call_type, file_path, line, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare call insert: %w", err)
	}
	defer callStmt.Close()

	for _, call := range calls {
		if _, err = callStmt.Exec(call.CallerID, call.CalleeID, call.CallType, call.FilePath, call.Line, now); err != nil {
			return fmt.Errorf("insert file method call: %w", err)
		}
	}

	if filePath != "" && fileHash != "" {
		if _, err = tx.Exec(`
			INSERT OR REPLACE INTO file_cache (file_path, file_hash, last_indexed)
			VALUES (?, ?, ?)
		`, filePath, fileHash, now); err != nil {
			return fmt.Errorf("upsert file cache: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func placeholderList(n int) string {
	if n <= 0 {
		return ""
	}
	p := make([]string, n)
	for i := range p {
		p[i] = "?"
	}
	return strings.Join(p, ",")
}
