package db

import (
	"testing"

	"example.com/fyp/pkg/types"
)

func TestResolveSymbolIDsByLocations(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	if err := store.InsertSymbol(types.Symbol{
		ID:       "symA",
		Name:     "A.m()",
		Kind:     "method",
		FilePath: "/tmp/A.java",
		Line:     10,
		EndLine:  20,
	}); err != nil {
		t.Fatalf("InsertSymbol A failed: %v", err)
	}
	if err := store.InsertSymbol(types.Symbol{
		ID:       "symB",
		Name:     "B.n()",
		Kind:     "method",
		FilePath: "/tmp/B.java",
		Line:     5,
		EndLine:  8,
	}); err != nil {
		t.Fatalf("InsertSymbol B failed: %v", err)
	}

	resolved, err := store.ResolveSymbolIDsByLocations([]types.SymbolLocation{
		{FilePath: "/tmp/A.java", Line: 10},
		{FilePath: "/tmp/A.java", Line: 15},
		{FilePath: "/tmp/B.java", Line: 7},
		{FilePath: "/tmp/Unknown.java", Line: 1},
	})
	if err != nil {
		t.Fatalf("ResolveSymbolIDsByLocations failed: %v", err)
	}
	if len(resolved) != 4 {
		t.Fatalf("expected 4 results, got %d", len(resolved))
	}
	if resolved[0].SymbolID != "symA" || resolved[1].SymbolID != "symA" || resolved[2].SymbolID != "symB" {
		t.Fatalf("unexpected resolved IDs: %+v", resolved)
	}
	if resolved[3].SymbolID != "" {
		t.Fatalf("expected unresolved symbol for unknown file, got %+v", resolved[3])
	}
}

func TestApplyFileDelta_PreservesManualAndReplacesChanged(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	filePath := "/tmp/A.java"

	if err := store.InsertSymbol(types.Symbol{ID: "callerOld", Name: "A.old()", Kind: "method", FilePath: filePath, Line: 10}); err != nil {
		t.Fatalf("InsertSymbol callerOld failed: %v", err)
	}
	if err := store.InsertSymbol(types.Symbol{ID: "removedSym", Name: "A.removed()", Kind: "method", FilePath: filePath, Line: 20}); err != nil {
		t.Fatalf("InsertSymbol removedSym failed: %v", err)
	}
	if err := store.InsertMethodCall(types.MethodCall{CallerID: "callerOld", CalleeID: "target1", CallType: "direct", FilePath: filePath, Line: 11}); err != nil {
		t.Fatalf("InsertMethodCall callerOld->target1 failed: %v", err)
	}
	if err := store.InsertMethodCall(types.MethodCall{CallerID: "external", CalleeID: "removedSym", CallType: "direct", FilePath: "/tmp/ext.java", Line: 1}); err != nil {
		t.Fatalf("InsertMethodCall external->removedSym failed: %v", err)
	}
	if err := store.InsertMethodCall(types.MethodCall{CallerID: "callerOld", CalleeID: "manualTarget", CallType: "manual", FilePath: "", Line: 0}); err != nil {
		t.Fatalf("InsertMethodCall manual failed: %v", err)
	}

	newSymbols := []types.Symbol{
		{ID: "callerOld", Name: "A.old()", Kind: "method", FilePath: filePath, Line: 10, BodyHash: "newhash"},
		{ID: "newSym", Name: "A.new()", Kind: "method", FilePath: filePath, Line: 30},
	}
	newCalls := []types.MethodCall{
		{CallerID: "callerOld", CalleeID: "target2", CallType: "direct", FilePath: filePath, Line: 12},
	}

	if err := store.ApplyFileDelta(
		filePath,
		"filehash",
		newSymbols,
		[]string{"callerOld"},
		[]string{"removedSym"},
		newCalls,
	); err != nil {
		t.Fatalf("ApplyFileDelta failed: %v", err)
	}

	symbols, err := store.GetSymbolsByFile(filePath)
	if err != nil {
		t.Fatalf("GetSymbolsByFile failed: %v", err)
	}
	if len(symbols) != 2 {
		t.Fatalf("expected 2 symbols in file after delta, got %d", len(symbols))
	}

	calls, err := store.GetAllMethodCalls()
	if err != nil {
		t.Fatalf("GetAllMethodCalls failed: %v", err)
	}

	hasManual := false
	hasNewDirect := false
	for _, c := range calls {
		if c.CallType == "manual" && c.CallerID == "callerOld" && c.CalleeID == "manualTarget" {
			hasManual = true
		}
		if c.CallType == "direct" && c.CallerID == "callerOld" && c.CalleeID == "target2" {
			hasNewDirect = true
		}
		if c.CallerID == "callerOld" && c.CalleeID == "target1" {
			t.Fatalf("stale changed-caller edge still exists: %+v", c)
		}
		if c.CalleeID == "removedSym" {
			t.Fatalf("edge to removed callee still exists: %+v", c)
		}
	}
	if !hasManual {
		t.Fatalf("expected manual edge to be preserved")
	}
	if !hasNewDirect {
		t.Fatalf("expected new direct edge to be inserted")
	}

	cache, err := store.GetCacheEntry(filePath)
	if err != nil {
		t.Fatalf("GetCacheEntry failed: %v", err)
	}
	if cache == nil || cache.FileHash != "filehash" {
		t.Fatalf("expected file cache to be updated, got %+v", cache)
	}
}
