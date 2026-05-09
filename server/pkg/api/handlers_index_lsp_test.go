package api

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"example.com/fyp/pkg/db"
	"example.com/fyp/pkg/types"
)

func newTestIndexServer(t *testing.T) (*StdioServer, *db.Store, *bytes.Buffer) {
	t.Helper()

	workspace := t.TempDir()
	store, err := db.NewStore(workspace)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	var out bytes.Buffer
	server := &StdioServer{
		store:         store,
		workspaceRoot: workspace,
		writer:        &out,
	}

	return server, store, &out
}

func decodeSingleRPCResponse(t *testing.T, out *bytes.Buffer) map[string]interface{} {
	t.Helper()

	var resp map[string]interface{}
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp); err != nil {
		t.Fatalf("failed to decode response %q: %v", out.String(), err)
	}
	return resp
}

func TestHandleIndexStoreEdges_AllowsEmptySymbols(t *testing.T) {
	server, store, out := newTestIndexServer(t)

	if err := store.InsertSymbol(types.Symbol{
		ID:       "oldSym",
		Name:     "Old.m",
		Kind:     "method",
		FilePath: "Old.java",
		Line:     1,
	}); err != nil {
		t.Fatalf("InsertSymbol failed: %v", err)
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

	server.handleIndexStoreEdges(&StdioRequest{
		ID:     1,
		Method: "index.storeEdges",
		Params: map[string]interface{}{
			"rebuild": false,
			"symbols": []interface{}{},
			"edges":   []interface{}{},
		},
	})

	resp := decodeSingleRPCResponse(t, out)
	if _, hasError := resp["error"]; hasError {
		t.Fatalf("expected success response, got error: %v", resp["error"])
	}

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result object, got %T", resp["result"])
	}
	if success, ok := result["success"].(bool); !ok || !success {
		t.Fatalf("expected success=true, got %v", result["success"])
	}

	symbols, err := store.GetAllSymbols()
	if err != nil {
		t.Fatalf("GetAllSymbols failed: %v", err)
	}
	if len(symbols) != 0 {
		t.Fatalf("expected no symbols after empty index store, got %d", len(symbols))
	}

	calls, err := store.GetAllMethodCalls()
	if err != nil {
		t.Fatalf("GetAllMethodCalls failed: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected only manual edge to remain, got %d calls", len(calls))
	}
	if calls[0].CallType != "manual" || calls[0].CallerID != "manualCaller" || calls[0].CalleeID != "manualCallee" {
		t.Fatalf("unexpected remaining call: %+v", calls[0])
	}
}

func TestHandleIndexStoreEdges_RejectsMissingSymbols(t *testing.T) {
	server, _, out := newTestIndexServer(t)

	server.handleIndexStoreEdges(&StdioRequest{
		ID:     2,
		Method: "index.storeEdges",
		Params: map[string]interface{}{
			"rebuild": false,
			"edges":   []interface{}{},
		},
	})

	resp := decodeSingleRPCResponse(t, out)
	errObj, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error response, got: %v", resp)
	}
	if msg, _ := errObj["message"].(string); msg != "symbols is required" {
		t.Fatalf("unexpected error message: %q", msg)
	}
}

func TestHandleIndexScanFile_ReportsChangedCallers(t *testing.T) {
	server, store, out := newTestIndexServer(t)

	filePath := filepath.Join(server.workspaceRoot, "UserService.java")
	codeV1 := `
public class UserService {
  public void save(String id) { System.out.println(id); }
}`
	if err := os.WriteFile(filePath, []byte(codeV1), 0644); err != nil {
		t.Fatalf("write java file v1: %v", err)
	}

	server.handleIndexScanFile(&StdioRequest{
		ID:     10,
		Method: "index.scanFile",
		Params: map[string]interface{}{"filePath": filePath},
	})
	resp := decodeSingleRPCResponse(t, out)
	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result object, got %T", resp["result"])
	}
	symbolsRaw, ok := result["symbols"].([]interface{})
	if !ok || len(symbolsRaw) == 0 {
		t.Fatalf("expected non-empty symbols in scanFile response, got %v", result["symbols"])
	}
	firstSymbol, ok := symbolsRaw[0].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected symbol payload: %T", symbolsRaw[0])
	}

	if err := store.InsertSymbol(types.Symbol{
		ID:       toString(firstSymbol["id"]),
		Name:     toString(firstSymbol["name"]),
		Kind:     toString(firstSymbol["kind"]),
		FilePath: toString(firstSymbol["filePath"]),
		Line:     toInt(firstSymbol["line"]),
		EndLine:  toInt(firstSymbol["endLine"]),
		BodyHash: toString(firstSymbol["bodyHash"]),
	}); err != nil {
		t.Fatalf("InsertSymbol baseline failed: %v", err)
	}

	out.Reset()
	codeV2 := `
public class UserService {
  public void save(String id) { System.out.println(id + "!"); }
}`
	if err := os.WriteFile(filePath, []byte(codeV2), 0644); err != nil {
		t.Fatalf("write java file v2: %v", err)
	}

	server.handleIndexScanFile(&StdioRequest{
		ID:     11,
		Method: "index.scanFile",
		Params: map[string]interface{}{"filePath": filePath},
	})
	resp2 := decodeSingleRPCResponse(t, out)
	result2, ok := resp2["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result object, got %T", resp2["result"])
	}
	changedRaw, ok := result2["changedCallerIds"].([]interface{})
	if !ok {
		t.Fatalf("expected changedCallerIds array, got %T", result2["changedCallerIds"])
	}
	if len(changedRaw) == 0 {
		t.Fatalf("expected changed caller IDs after body edit")
	}
	if toString(result2["fileHash"]) == "" {
		t.Fatalf("expected non-empty fileHash")
	}
}

func TestHandleIndexStoreFileDelta_AppliesIncrementalUpdate(t *testing.T) {
	server, store, out := newTestIndexServer(t)

	filePath := filepath.Join(server.workspaceRoot, "UserService.java")
	if err := store.InsertSymbol(types.Symbol{
		ID:       "oldCaller",
		Name:     "UserService.save(String)",
		Kind:     "method",
		FilePath: filePath,
		Line:     2,
		EndLine:  3,
	}); err != nil {
		t.Fatalf("InsertSymbol old failed: %v", err)
	}
	if err := store.InsertMethodCall(types.MethodCall{
		CallerID: "oldCaller",
		CalleeID: "oldTarget",
		CallType:     "direct",
		FilePath:     filePath,
		Line:         3,
	}); err != nil {
		t.Fatalf("InsertMethodCall old direct failed: %v", err)
	}
	if err := store.InsertMethodCall(types.MethodCall{
		CallerID: "oldCaller",
		CalleeID: "manualTarget",
		CallType:     "manual",
		FilePath:     "",
		Line:         0,
	}); err != nil {
		t.Fatalf("InsertMethodCall manual failed: %v", err)
	}

	server.handleIndexStoreFileDelta(&StdioRequest{
		ID:     12,
		Method: "index.storeFileDelta",
		Params: map[string]interface{}{
			"filePath":         filePath,
			"fileHash":         "hash_v1",
			"symbols":          []interface{}{},
			"changedCallerIds": []interface{}{"oldCaller"},
			"removedCallerIds": []interface{}{"oldCaller"},
			"edges":            []interface{}{},
		},
	})

	resp := decodeSingleRPCResponse(t, out)
	if _, hasError := resp["error"]; hasError {
		t.Fatalf("expected success response, got error: %v", resp["error"])
	}

	symbols, err := store.GetSymbolsByFile(filePath)
	if err != nil {
		t.Fatalf("GetSymbolsByFile failed: %v", err)
	}
	if len(symbols) != 0 {
		t.Fatalf("expected file symbols removed by empty delta, got %d", len(symbols))
	}

	calls, err := store.GetAllMethodCalls()
	if err != nil {
		t.Fatalf("GetAllMethodCalls failed: %v", err)
	}
	if len(calls) != 1 || calls[0].CallType != "manual" {
		t.Fatalf("expected only manual edge to remain, got %+v", calls)
	}

	cache, err := store.GetCacheEntry(filePath)
	if err != nil {
		t.Fatalf("GetCacheEntry failed: %v", err)
	}
	if cache == nil || cache.FileHash != "hash_v1" {
		t.Fatalf("expected file cache updated, got %+v", cache)
	}
}
