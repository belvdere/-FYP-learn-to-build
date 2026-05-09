package masking

import "time"

// MaskType represents the type of decision point being masked
type MaskType string

const (
	MaskTypeCondition  MaskType = "condition"  // if/else, switch, ternary
	MaskTypeReturn     MaskType = "return"     // return statements
	MaskTypeValidation MaskType = "validation" // input checks, null checks
	MaskTypeError      MaskType = "error"      // exception handling
)

// MaskState represents the lifecycle state of a mask
type MaskState string

const (
	MaskStatePending    MaskState = "pending"    // Mask created, not filled
	MaskStateFilled     MaskState = "filled"     // User filled in code
	MaskStateValidating MaskState = "validating" // Validation in progress
	MaskStateValid      MaskState = "valid"      // Validation passed
	MaskStateInvalid    MaskState = "invalid"    // Validation failed
	MaskStateResolved   MaskState = "resolved"   // Mask resolved to final code
)

// Mask represents a single masked decision point
type Mask struct {
	ID           string   `json:"id"`
	Type         MaskType `json:"type"`
	Hint         string   `json:"hint"`
	OriginalCode string   `json:"originalCode"`
	Range        Range    `json:"range"`
}

// Range represents a position range in source code
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Position represents a line/character position
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// MaskResult is the output of the masking service
type MaskResult struct {
	MaskedCode string `json:"maskedCode"`
	Masks      []Mask `json:"masks"`
}

// MaskMetadata is the persisted mask information
type MaskMetadata struct {
	ID              string     `json:"id"`
	FilePath        string     `json:"filePath"`
	Range           Range      `json:"range"`
	Hint            string     `json:"hint"`
	Type            MaskType   `json:"type"`
	State           MaskState  `json:"state"`
	OriginalCode    string     `json:"originalCode"`
	OriginalAST     string     `json:"originalAST"`     // Serialized AST structure
	ExpectedPattern string     `json:"expectedPattern"` // For semantic similarity
	CreatedAt       time.Time  `json:"createdAt"`
	FilledAt        *time.Time `json:"filledAt,omitempty"`
	UserInput       string     `json:"userInput,omitempty"`
	ValidatedAt     *time.Time `json:"validatedAt,omitempty"`
	ResolvedAt      *time.Time `json:"resolvedAt,omitempty"`

	// Validation metadata for Phase 4
	Dependencies     []string          `json:"dependencies,omitempty"`     // Referenced symbols
	ExpectedBehavior map[string]string `json:"expectedBehavior,omitempty"` // For functional tests
	HasReferences    bool              `json:"hasReferences"`              // Function has references
}

// DecisionPoint represents an identified point in code to mask
type DecisionPoint struct {
	Type         MaskType
	Node         interface{} // *sitter.Node
	StartByte    uint32
	EndByte      uint32
	StartLine    int
	EndLine      int
	Hint         string
	OriginalCode string
	Priority     int
}

// MaskingConfig configures the masking behavior
type MaskingConfig struct {
	MaskValidation      bool `json:"maskValidation"`
	MaskReturns         bool `json:"maskReturns"`
	MaskErrors          bool `json:"maskErrors"`
	MaskReferencedFuncs bool `json:"maskReferencedFuncs"`
	MaxMasksPerFunction int  `json:"maxMasksPerFunction"`
	ComplexityThreshold int  `json:"complexityThreshold"`
}

// DefaultMaskingConfig returns the default masking configuration
func DefaultMaskingConfig() *MaskingConfig {
	return &MaskingConfig{
		MaskValidation:      true,
		MaskReturns:         false, // Don't mask simple returns
		MaskErrors:          true,
		MaskReferencedFuncs: true,
		MaxMasksPerFunction: 3,
		ComplexityThreshold: 1,
	}
}
