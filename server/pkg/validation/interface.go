package validation

// Validator validates code through multiple stages
type Validator interface {
	// ValidateFilledCode validates user-filled masked code
	// filePath: context for the code (e.g., "src/service/UserService.java")
	// code: the code snippet to validate (method, class, or any valid code)
	// Returns: results for each validation stage
	ValidateFilledCode(filePath string, code string) ([]ValidationResult, error)
}

// ValidationStage represents a stage in the validation pipeline
type ValidationStage string

const (
	StageParse ValidationStage = "parse" // Syntax check (blocking)
)

// ValidationResult represents the outcome of a validation stage
type ValidationResult struct {
	Stage       ValidationStage    `json:"stage"`
	Passed      bool               `json:"passed"`
	Errors      []ValidationError  `json:"errors"`
	Warnings    []string           `json:"warnings,omitempty"`
	Suggestions []string           `json:"suggestions,omitempty"`
	Hints       []string           `json:"hints,omitempty"`
	Metrics     *ValidationMetrics `json:"metrics,omitempty"`
}

// ValidationError represents a specific validation error
type ValidationError struct {
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Message  string `json:"message"`
	Severity string `json:"severity"` // "error" or "warning"
	Code     string `json:"code"`     // Error code (e.g., "NULL_PTR", "UNUSED_VAR")
}

// Note: ValidationError.Code is optional in old code, but required in new interface
// We'll add it when we refactor the Java validator

// ValidationMetrics provides statistics about validation
type ValidationMetrics struct {
	Duration     int `json:"duration_ms"`
	TestsPassed  int `json:"tests_passed"`
	TestsFailed  int `json:"tests_failed"`
	FuzzInputs   int `json:"fuzz_inputs"`
	FuzzFailures int `json:"fuzz_failures"`
}
