package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"example.com/fyp/pkg/indexer"
)

func TestEndToEndScanSymbols(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fyp-integration-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	javaFile := filepath.Join(tmpDir, "UserService.java")
	code := `
public class UserService {
    public UserService() {}
    public boolean authenticate(String username, String password) { return true; }
    public void registerUser(String username, String password) {}
}`
	if err := os.WriteFile(javaFile, []byte(code), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	idx, err := indexer.NewIndexer(tmpDir)
	if err != nil {
		t.Fatalf("create indexer: %v", err)
	}
	defer idx.Close()

	symbols, err := idx.ScanSymbols(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("scan symbols: %v", err)
	}
	if len(symbols) == 0 {
		t.Fatalf("expected non-empty scan result")
	}

	dbPath := filepath.Join(tmpDir, ".fyp", "index.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Errorf("database file not created at %s", dbPath)
	}
}
