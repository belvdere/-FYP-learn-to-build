package db

import (
	"database/sql"
	"fmt"
	"time"
)

// Session status constants
const (
	StatusPending   = "pending"   // Initial state after masking
	StatusFilled    = "filled"    // User filled code but validation failed
	StatusValidated = "validated" // User filled code and validation passed
)

// MaskSession represents a masking session stored in the database
type MaskSession struct {
	ID           string
	FilePath     string
	OriginalCode string
	MaskedCode   string
	FilledCode   string
	Language     string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Status       string // 'pending', 'filled', 'validated'
}

// CreateMaskSession creates a new mask session in the database
func (s *Store) CreateMaskSession(session *MaskSession) error {
	_, err := s.db.Exec(`
		INSERT INTO mask_sessions (
			id, file_path, original_code, masked_code, filled_code,
			language, created_at, updated_at, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session.ID,
		session.FilePath,
		session.OriginalCode,
		session.MaskedCode,
		session.FilledCode,
		session.Language,
		session.CreatedAt.Unix(),
		session.UpdatedAt.Unix(),
		session.Status,
	)
	return err
}

// GetMaskSession retrieves a mask session by ID
func (s *Store) GetMaskSession(id string) (*MaskSession, error) {
	var session MaskSession
	var createdAt, updatedAt int64

	err := s.db.QueryRow(`
		SELECT id, file_path, original_code, masked_code, filled_code,
		       language, created_at, updated_at, status
		FROM mask_sessions
		WHERE id = ?`,
		id,
	).Scan(
		&session.ID,
		&session.FilePath,
		&session.OriginalCode,
		&session.MaskedCode,
		&session.FilledCode,
		&session.Language,
		&createdAt,
		&updatedAt,
		&session.Status,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("mask session not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get mask session: %w", err)
	}

	session.CreatedAt = time.Unix(createdAt, 0)
	session.UpdatedAt = time.Unix(updatedAt, 0)

	return &session, nil
}

// UpdateFilledCode updates the filled code for a mask session and sets status to 'filled'
func (s *Store) UpdateFilledCode(id string, filledCode string) error {
	_, err := s.db.Exec(`
		UPDATE mask_sessions
		SET filled_code = ?, status = ?, updated_at = ?
		WHERE id = ?`,
		filledCode,
		StatusFilled,
		time.Now().Unix(),
		id,
	)
	return err
}

// UpdateStatus updates the status of a mask session
func (s *Store) UpdateStatus(id string, status string) error {
	_, err := s.db.Exec(`
		UPDATE mask_sessions
		SET status = ?, updated_at = ?
		WHERE id = ?`,
		status,
		time.Now().Unix(),
		id,
	)
	return err
}

// UpdateFilledCodeAndStatus atomically updates filled code and status
func (s *Store) UpdateFilledCodeAndStatus(id string, filledCode string, status string) error {
	_, err := s.db.Exec(`
		UPDATE mask_sessions
		SET filled_code = ?, status = ?, updated_at = ?
		WHERE id = ?`,
		filledCode,
		status,
		time.Now().Unix(),
		id,
	)
	return err
}

// SymbolExists checks if a symbol exists in the workspace
func (s *Store) SymbolExists(symbolName string) (bool, error) {
	var count int
	err := s.db.QueryRow(
		"SELECT COUNT(*) FROM symbols WHERE name = ?",
		symbolName,
	).Scan(&count)

	if err != nil {
		return false, fmt.Errorf("failed to check symbol existence: %w", err)
	}

	return count > 0, nil
}
