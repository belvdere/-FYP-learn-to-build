package db

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"time"

	"example.com/fyp/pkg/config"
	"example.com/fyp/pkg/types"
	_ "github.com/mattn/go-sqlite3"
)

// Store manages the SQLite database connection and operations.
type Store struct {
	db *sql.DB
}

// NewStore creates a new database store.
// It opens the database file and initializes the schema.
func NewStore(workspaceRoot string) (*Store, error) {
	dbPath := config.GetDBPath(workspaceRoot)

	// Ensure .fyp directory exists
	if err := config.EnsureDir(filepath.Join(workspaceRoot, ".fyp")); err != nil {
		return nil, fmt.Errorf("create .fyp dir: %w", err)
	}

	// Use DSN-level SQLite settings so every pooled connection is configured
	// consistently (WAL, FK enforcement, and busy timeout).
	dsn := fmt.Sprintf("%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on", dbPath)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	// SQLite is safest with a single connection per process for mixed
	// read/write transactional workloads. This avoids pool-level cross-connection
	// lock/readonly races during concurrent indexing + graph operations.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	store := &Store{db: db}

	// Initialize schema
	if err := InitSchema(store); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return store, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB returns the underlying database connection for direct access (used for debugging).
func (s *Store) DB() *sql.DB {
	return s.db
}

// GetAllIndexedFilePaths returns file paths that currently have indexed data in the DB.
// Used to prune stale data for deleted/moved files during indexing.
func (s *Store) GetAllIndexedFilePaths() ([]string, error) {
	query := `
		SELECT DISTINCT file_path FROM (
			SELECT file_path FROM symbols
			UNION
			SELECT file_path FROM file_cache
		)
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query indexed file paths: %w", err)
	}
	defer rows.Close()

	var files []string
	for rows.Next() {
		var fp string
		if err := rows.Scan(&fp); err != nil {
			return nil, fmt.Errorf("scan indexed file path: %w", err)
		}
		if fp != "" {
			files = append(files, fp)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate indexed file paths: %w", err)
	}
	return files, nil
}

// ============================================================================
// Symbol operations
// ============================================================================

// InsertSymbol inserts a new symbol into the database.
func (s *Store) InsertSymbol(symbol types.Symbol) error {
	now := time.Now().Unix()

	query := `
		INSERT OR REPLACE INTO symbols 
		(id, name, kind, file_path, line, end_line, signature, body_hash, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(query,
		symbol.ID,
		symbol.Name,
		symbol.Kind,
		symbol.FilePath,
		symbol.Line,
		symbol.EndLine,
		symbol.Signature,
		symbol.BodyHash,
		now,
	)

	return err
}

// GetAllSymbolNames returns a set of all symbol names in the database.
// Used by fast mode to filter calls against known project symbols.
func (s *Store) GetAllSymbolNames() (map[string]bool, error) {
	rows, err := s.db.Query(`SELECT name FROM symbols`)
	if err != nil {
		return nil, fmt.Errorf("query symbol names: %w", err)
	}
	defer rows.Close()

	names := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan symbol name: %w", err)
		}
		names[name] = true
	}
	return names, rows.Err()
}

// GetAllSymbols retrieves all symbols from the database.
func (s *Store) GetAllSymbols() ([]types.Symbol, error) {
	query := `
		SELECT id, name, kind, file_path, line, end_line, signature, body_hash
		FROM symbols
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query symbols: %w", err)
	}
	defer rows.Close()

	var symbols []types.Symbol
	for rows.Next() {
		var sym types.Symbol
		var signature sql.NullString
		var bodyHash sql.NullString

		err := rows.Scan(
			&sym.ID,
			&sym.Name,
			&sym.Kind,
			&sym.FilePath,
			&sym.Line,
			&sym.EndLine,
			&signature,
			&bodyHash,
		)
		if err != nil {
			return nil, fmt.Errorf("scan symbol: %w", err)
		}

		if signature.Valid {
			sym.Signature = signature.String
		}
		if bodyHash.Valid {
			sym.BodyHash = bodyHash.String
		}

		symbols = append(symbols, sym)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate symbols: %w", err)
	}

	return symbols, nil
}

// DeleteSymbolsByFile deletes all symbols for a given file.
func (s *Store) DeleteSymbolsByFile(filePath string) error {
	query := `DELETE FROM symbols WHERE file_path = ?`
	_, err := s.db.Exec(query, filePath)
	return err
}

// GetSymbolsByFile retrieves all symbols for a given file path.
func (s *Store) GetSymbolsByFile(filePath string) ([]types.Symbol, error) {
	query := `
		SELECT id, name, kind, file_path, line, end_line, signature, body_hash
		FROM symbols
		WHERE file_path = ?
		ORDER BY line
	`

	rows, err := s.db.Query(query, filePath)
	if err != nil {
		return nil, fmt.Errorf("query symbols by file: %w", err)
	}
	defer rows.Close()

	var symbols []types.Symbol
	for rows.Next() {
		var sym types.Symbol
		var signature sql.NullString
		var bodyHash sql.NullString

		if err := rows.Scan(
			&sym.ID,
			&sym.Name,
			&sym.Kind,
			&sym.FilePath,
			&sym.Line,
			&sym.EndLine,
			&signature,
			&bodyHash,
		); err != nil {
			return nil, fmt.Errorf("scan symbol by file: %w", err)
		}
		if signature.Valid {
			sym.Signature = signature.String
		}
		if bodyHash.Valid {
			sym.BodyHash = bodyHash.String
		}
		symbols = append(symbols, sym)
	}

	return symbols, rows.Err()
}

// ============================================================================
// Data cleanup operations
// ============================================================================

// DeleteAllDataByFile deletes all indexed data for a given file.
// This is used before re-indexing to prevent duplicate data.
func (s *Store) DeleteAllDataByFile(filePath string) error {
	// Delete method calls
	if err := s.DeleteMethodCallsByFile(filePath); err != nil {
		return fmt.Errorf("delete method calls: %w", err)
	}

	// Delete symbols
	if err := s.DeleteSymbolsByFile(filePath); err != nil {
		return fmt.Errorf("delete symbols: %w", err)
	}

	return nil
}

// ClearAllData deletes all indexed data from all tables.
// Used by RebuildIndex to start fresh.
// Preserves manually-created edges (those with empty file_path).
func (s *Store) ClearAllData() error {
	// Clear indexed method calls (preserve manual edges with empty file_path)
	if _, err := s.db.Exec("DELETE FROM method_calls WHERE file_path != ''"); err != nil {
		return fmt.Errorf("clear method_calls: %w", err)
	}

	// Clear all symbols
	if _, err := s.db.Exec("DELETE FROM symbols"); err != nil {
		return fmt.Errorf("clear symbols: %w", err)
	}

	return nil
}

// ClearAllMethodCalls deletes all indexed method calls from the database.
// Preserves manually-created edges (those with empty file_path) so user-created
// relationships survive full re-indexing.
func (s *Store) ClearAllMethodCalls() error {
	_, err := s.db.Exec("DELETE FROM method_calls WHERE file_path != ''")
	return err
}

// ReplaceIndexData replaces indexed symbols and edges atomically.
// If rebuild is true, all existing symbols and edges are cleared first.
// If rebuild is false, manual edges are preserved.
func (s *Store) ReplaceIndexData(symbols []types.Symbol, calls []types.MethodCall, rebuild bool) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if rebuild {
		if _, err = tx.Exec("DELETE FROM method_calls"); err != nil {
			return fmt.Errorf("clear method_calls: %w", err)
		}
		if _, err = tx.Exec("DELETE FROM symbols"); err != nil {
			return fmt.Errorf("clear symbols: %w", err)
		}
	} else {
		// Preserve user-created edges across normal index refreshes.
		if _, err = tx.Exec("DELETE FROM method_calls WHERE call_type != 'manual'"); err != nil {
			return fmt.Errorf("clear indexed method_calls: %w", err)
		}
		if _, err = tx.Exec("DELETE FROM symbols"); err != nil {
			return fmt.Errorf("clear symbols: %w", err)
		}
	}

	symbolStmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO symbols
		(id, name, kind, file_path, line, end_line, signature, body_hash, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare symbol stmt: %w", err)
	}
	defer symbolStmt.Close()

	now := time.Now().Unix()
	for _, sym := range symbols {
		if _, err = symbolStmt.Exec(sym.ID, sym.Name, sym.Kind, sym.FilePath, sym.Line, sym.EndLine, sym.Signature, sym.BodyHash, now); err != nil {
			return fmt.Errorf("insert symbol: %w", err)
		}
	}

	callStmt, err := tx.Prepare(`
		INSERT INTO method_calls
		(caller_id, callee_id, call_type, file_path, line, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare call stmt: %w", err)
	}
	defer callStmt.Close()

	for _, call := range calls {
		if _, err = callStmt.Exec(call.CallerID, call.CalleeID, call.CallType, call.FilePath, call.Line, now); err != nil {
			return fmt.Errorf("insert method call: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// BulkInsertEdges appends a batch of method calls to the database without
// touching symbols or deleting existing edges. Used for chunked edge storage.
func (s *Store) BulkInsertEdges(calls []types.MethodCall) error {
	if len(calls) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	stmt, err := tx.Prepare(`
		INSERT INTO method_calls
		(caller_id, callee_id, call_type, file_path, line, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare stmt: %w", err)
	}
	defer stmt.Close()

	now := time.Now().Unix()
	for _, call := range calls {
		if _, err = stmt.Exec(call.CallerID, call.CalleeID, call.CallType, call.FilePath, call.Line, now); err != nil {
			return fmt.Errorf("insert edge: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
