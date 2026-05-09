package masking

import "example.com/fyp/pkg/db"

// ============================================================================
// Masker Interface
// ============================================================================

// Masker analyzes code and inserts mask markers at decision points.
// Each language implements its own masking strategy based on language-specific AST patterns.
type Masker interface {
	// MaskCode analyzes code and inserts mask markers at decision points.
	// Returns the masked code and a list of masks inserted.
	MaskCode(code string) (*MaskResult, error)

	// MaskCodeWithStore masks code and stores the session in the database.
	// Returns: masked code, session ID, error
	MaskCodeWithStore(code string, filePath string, store *db.Store) (string, string, error)
}

