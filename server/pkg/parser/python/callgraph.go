package python

import (
	"fmt"

	"example.com/fyp/pkg/types"
)

// PythonCallGraphExtractor is intentionally disabled because call resolution
// is handled by the LSP bridge in the extension.
type PythonCallGraphExtractor struct{}

// NewPythonCallGraphExtractor creates a Python call graph extractor.
func NewPythonCallGraphExtractor() *PythonCallGraphExtractor {
	return &PythonCallGraphExtractor{}
}

// ExtractMethodCalls always returns an explicit unsupported error.
func (e *PythonCallGraphExtractor) ExtractMethodCalls(filePath string, sourceCode []byte) ([]types.MethodCall, error) {
	return nil, fmt.Errorf("python heuristic call graph extraction is disabled; use LSP indexing")
}
