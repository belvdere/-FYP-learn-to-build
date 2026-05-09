package api

import (
	"fmt"
	"os"

	"example.com/fyp/pkg/db"
	"example.com/fyp/pkg/validation"
)

// ============================================================================
// Validation handlers
// ============================================================================

// handleValidationRun validates filled masked code (parse-only).
// Params: code (string), filePath (string), sessionId (string)
func (s *StdioServer) handleValidationRun(req *StdioRequest) {
	code := s.getStringParam(req, "code")
	filePath := s.getStringParam(req, "filePath")
	sessionID := s.getStringParam(req, "sessionId")

	fmt.Fprintf(os.Stderr, "fypd: [validation] Starting validation for session=%s file=%s\n", sessionID, filePath)
	fmt.Fprintf(os.Stderr, "fypd: [validation] Code length: %d bytes\n", len(code))

	if code == "" {
		fmt.Fprintln(os.Stderr, "fypd: [validation] Error: code is required")
		s.sendError(req.ID, ErrInvalidParams, "code is required", nil)
		return
	}
	if filePath == "" {
		fmt.Fprintln(os.Stderr, "fypd: [validation] Error: filePath is required")
		s.sendError(req.ID, ErrInvalidParams, "filePath is required", nil)
		return
	}
	if sessionID == "" {
		fmt.Fprintln(os.Stderr, "fypd: [validation] Error: sessionId is required")
		s.sendError(req.ID, ErrInvalidParams, "sessionId is required", nil)
		return
	}

	fmt.Fprintf(os.Stderr, "fypd: [validation] Running stage: parse\n")

	// Get plugin for file type
	plugin, err := s.langController.GetPlugin(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fypd: [validation] Error: unsupported language for file %s\n", filePath)
		s.sendError(req.ID, ErrInvalidParams, "Unsupported language", err.Error())
		return
	}
	fmt.Fprintf(os.Stderr, "fypd: [validation] Using language plugin: %s\n", plugin.Language())

	// Create parse-only validator
	validator := plugin.Validator()

	// Run parse validation
	fmt.Fprintln(os.Stderr, "fypd: [validation] Executing parse validation...")
	results, err := validator.ValidateFilledCode(filePath, code)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fypd: [validation] Pipeline error: %v\n", err)
		s.sendError(req.ID, ErrInternalError, "Validation failed", err.Error())
		return
	}

	// Log each result
	for _, result := range results {
		status := "PASS"
		if !result.Passed {
			status = "FAIL"
		}
		errorCount := 0
		if result.Errors != nil {
			errorCount = len(result.Errors)
		}

		fmt.Fprintf(os.Stderr, "fypd: [validation] Stage '%s': %s (errors=%d)\n",
			result.Stage, status, errorCount)

		if result.Errors != nil {
			for _, e := range result.Errors {
				fmt.Fprintf(os.Stderr, "fypd: [validation]   - Error: %s\n", e.Message)
			}
		}
	}

	// Check overall pass (parse must pass)
	allPassed := s.checkValidationPass(results)

	// Log overall result
	if allPassed {
		fmt.Fprintf(os.Stderr, "fypd: [validation] Overall result: PASSED (session=%s)\n", sessionID)
	} else {
		fmt.Fprintf(os.Stderr, "fypd: [validation] Overall result: FAILED (session=%s)\n", sessionID)
	}

	// Update session in database
	if err := s.store.UpdateFilledCode(sessionID, code); err != nil {
		fmt.Fprintf(os.Stderr, "fypd: [validation] Warning: failed to update filled code: %v\n", err)
	}

	status := db.StatusFilled
	if allPassed {
		status = db.StatusValidated
	}
	if err := s.store.UpdateStatus(sessionID, status); err != nil {
		fmt.Fprintf(os.Stderr, "fypd: [validation] Warning: failed to update session status: %v\n", err)
	}
	fmt.Fprintf(os.Stderr, "fypd: [validation] Session status updated to: %s\n", status)

	s.sendResponse(req.ID, map[string]interface{}{
		"passed":  allPassed,
		"results": results,
	})
}

// checkValidationPass checks if parse validation passed.
func (s *StdioServer) checkValidationPass(results []validation.ValidationResult) bool {
	for _, result := range results {
		if result.Stage == validation.StageParse && !result.Passed {
			return false
		}
	}
	for _, result := range results {
		if result.Stage == validation.StageParse {
			return true
		}
	}
	return false
}
