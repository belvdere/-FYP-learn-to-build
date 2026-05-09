package db

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestComputeFileHash(t *testing.T) {
	tests := []struct {
		name     string
		content1 string
		content2 string
		sameHash bool
	}{
		{
			name:     "Same content",
			content1: "public class Test {}",
			content2: "public class Test {}",
			sameHash: true,
		},
		{
			name:     "Different content",
			content1: "public class Test1 {}",
			content2: "public class Test2 {}",
			sameHash: false,
		},
		{
			name:     "Whitespace matters",
			content1: "public class Test{}",
			content2: "public class Test {}",
			sameHash: false,
		},
		{
			name:     "Empty content",
			content1: "",
			content2: "",
			sameHash: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1 := ComputeFileHash([]byte(tt.content1))
			hash2 := ComputeFileHash([]byte(tt.content2))

			if tt.sameHash && hash1 != hash2 {
				t.Errorf("Expected same hash, got %s and %s", hash1, hash2)
			}
			if !tt.sameHash && hash1 == hash2 {
				t.Errorf("Expected different hashes, got same: %s", hash1)
			}

			// Hash should be 64 characters (SHA256 hex)
			if len(hash1) != 64 {
				t.Errorf("Expected hash length 64, got %d", len(hash1))
			}
		})
	}
}

func TestCacheOperations(t *testing.T) {
	// Create temporary database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Test data
	filePath := "/test/Example.java"
	fileHash := "abc123"

	// Initially, cache should be invalid
	valid, err := store.IsCacheValid(filePath, fileHash)
	if err != nil {
		t.Fatalf("Failed to check cache: %v", err)
	}
	if valid {
		t.Error("Cache should be invalid initially")
	}

	// Upsert cache entry
	entry := &CacheEntry{
		FilePath:    filePath,
		FileHash:    fileHash,
		LastIndexed: time.Now(),
	}
	err = store.UpsertCacheEntry(entry)
	if err != nil {
		t.Fatalf("Failed to upsert cache entry: %v", err)
	}

	// Now cache should be valid
	valid, err = store.IsCacheValid(filePath, fileHash)
	if err != nil {
		t.Fatalf("Failed to check cache: %v", err)
	}
	if !valid {
		t.Error("Cache should be valid after upsert")
	}

	// Different hash should be invalid
	valid, err = store.IsCacheValid(filePath, "different_hash")
	if err != nil {
		t.Fatalf("Failed to check cache: %v", err)
	}
	if valid {
		t.Error("Cache should be invalid with different hash")
	}

	// Update with new hash
	newHash := "def456"
	entry.FileHash = newHash
	entry.LastIndexed = time.Now()
	err = store.UpsertCacheEntry(entry)
	if err != nil {
		t.Fatalf("Failed to update cache entry: %v", err)
	}

	// Old hash should be invalid
	valid, err = store.IsCacheValid(filePath, fileHash)
	if err != nil {
		t.Fatalf("Failed to check cache: %v", err)
	}
	if valid {
		t.Error("Cache should be invalid after update with new hash")
	}

	// New hash should be valid
	valid, err = store.IsCacheValid(filePath, newHash)
	if err != nil {
		t.Fatalf("Failed to check cache: %v", err)
	}
	if !valid {
		t.Error("Cache should be valid with new hash")
	}
}

func TestCacheClear(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Add multiple cache entries
	files := []string{"/test/File1.java", "/test/File2.java", "/test/File3.java"}
	for _, file := range files {
		entry := &CacheEntry{
			FilePath:    file,
			FileHash:    "hash",
			LastIndexed: time.Now(),
		}
		err := store.UpsertCacheEntry(entry)
		if err != nil {
			t.Fatalf("Failed to upsert cache entry: %v", err)
		}
	}

	// Verify all are cached
	for _, file := range files {
		valid, err := store.IsCacheValid(file, "hash")
		if err != nil {
			t.Fatalf("Failed to check cache: %v", err)
		}
		if !valid {
			t.Errorf("File %s should be cached", file)
		}
	}

	// Clear cache
	err = store.ClearCache()
	if err != nil {
		t.Fatalf("Failed to clear cache: %v", err)
	}

	// Verify all are cleared
	for _, file := range files {
		valid, err := store.IsCacheValid(file, "hash")
		if err != nil {
			t.Fatalf("Failed to check cache: %v", err)
		}
		if valid {
			t.Errorf("File %s should not be cached after clear", file)
		}
	}
}

func TestCacheWithRealFile(t *testing.T) {
	// Create temporary directory and file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "Test.java")
	content := []byte("public class Test {}")

	err := os.WriteFile(testFile, content, 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create database
	dbPath := filepath.Join(tempDir, "test.db")
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Compute hash
	hash1 := ComputeFileHash(content)

	// Cache the file
	entry := &CacheEntry{
		FilePath:    testFile,
		FileHash:    hash1,
		LastIndexed: time.Now(),
	}
	err = store.UpsertCacheEntry(entry)
	if err != nil {
		t.Fatalf("Failed to cache file: %v", err)
	}

	// Should be valid
	valid, err := store.IsCacheValid(testFile, hash1)
	if err != nil {
		t.Fatalf("Failed to check cache: %v", err)
	}
	if !valid {
		t.Error("Cache should be valid")
	}

	// Modify file
	modifiedContent := []byte("public class Test { /* modified */ }")
	err = os.WriteFile(testFile, modifiedContent, 0644)
	if err != nil {
		t.Fatalf("Failed to modify file: %v", err)
	}

	// Compute new hash from modified content
	hash2 := ComputeFileHash(modifiedContent)

	// The new hash should be different from old hash
	if hash2 == hash1 {
		t.Fatal("Hash should change after file modification")
	}

	// Cache should be INVALID for the NEW hash (file changed, cache has old hash)
	valid, err = store.IsCacheValid(testFile, hash2)
	if err != nil {
		t.Fatalf("Failed to check cache: %v", err)
	}
	if valid {
		t.Error("Cache should be invalid when file hash doesn't match cached hash")
	}

	// Cache should still be VALID for the OLD hash (this is what's in the cache)
	// This demonstrates that the cache hasn't been updated yet
	valid, err = store.IsCacheValid(testFile, hash1)
	if err != nil {
		t.Fatalf("Failed to check cache: %v", err)
	}
	if !valid {
		t.Error("Cache should still be valid for old hash until updated")
	}

	// Update cache with new hash
	entry.FileHash = hash2
	entry.LastIndexed = time.Now()
	err = store.UpsertCacheEntry(entry)
	if err != nil {
		t.Fatalf("Failed to update cache: %v", err)
	}

	// New hash should be valid
	valid, err = store.IsCacheValid(testFile, hash2)
	if err != nil {
		t.Fatalf("Failed to check cache: %v", err)
	}
	if !valid {
		t.Error("Cache should be valid with new hash")
	}
}

func TestCacheTimestamp(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	filePath := "/test/Example.java"
	fileHash := "abc123"

	// Record time before caching
	before := time.Now().Unix()

	// Cache file
	entry := &CacheEntry{
		FilePath:    filePath,
		FileHash:    fileHash,
		LastIndexed: time.Now(),
	}
	err = store.UpsertCacheEntry(entry)
	if err != nil {
		t.Fatalf("Failed to cache file: %v", err)
	}

	// Record time after caching
	time.Sleep(10 * time.Millisecond) // Ensure time difference
	after := time.Now().Unix()

	// Query cache entry directly to check timestamp
	var lastIndexed int64
	query := `SELECT last_indexed FROM file_cache WHERE file_path = ?`
	err = store.DB().QueryRow(query, filePath).Scan(&lastIndexed)
	if err != nil {
		t.Fatalf("Failed to query cache entry: %v", err)
	}

	// Timestamp should be between before and after
	if lastIndexed < before || lastIndexed > after {
		t.Errorf("Timestamp %d should be between %d and %d", lastIndexed, before, after)
	}
}

func TestCacheStats(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Initially should be empty
	fileCount, err := store.GetCacheStats()
	if err != nil {
		t.Fatalf("Failed to get cache stats: %v", err)
	}
	if fileCount != 0 {
		t.Errorf("Expected 0 files, got %d", fileCount)
	}

	// Add some cache entries
	paths := []string{"/test/File1.java", "/test/File2.java", "/test/File3.java"}

	for _, p := range paths {
		entry := &CacheEntry{
			FilePath:    p,
			FileHash:    "hash",
			LastIndexed: time.Now(),
		}
		err := store.UpsertCacheEntry(entry)
		if err != nil {
			t.Fatalf("Failed to upsert cache entry: %v", err)
		}
	}

	// Check stats
	fileCount, err = store.GetCacheStats()
	if err != nil {
		t.Fatalf("Failed to get cache stats: %v", err)
	}
	if fileCount != 3 {
		t.Errorf("Expected 3 files, got %d", fileCount)
	}
}

func TestDeleteCacheEntry(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	filePath := "/test/Example.java"

	// Add cache entry
	entry := &CacheEntry{
		FilePath:    filePath,
		FileHash:    "hash",
		LastIndexed: time.Now(),
	}
	err = store.UpsertCacheEntry(entry)
	if err != nil {
		t.Fatalf("Failed to upsert cache entry: %v", err)
	}

	// Verify it's cached
	valid, err := store.IsCacheValid(filePath, "hash")
	if err != nil {
		t.Fatalf("Failed to check cache: %v", err)
	}
	if !valid {
		t.Error("Cache should be valid")
	}

	// Delete the entry
	err = store.DeleteCacheEntry(filePath)
	if err != nil {
		t.Fatalf("Failed to delete cache entry: %v", err)
	}

	// Verify it's no longer cached
	valid, err = store.IsCacheValid(filePath, "hash")
	if err != nil {
		t.Fatalf("Failed to check cache: %v", err)
	}
	if valid {
		t.Error("Cache should be invalid after deletion")
	}
}

// BenchmarkCacheCheck benchmarks cache validation
func BenchmarkCacheCheck(b *testing.B) {
	tempDir := b.TempDir()
	dbPath := filepath.Join(tempDir, "bench.db")

	store, err := NewStore(dbPath)
	if err != nil {
		b.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Populate cache with some entries
	for i := 0; i < 100; i++ {
		filePath := filepath.Join("/test", string(rune(i))+".java")
		entry := &CacheEntry{
			FilePath:    filePath,
			FileHash:    "hash",
			LastIndexed: time.Now(),
		}
		err := store.UpsertCacheEntry(entry)
		if err != nil {
			b.Fatalf("Failed to populate cache: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.IsCacheValid("/test/50.java", "hash")
	}
}
