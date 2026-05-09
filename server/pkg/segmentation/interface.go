package segmentation

// Segmenter segments code into smaller units (methods, functions, etc.)
type Segmenter interface {
	// SegmentClass segments a full class into individual methods
	SegmentClass(code string) ([]MethodSegment, error)
}

// MethodSegment represents a single method extracted from a class
type MethodSegment struct {
	Name       string      `json:"name"`        // Method name
	Signature  string      `json:"signature"`   // Full signature (e.g., "public static int add(int a, int b)")
	Body       string      `json:"body"`        // Method body with signature
	StartLine  int         `json:"start_line"`  // Starting line in original file
	EndLine    int         `json:"end_line"`    // Ending line in original file
	IsStatic   bool        `json:"is_static"`   // Whether method is static
	IsPublic   bool        `json:"is_public"`   // Whether method is public
	ReturnType string      `json:"return_type"` // Return type
	Parameters []Parameter `json:"parameters"`  // Method parameters
}

// Parameter represents a method parameter
type Parameter struct {
	Name string `json:"name"`
	Type string `json:"type"`
}
