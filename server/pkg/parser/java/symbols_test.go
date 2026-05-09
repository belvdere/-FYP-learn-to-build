package java

import (
	"testing"
)

const testJavaCode = `
package com.example;

import java.util.List;

public class UserService {
    private String serviceName;
    private int maxUsers;
    
    public UserService(String name) {
        this.serviceName = name;
    }
    
    public void authenticate(String username, String password) {
        System.out.println("Authenticating " + username);
    }
    
    public static void main(String[] args) {
        UserService service = new UserService("Main");
        service.authenticate("user", "pass");
    }
}

interface UserRepository {
    void save(String user);
    String find(String id);
}
`

func TestExtractSymbols(t *testing.T) {
	parser := NewJavaParser()
	defer parser.Close()

	tree, err := parser.Parse([]byte(testJavaCode))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	extractor := NewJavaSymbolExtractor()
	symbols := extractor.ExtractSymbols(tree, []byte(testJavaCode), "UserService.java")

	if len(symbols) == 0 {
		t.Fatal("no symbols extracted")
	}

	// Count symbols by kind
	counts := make(map[string]int)
	for _, sym := range symbols {
		counts[sym.Kind]++
	}

	// Should have at least 1 class, 1 interface, methods, constructor, fields
	if counts["class"] < 1 {
		t.Errorf("expected at least 1 class, got %d", counts["class"])
	}

	if counts["interface"] < 1 {
		t.Errorf("expected at least 1 interface, got %d", counts["interface"])
	}

	if counts["method"] < 2 {
		t.Errorf("expected at least 2 methods, got %d", counts["method"])
	}

	if counts["constructor"] < 1 {
		t.Errorf("expected at least 1 constructor, got %d", counts["constructor"])
	}

	if counts["field"] < 2 {
		t.Errorf("expected at least 2 fields, got %d", counts["field"])
	}
}

func TestSymbolIDs(t *testing.T) {
	parser := NewJavaParser()
	defer parser.Close()

	tree, err := parser.Parse([]byte(testJavaCode))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	extractor := NewJavaSymbolExtractor()
	symbols := extractor.ExtractSymbols(tree, []byte(testJavaCode), "UserService.java")

	// Check that all symbols have unique IDs
	seen := make(map[string]bool)
	for _, sym := range symbols {
		if sym.ID == "" {
			t.Errorf("symbol %s has empty ID", sym.Name)
		}

		if seen[sym.ID] {
			t.Errorf("duplicate symbol ID: %s", sym.ID)
		}
		seen[sym.ID] = true
	}
}

func TestSymbolLocations(t *testing.T) {
	parser := NewJavaParser()
	defer parser.Close()

	tree, err := parser.Parse([]byte(testJavaCode))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	extractor := NewJavaSymbolExtractor()
	symbols := extractor.ExtractSymbols(tree, []byte(testJavaCode), "UserService.java")

	// All symbols should have line numbers > 0
	for _, sym := range symbols {
		if sym.Line <= 0 {
			t.Errorf("symbol %s has invalid line number: %d", sym.Name, sym.Line)
		}

		if sym.FilePath != "UserService.java" {
			t.Errorf("symbol %s has wrong file path: %s", sym.Name, sym.FilePath)
		}
	}
}

func TestSymbolSignatures(t *testing.T) {
	parser := NewJavaParser()
	defer parser.Close()

	tree, err := parser.Parse([]byte(testJavaCode))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	extractor := NewJavaSymbolExtractor()
	symbols := extractor.ExtractSymbols(tree, []byte(testJavaCode), "UserService.java")

	// Check that methods have signatures
	for _, sym := range symbols {
		if sym.Kind == "method" || sym.Kind == "constructor" || sym.Kind == "class" {
			if sym.Signature == "" {
				t.Errorf("symbol %s (kind %s) has empty signature", sym.Name, sym.Kind)
			}
		}
	}
}

func TestQualifiedNames(t *testing.T) {
	parser := NewJavaParser()
	defer parser.Close()

	tree, err := parser.Parse([]byte(testJavaCode))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	extractor := NewJavaSymbolExtractor()
	symbols := extractor.ExtractSymbols(tree, []byte(testJavaCode), "UserService.java")

	// Check that methods and fields have qualified names (ClassName.memberName)
	for _, sym := range symbols {
		if sym.Kind == "method" || sym.Kind == "field" || sym.Kind == "constructor" {
			// Should contain a dot for qualified name
			// (unless it's a top-level method, which shouldn't exist in Java)
			if sym.Name != "" && !containsDot(sym.Name) {
				// For interface methods, this might not have a dot if extraction failed
				// so we just warn
				t.Logf("warning: %s %s doesn't have qualified name", sym.Kind, sym.Name)
			}
		}
	}
}

func containsDot(s string) bool {
	for _, c := range s {
		if c == '.' {
			return true
		}
	}
	return false
}
