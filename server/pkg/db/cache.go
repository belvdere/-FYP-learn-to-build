package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

// CacheEntry represents a cached file's metadata.
type CacheEntry struct {
	FilePath    string
	FileHash    string
	LastIndexed time.Time
}

// ComputeFileHash computes SHA256 hash of file content.
func ComputeFileHash(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

// GetCacheEntry retrieves the cache entry for a file.
func (s *Store) GetCacheEntry(filePath string) (*CacheEntry, error) {
	query := `
		SELECT file_path, file_hash, last_indexed
		FROM file_cache
		WHERE file_path = ?
	`

	var entry CacheEntry
	var lastIndexedUnix int64

	err := s.db.QueryRow(query, filePath).Scan(
		&entry.FilePath,
		&entry.FileHash,
		&lastIndexedUnix,
	)

	if err == sql.ErrNoRows {
		return nil, nil // No cache entry found
	}
	if err != nil {
		return nil, fmt.Errorf("query cache entry: %w", err)
	}

	entry.LastIndexed = time.Unix(lastIndexedUnix, 0)
	return &entry, nil
}

// UpsertCacheEntry inserts or updates a cache entry.
func (s *Store) UpsertCacheEntry(entry *CacheEntry) error {
	query := `
		INSERT OR REPLACE INTO file_cache
		(file_path, file_hash, last_indexed)
		VALUES (?, ?, ?)
	`

	_, err := s.db.Exec(
		query,
		entry.FilePath,
		entry.FileHash,
		entry.LastIndexed.Unix(),
	)

	if err != nil {
		return fmt.Errorf("upsert cache entry: %w", err)
	}

	return nil
}

// DeleteCacheEntry removes a cache entry for a file.
func (s *Store) DeleteCacheEntry(filePath string) error {
	query := `DELETE FROM file_cache WHERE file_path = ?`
	_, err := s.db.Exec(query, filePath)
	if err != nil {
		return fmt.Errorf("delete cache entry: %w", err)
	}
	return nil
}

// IsCacheValid checks if the cache is valid for a file.
// It returns true if:
// 1. Cache entry exists
// 2. File hash matches
func (s *Store) IsCacheValid(filePath string, fileHash string) (bool, error) {
	entry, err := s.GetCacheEntry(filePath)
	if err != nil {
		return false, fmt.Errorf("get cache entry: %w", err)
	}

	if entry == nil {
		return false, nil // No cache entry
	}

	// Check if file hash matches
	if entry.FileHash != fileHash {
		return false, nil // File changed
	}

	return true, nil
}

// ClearCache removes all cache entries.
func (s *Store) ClearCache() error {
	query := `DELETE FROM file_cache`
	_, err := s.db.Exec(query)
	if err != nil {
		return fmt.Errorf("clear cache: %w", err)
	}
	return nil
}

// GetCacheStats returns the number of cached files.
func (s *Store) GetCacheStats() (int, error) {
	var totalFiles int

	query := `SELECT COUNT(*) as file_count FROM file_cache`
	err := s.db.QueryRow(query).Scan(&totalFiles)
	if err != nil {
		return 0, fmt.Errorf("get cache stats: %w", err)
	}

	return totalFiles, nil
}
