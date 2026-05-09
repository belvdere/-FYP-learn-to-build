package db

import (
	"database/sql"
	"fmt"
	"time"

	"example.com/fyp/pkg/types"
)

// InsertVirtualDirectory inserts a new virtual directory into the database.
func (s *Store) InsertVirtualDirectory(dir types.VirtualDirectory) error {
	now := time.Now().Unix()

	query := `
		INSERT INTO virtual_directories
		(id, path, name, parent_path, created_at)
		VALUES (?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(query,
		dir.ID,
		dir.Path,
		dir.Name,
		dir.ParentPath,
		now,
	)

	return err
}

// GetVirtualDirectory retrieves a virtual directory by ID.
func (s *Store) GetVirtualDirectory(id string) (*types.VirtualDirectory, error) {
	query := `
		SELECT id, path, name, parent_path, created_at
		FROM virtual_directories
		WHERE id = ?
	`

	var dir types.VirtualDirectory
	var parentPath sql.NullString

	err := s.db.QueryRow(query, id).Scan(
		&dir.ID,
		&dir.Path,
		&dir.Name,
		&parentPath,
		&dir.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	if parentPath.Valid {
		dir.ParentPath = parentPath.String
	}

	return &dir, nil
}

// GetVirtualDirectoryByPath retrieves a virtual directory by path.
func (s *Store) GetVirtualDirectoryByPath(path string) (*types.VirtualDirectory, error) {
	query := `
		SELECT id, path, name, parent_path, created_at
		FROM virtual_directories
		WHERE path = ?
	`

	var dir types.VirtualDirectory
	var parentPath sql.NullString

	err := s.db.QueryRow(query, path).Scan(
		&dir.ID,
		&dir.Path,
		&dir.Name,
		&parentPath,
		&dir.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	if parentPath.Valid {
		dir.ParentPath = parentPath.String
	}

	return &dir, nil
}

// GetVirtualDirectoriesByParent retrieves all virtual directories with a given parent.
func (s *Store) GetVirtualDirectoriesByParent(parentPath string) ([]types.VirtualDirectory, error) {
	query := `
		SELECT id, path, name, parent_path, created_at
		FROM virtual_directories
		WHERE parent_path = ?
		ORDER BY name
	`

	rows, err := s.db.Query(query, parentPath)
	if err != nil {
		return nil, fmt.Errorf("query virtual directories: %w", err)
	}
	defer rows.Close()

	var directories []types.VirtualDirectory
	for rows.Next() {
		var dir types.VirtualDirectory
		var parentPath sql.NullString

		err := rows.Scan(
			&dir.ID,
			&dir.Path,
			&dir.Name,
			&parentPath,
			&dir.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan virtual directory: %w", err)
		}

		if parentPath.Valid {
			dir.ParentPath = parentPath.String
		}

		directories = append(directories, dir)
	}

	return directories, rows.Err()
}

// GetAllVirtualDirectories retrieves all virtual directories.
func (s *Store) GetAllVirtualDirectories() ([]types.VirtualDirectory, error) {
	query := `
		SELECT id, path, name, parent_path, created_at
		FROM virtual_directories
		ORDER BY path
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query virtual directories: %w", err)
	}
	defer rows.Close()

	var directories []types.VirtualDirectory
	for rows.Next() {
		var dir types.VirtualDirectory
		var parentPath sql.NullString

		err := rows.Scan(
			&dir.ID,
			&dir.Path,
			&dir.Name,
			&parentPath,
			&dir.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan virtual directory: %w", err)
		}

		if parentPath.Valid {
			dir.ParentPath = parentPath.String
		}

		directories = append(directories, dir)
	}

	return directories, rows.Err()
}

// DeleteVirtualDirectory deletes a virtual directory by ID.
// This operation is idempotent - it succeeds even if the directory doesn't exist.
func (s *Store) DeleteVirtualDirectory(id string) error {
	query := `DELETE FROM virtual_directories WHERE id = ?`

	_, err := s.db.Exec(query, id)
	return err
}
