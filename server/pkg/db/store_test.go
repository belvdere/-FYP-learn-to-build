package db

import (
	"os"
	"path/filepath"
	"testing"

	"example.com/fyp/pkg/types"
)

func TestStoreCreation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fyp-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	// Check that database file was created
	dbPath := filepath.Join(tmpDir, ".fyp", "index.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Errorf("database file not created at %s", dbPath)
	}
}

func TestSymbolOperations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fyp-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	// Test inserting a symbol
	symbol := types.Symbol{
		ID:        "sym1",
		Name:      "TestClass",
		Kind:      "class",
		FilePath:  "Test.java",
		Line:      1,
		Signature: "public class TestClass",
	}

	err = store.InsertSymbol(symbol)
	if err != nil {
		t.Fatalf("InsertSymbol failed: %v", err)
	}

	// Verify symbol is stored
	symbols, err := store.GetAllSymbols()
	if err != nil {
		t.Fatalf("GetAllSymbols failed: %v", err)
	}

	if len(symbols) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(symbols))
	}

	if symbols[0].Name != "TestClass" {
		t.Errorf("expected name TestClass, got %s", symbols[0].Name)
	}
}

func TestMethodCallOperations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fyp-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	// Insert a method call
	call := types.MethodCall{
		CallerID: "UserService.createUser",
		CalleeID: "UserRepository.save",
		CallType:     "direct",
		FilePath:     "UserService.java",
		Line:         25,
	}
	if err := store.InsertMethodCall(call); err != nil {
		t.Fatalf("InsertMethodCall failed: %v", err)
	}

	// Verify it was stored
	calls, err := store.GetAllMethodCalls()
	if err != nil {
		t.Fatalf("GetAllMethodCalls failed: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 method call, got %d", len(calls))
	}
	if calls[0].CallerID != "UserService.createUser" {
		t.Errorf("expected caller UserService.createUser, got %s", calls[0].CallerID)
	}
	if calls[0].CalleeID != "UserRepository.save" {
		t.Errorf("expected callee UserRepository.save, got %s", calls[0].CalleeID)
	}
}

func TestDeleteAllDataByFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fyp-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	filePath := "Test.java"

	// Insert symbol
	symbol := types.Symbol{
		ID:       "s1",
		Name:     "TestMethod",
		Kind:     "method",
		FilePath: filePath,
		Line:     1,
	}
	if err := store.InsertSymbol(symbol); err != nil {
		t.Fatalf("InsertSymbol failed: %v", err)
	}

	// Insert method call
	call := types.MethodCall{
		CallerID: "TestMethod",
		CalleeID: "Other.method",
		CallType:     "direct",
		FilePath:     filePath,
		Line:         5,
	}
	if err := store.InsertMethodCall(call); err != nil {
		t.Fatalf("InsertMethodCall failed: %v", err)
	}

	// Verify data exists
	symbols, err := store.GetAllSymbols()
	if err != nil {
		t.Fatalf("GetAllSymbols failed: %v", err)
	}
	if len(symbols) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(symbols))
	}

	// Delete all data for file
	if err := store.DeleteAllDataByFile(filePath); err != nil {
		t.Fatalf("DeleteAllDataByFile failed: %v", err)
	}

	// Verify symbols are gone
	symbols, err = store.GetAllSymbols()
	if err != nil {
		t.Fatalf("GetAllSymbols failed: %v", err)
	}
	if len(symbols) != 0 {
		t.Errorf("expected 0 symbols after delete, got %d", len(symbols))
	}

	// Verify method calls are gone
	calls, err := store.GetAllMethodCalls()
	if err != nil {
		t.Fatalf("GetAllMethodCalls failed: %v", err)
	}
	if len(calls) != 0 {
		t.Errorf("expected 0 method calls after delete, got %d", len(calls))
	}
}

func TestClearAllData(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fyp-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	// Insert some data
	symbol := types.Symbol{ID: "s1", Name: "Test", Kind: "class", FilePath: "Test.java", Line: 1}
	if err := store.InsertSymbol(symbol); err != nil {
		t.Fatalf("InsertSymbol failed: %v", err)
	}

	call := types.MethodCall{CallerID: "A.a", CalleeID: "B.b", CallType: "direct", FilePath: "A.java", Line: 1}
	if err := store.InsertMethodCall(call); err != nil {
		t.Fatalf("InsertMethodCall failed: %v", err)
	}

	// Clear all data
	if err := store.ClearAllData(); err != nil {
		t.Fatalf("ClearAllData failed: %v", err)
	}

	// Verify symbols are gone
	symbols, err := store.GetAllSymbols()
	if err != nil {
		t.Fatalf("GetAllSymbols failed: %v", err)
	}
	if len(symbols) != 0 {
		t.Errorf("expected 0 symbols after clear, got %d", len(symbols))
	}

	// Verify method calls are gone
	calls, err := store.GetAllMethodCalls()
	if err != nil {
		t.Fatalf("GetAllMethodCalls failed: %v", err)
	}
	if len(calls) != 0 {
		t.Errorf("expected 0 method calls after clear, got %d", len(calls))
	}
}

func TestReplaceIndexData_NonRebuildPreservesManualEdges(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fyp-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	if err := store.InsertSymbol(types.Symbol{ID: "oldSym", Name: "Old.m", Kind: "method", FilePath: "Old.java", Line: 1}); err != nil {
		t.Fatalf("InsertSymbol old failed: %v", err)
	}
	if err := store.InsertMethodCall(types.MethodCall{
		CallerID: "oldCaller",
		CalleeID: "oldCallee",
		CallType:     "direct",
		FilePath:     "Old.java",
		Line:         10,
	}); err != nil {
		t.Fatalf("InsertMethodCall indexed failed: %v", err)
	}
	if err := store.InsertMethodCall(types.MethodCall{
		CallerID: "manualCaller",
		CalleeID: "manualCallee",
		CallType:     "manual",
		FilePath:     "",
		Line:         0,
	}); err != nil {
		t.Fatalf("InsertMethodCall manual failed: %v", err)
	}

	newSymbols := []types.Symbol{
		{ID: "newSym", Name: "New.m", Kind: "method", FilePath: "New.java", Line: 2},
	}
	newCalls := []types.MethodCall{
		{CallerID: "newCaller", CalleeID: "newCallee", CallType: "direct", FilePath: "New.java", Line: 12},
	}

	if err := store.ReplaceIndexData(newSymbols, newCalls, false); err != nil {
		t.Fatalf("ReplaceIndexData non-rebuild failed: %v", err)
	}

	symbols, err := store.GetAllSymbols()
	if err != nil {
		t.Fatalf("GetAllSymbols failed: %v", err)
	}
	if len(symbols) != 1 || symbols[0].ID != "newSym" {
		t.Fatalf("expected only new symbol, got %+v", symbols)
	}

	calls, err := store.GetAllMethodCalls()
	if err != nil {
		t.Fatalf("GetAllMethodCalls failed: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls (manual + new indexed), got %d", len(calls))
	}

	hasManual := false
	hasNewIndexed := false
	for _, c := range calls {
		if c.CallType == "manual" && c.CallerID == "manualCaller" && c.CalleeID == "manualCallee" {
			hasManual = true
		}
		if c.CallType == "direct" && c.CallerID == "newCaller" && c.CalleeID == "newCallee" {
			hasNewIndexed = true
		}
		if c.CallerID == "oldCaller" && c.CalleeID == "oldCallee" {
			t.Fatalf("old indexed call should have been removed, got %+v", c)
		}
	}
	if !hasManual {
		t.Fatalf("expected manual edge to be preserved")
	}
	if !hasNewIndexed {
		t.Fatalf("expected new indexed edge to be inserted")
	}
}

func TestReplaceIndexData_RebuildClearsManualEdges(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fyp-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	if err := store.InsertMethodCall(types.MethodCall{
		CallerID: "manualCaller",
		CalleeID: "manualCallee",
		CallType:     "manual",
		FilePath:     "",
		Line:         0,
	}); err != nil {
		t.Fatalf("InsertMethodCall manual failed: %v", err)
	}

	newSymbols := []types.Symbol{
		{ID: "newSym", Name: "New.m", Kind: "method", FilePath: "New.java", Line: 2},
	}
	newCalls := []types.MethodCall{
		{CallerID: "newCaller", CalleeID: "newCallee", CallType: "direct", FilePath: "New.java", Line: 12},
	}

	if err := store.ReplaceIndexData(newSymbols, newCalls, true); err != nil {
		t.Fatalf("ReplaceIndexData rebuild failed: %v", err)
	}

	calls, err := store.GetAllMethodCalls()
	if err != nil {
		t.Fatalf("GetAllMethodCalls failed: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected only rebuilt indexed call, got %d", len(calls))
	}
	if calls[0].CallType != "direct" || calls[0].CallerID != "newCaller" || calls[0].CalleeID != "newCallee" {
		t.Fatalf("unexpected call after rebuild: %+v", calls[0])
	}
}

func TestReplaceIndexData_EmptySymbolsAndCallsClearsIndexedData(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fyp-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	if err := store.InsertSymbol(types.Symbol{ID: "oldSym", Name: "Old.m", Kind: "method", FilePath: "Old.java", Line: 1}); err != nil {
		t.Fatalf("InsertSymbol old failed: %v", err)
	}
	if err := store.InsertMethodCall(types.MethodCall{
		CallerID: "oldCaller",
		CalleeID: "oldCallee",
		CallType:     "direct",
		FilePath:     "Old.java",
		Line:         10,
	}); err != nil {
		t.Fatalf("InsertMethodCall indexed failed: %v", err)
	}
	if err := store.InsertMethodCall(types.MethodCall{
		CallerID: "manualCaller",
		CalleeID: "manualCallee",
		CallType:     "manual",
		FilePath:     "",
		Line:         0,
	}); err != nil {
		t.Fatalf("InsertMethodCall manual failed: %v", err)
	}

	if err := store.ReplaceIndexData([]types.Symbol{}, []types.MethodCall{}, false); err != nil {
		t.Fatalf("ReplaceIndexData empty non-rebuild failed: %v", err)
	}

	symbols, err := store.GetAllSymbols()
	if err != nil {
		t.Fatalf("GetAllSymbols failed: %v", err)
	}
	if len(symbols) != 0 {
		t.Fatalf("expected no symbols after empty replace, got %d", len(symbols))
	}

	calls, err := store.GetAllMethodCalls()
	if err != nil {
		t.Fatalf("GetAllMethodCalls failed: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected only preserved manual edge, got %d", len(calls))
	}
	if calls[0].CallType != "manual" || calls[0].CallerID != "manualCaller" || calls[0].CalleeID != "manualCallee" {
		t.Fatalf("unexpected remaining call: %+v", calls[0])
	}
}
