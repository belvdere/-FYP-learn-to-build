package db

import (
	"database/sql"
)

const schema = `
-- Symbols table: stores code symbols (classes, methods, constructors)
CREATE TABLE IF NOT EXISTS symbols (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    kind TEXT NOT NULL,
    file_path TEXT NOT NULL,
    line INTEGER NOT NULL,
    end_line INTEGER DEFAULT 0,
    signature TEXT,
    body_hash TEXT DEFAULT '',
    created_at INTEGER NOT NULL
);

-- Method calls table: stores method invocation relationships
CREATE TABLE IF NOT EXISTS method_calls (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    caller_id TEXT NOT NULL,        -- Qualified name of calling method (e.g., "Controller.handleRequest")
    callee_id TEXT NOT NULL,        -- Name of called method (e.g., "Service.save")
    call_type TEXT NOT NULL,            -- 'direct', 'static', 'super', 'constructor'
    file_path TEXT NOT NULL,            -- File where the call occurs
    line INTEGER NOT NULL,              -- Line number of the call
    created_at INTEGER NOT NULL
);

-- File cache: stores file hashes for incremental indexing
CREATE TABLE IF NOT EXISTS file_cache (
    file_path TEXT PRIMARY KEY,
    file_hash TEXT NOT NULL,           -- SHA256 hash of file content
    last_indexed INTEGER NOT NULL      -- Timestamp of last indexing
);

-- Virtual nodes table: user-created nodes not tied to actual code
CREATE TABLE IF NOT EXISTS virtual_nodes (
    id TEXT PRIMARY KEY,
    label TEXT NOT NULL,
    type TEXT NOT NULL,              -- 'method', 'class', 'concept', 'module'
    virtual_class_id TEXT,           -- ID of parent virtual class node (virtual_nodes.id where type='class')
    virtual_directory TEXT,          -- Virtual directory path (for virtual class nodes)
    is_virtual INTEGER DEFAULT 1,    -- Always 1 for this table (SQLite uses INTEGER for boolean)
    materialized INTEGER DEFAULT 0,  -- 1 = real code has been generated for this node
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

-- Node annotations table: descriptions and AI remarks for ANY node (real or virtual)
CREATE TABLE IF NOT EXISTS node_annotations (
    node_id TEXT PRIMARY KEY,        -- References either symbols.id or virtual_nodes.id
    description TEXT,                -- User description of node
    ai_remarks TEXT,                 -- AI-specific notes/instructions
    code_snippet TEXT,               -- Optional code snippet for virtual nodes
    custom_properties TEXT,          -- JSON field for additional metadata
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

-- Edge annotations table: remarks for edges
CREATE TABLE IF NOT EXISTS edge_annotations (
    edge_id TEXT PRIMARY KEY,        -- Composite: source_id + target_id
    source_node_id TEXT NOT NULL,
    target_node_id TEXT NOT NULL,
    remarks TEXT,                    -- User notes about this relationship
    call_count INTEGER DEFAULT 1,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

-- Virtual directories table: user-created directory containers
CREATE TABLE IF NOT EXISTS virtual_directories (
    id TEXT PRIMARY KEY,
    path TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    parent_path TEXT,
    created_at INTEGER NOT NULL
);

-- Mask sessions table: stores masking sessions for context-aware validation
CREATE TABLE IF NOT EXISTS mask_sessions (
    id TEXT PRIMARY KEY,              -- UUID
    file_path TEXT NOT NULL,
    original_code TEXT NOT NULL,
    masked_code TEXT NOT NULL,
    filled_code TEXT,
    language TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    status TEXT NOT NULL              -- 'pending', 'filled', 'validated'
);

-- Snapshots table: captures graph state for agentic code generation
CREATE TABLE IF NOT EXISTS snapshots (
    id TEXT PRIMARY KEY,
    name TEXT,
    virtual_nodes TEXT,    -- JSON array of virtual node objects
    context_nodes TEXT,    -- JSON array of context node objects
    edges TEXT,            -- JSON array of edge objects
    custom_prompt TEXT,    -- User-edited prompt (NULL = use generated)
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

-- Indexes for faster queries
CREATE INDEX IF NOT EXISTS idx_symbols_name ON symbols(name);
CREATE INDEX IF NOT EXISTS idx_symbols_file ON symbols(file_path);
CREATE INDEX IF NOT EXISTS idx_symbols_file_line ON symbols(file_path, line);
CREATE INDEX IF NOT EXISTS idx_method_calls_caller ON method_calls(caller_id);
CREATE INDEX IF NOT EXISTS idx_method_calls_callee ON method_calls(callee_id);
CREATE INDEX IF NOT EXISTS idx_method_calls_file ON method_calls(file_path);
CREATE INDEX IF NOT EXISTS idx_virtual_nodes_class ON virtual_nodes(virtual_class_id);
CREATE INDEX IF NOT EXISTS idx_virtual_nodes_dir ON virtual_nodes(virtual_directory);
CREATE INDEX IF NOT EXISTS idx_virtual_nodes_label ON virtual_nodes(label);
CREATE INDEX IF NOT EXISTS idx_node_annotations_node ON node_annotations(node_id);
CREATE INDEX IF NOT EXISTS idx_edge_annotations_edge ON edge_annotations(edge_id);
CREATE INDEX IF NOT EXISTS idx_edge_annotations_source ON edge_annotations(source_node_id);
CREATE INDEX IF NOT EXISTS idx_edge_annotations_target ON edge_annotations(target_node_id);
CREATE INDEX IF NOT EXISTS idx_mask_sessions_status ON mask_sessions(status);
CREATE INDEX IF NOT EXISTS idx_mask_sessions_created ON mask_sessions(created_at);
CREATE INDEX IF NOT EXISTS idx_virtual_directories_parent ON virtual_directories(parent_path);
CREATE INDEX IF NOT EXISTS idx_snapshots_created ON snapshots(created_at);
`

// InitSchema creates the database schema.
func InitSchema(store *Store) error {
	// Execute main schema
	_, err := store.db.Exec(schema)
	if err != nil {
		return err
	}

	// Ensure symbols table has on-save indexing columns.
	if err := ensureSymbolsIncrementalColumns(store.db); err != nil {
		return err
	}

	// Ensure snapshot custom_prompt column exists for existing databases
	if err := ensureSnapshotCustomPromptColumn(store.db); err != nil {
		return err
	}

	// Ensure virtual_nodes.materialized column exists for existing databases
	if err := ensureVirtualNodesMaterializedColumn(store.db); err != nil {
		return err
	}

	// Ensure virtual_nodes.virtual_class_id column exists for existing databases
	if err := ensureVirtualNodesVirtualClassIDColumn(store.db); err != nil {
		return err
	}

	// Ensure symbols + method_calls have LSP canonical columns.
	if err := ensureSymbolsCanonicalColumns(store.db); err != nil {
		return err
	}
	if err := ensureMethodCallsCanonicalColumns(store.db); err != nil {
		return err
	}

	return nil
}

func ensureVirtualNodesMaterializedColumn(db *sql.DB) error {
	existing := map[string]bool{}
	rows, err := db.Query(`PRAGMA table_info(virtual_nodes)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		existing[name] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if !existing["materialized"] {
		if _, err := db.Exec("ALTER TABLE virtual_nodes ADD COLUMN materialized INTEGER DEFAULT 0"); err != nil {
			return err
		}
	}
	return nil
}

func ensureVirtualNodesVirtualClassIDColumn(db *sql.DB) error {
	existing := map[string]bool{}
	rows, err := db.Query(`PRAGMA table_info(virtual_nodes)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		existing[name] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if !existing["virtual_class_id"] {
		if _, err := db.Exec("ALTER TABLE virtual_nodes ADD COLUMN virtual_class_id TEXT"); err != nil {
			return err
		}
	}
	return nil
}

func ensureSnapshotCustomPromptColumn(db *sql.DB) error {
	// Check if custom_prompt column exists
	existing := map[string]bool{}
	rows, err := db.Query(`PRAGMA table_info(snapshots)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		existing[name] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if !existing["custom_prompt"] {
		if _, err := db.Exec("ALTER TABLE snapshots ADD COLUMN custom_prompt TEXT"); err != nil {
			return err
		}
	}
	return nil
}

func ensureSymbolsIncrementalColumns(db *sql.DB) error {
	existing := map[string]bool{}
	rows, err := db.Query(`PRAGMA table_info(symbols)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		existing[name] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if !existing["end_line"] {
		if _, err := db.Exec("ALTER TABLE symbols ADD COLUMN end_line INTEGER DEFAULT 0"); err != nil {
			return err
		}
	}
	if !existing["body_hash"] {
		if _, err := db.Exec("ALTER TABLE symbols ADD COLUMN body_hash TEXT DEFAULT ''"); err != nil {
			return err
		}
	}
	if _, err := db.Exec("CREATE INDEX IF NOT EXISTS idx_symbols_file_line ON symbols(file_path, line)"); err != nil {
		return err
	}
	return nil
}

func ensureSymbolsCanonicalColumns(db *sql.DB) error {
	existing := map[string]bool{}
	rows, err := db.Query(`PRAGMA table_info(symbols)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		existing[name] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	addCols := []struct {
		name string
		def  string
	}{
		{"canonical_id", "TEXT"},
		{"uri", "TEXT"},
		{"range_start_line", "INTEGER"},
		{"range_start_col", "INTEGER"},
		{"range_end_line", "INTEGER"},
		{"range_end_col", "INTEGER"},
	}
	for _, col := range addCols {
		if !existing[col.name] {
			if _, err := db.Exec("ALTER TABLE symbols ADD COLUMN " + col.name + " " + col.def); err != nil {
				return err
			}
		}
	}
	if _, err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_symbols_canonical ON symbols(canonical_id) WHERE canonical_id IS NOT NULL"); err != nil {
		return err
	}
	return nil
}

func ensureMethodCallsCanonicalColumns(db *sql.DB) error {
	existing := map[string]bool{}
	rows, err := db.Query(`PRAGMA table_info(method_calls)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		existing[name] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	addCols := []struct {
		name string
		def  string
	}{
		{"caller_canonical_id", "TEXT"},
		{"callee_canonical_id", "TEXT"},
		{"callee_uri", "TEXT"},
		{"callee_range_start_line", "INTEGER"},
		{"callee_range_start_col", "INTEGER"},
		{"callee_range_end_line", "INTEGER"},
		{"callee_range_end_col", "INTEGER"},
	}
	for _, col := range addCols {
		if !existing[col.name] {
			if _, err := db.Exec("ALTER TABLE method_calls ADD COLUMN " + col.name + " " + col.def); err != nil {
				return err
			}
		}
	}
	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_method_calls_canonical
		ON method_calls(caller_canonical_id, callee_canonical_id)
		WHERE caller_canonical_id IS NOT NULL AND callee_canonical_id IS NOT NULL`); err != nil {
		return err
	}
	return nil
}
