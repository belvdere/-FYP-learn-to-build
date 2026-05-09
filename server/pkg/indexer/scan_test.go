package indexer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestScanSymbols(t *testing.T) {
	tmp := t.TempDir()
	code := `
public class UserService {
    public UserService() {}
    public void save(User u) {}
    public User findById(String id) { return null; }
}`
	f := filepath.Join(tmp, "UserService.java")
	if err := os.WriteFile(f, []byte(code), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	idx, err := NewIndexer(tmp)
	if err != nil {
		t.Fatalf("new indexer: %v", err)
	}
	defer idx.Close()

	symbols, err := idx.ScanSymbols(context.Background(), tmp)
	if err != nil {
		t.Fatalf("scan symbols: %v", err)
	}
	if len(symbols) == 0 {
		t.Fatalf("expected scanned symbols, got none")
	}

	seen := map[string]bool{}
	for _, s := range symbols {
		seen[s.Name] = true
		if s.ID == "" || s.URI == "" || s.Line <= 0 {
			t.Fatalf("invalid scanned symbol: %+v", s)
		}
		if s.Kind == "method" || s.Kind == "constructor" {
			if s.BodyHash == "" {
				t.Fatalf("expected bodyHash for callable symbol: %+v", s)
			}
		}
	}

	if !seen["UserService.save(User)"] {
		t.Fatalf("expected UserService.save in scanned symbols")
	}
	if !seen["UserService.findById(String)"] {
		t.Fatalf("expected UserService.findById in scanned symbols")
	}
}

func TestScanSymbols_GoFunctionIncluded(t *testing.T) {
	tmp := t.TempDir()
	code := `package demo

func SaveUser(id string) error {
	return nil
}
`
	f := filepath.Join(tmp, "service.go")
	if err := os.WriteFile(f, []byte(code), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	idx, err := NewIndexer(tmp)
	if err != nil {
		t.Fatalf("new indexer: %v", err)
	}
	defer idx.Close()

	symbols, err := idx.ScanSymbols(context.Background(), tmp)
	if err != nil {
		t.Fatalf("scan symbols: %v", err)
	}
	if len(symbols) == 0 {
		t.Fatalf("expected scanned symbols, got none")
	}

	found := false
	for _, s := range symbols {
		if s.Name == "SaveUser(string)" {
			found = true
			if s.Kind != "function" {
				t.Fatalf("expected kind=function, got %s", s.Kind)
			}
			if s.BodyHash == "" {
				t.Fatalf("expected bodyHash for go function symbol: %+v", s)
			}
		}
	}

	if !found {
		t.Fatalf("expected SaveUser(string) in scanned symbols")
	}
}
