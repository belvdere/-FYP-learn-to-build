package ecmascript

import (
	"fmt"

	"example.com/fyp/pkg/types"
)

// ECMAScriptCallGraphExtractor is intentionally disabled because call resolution
// is handled by the LSP bridge in the extension.
type ECMAScriptCallGraphExtractor struct{}

// NewECMAScriptCallGraphExtractor creates an ECMAScript callgraph extractor.
func NewECMAScriptCallGraphExtractor() *ECMAScriptCallGraphExtractor {
	return &ECMAScriptCallGraphExtractor{}
}

// ExtractMethodCalls always returns unsupported to avoid heuristic fallback.
func (e *ECMAScriptCallGraphExtractor) ExtractMethodCalls(filePath string, sourceCode []byte) ([]types.MethodCall, error) {
	return nil, fmt.Errorf("ecmascript heuristic call graph extraction is disabled; use LSP indexing")
}
