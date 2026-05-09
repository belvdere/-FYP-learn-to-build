package java

import (
	"testing"

	"example.com/fyp/pkg/validation"
)

// TestJavaValidator_ValidateFilledCode tests the main validation method
func TestJavaValidator_ValidateFilledCode(t *testing.T) {
	validator := NewJavaValidator()

	tests := []struct {
		name            string
		filePath        string
		code            string
		expectStages    int
		expectParsePass bool
	}{
		{
			name:            "Valid code",
			filePath:        "Test.java",
			code:            `public void test() { return; }`,
			expectStages:    1, // Parse only
			expectParsePass: true,
		},
		{
			name:            "Invalid syntax",
			filePath:        "Test.java",
			code:            `public void test( { return; }`,
			expectStages:    1, // Parse only
			expectParsePass: false,
		},
		{
			name:            "Full class",
			filePath:        "Service.java",
			code:            `package com.example; public class Service { public void method() { } }`,
			expectStages:    1, // Parse only
			expectParsePass: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := validator.ValidateFilledCode(tt.filePath, tt.code)
			if err != nil {
				t.Fatalf("ValidateFilledCode failed: %v", err)
			}

			if len(results) != tt.expectStages {
				t.Errorf("Expected %d stages, got %d", tt.expectStages, len(results))
			}

			if len(results) > 0 {
				parseResult := results[0]
				if parseResult.Stage != validation.StageParse {
					t.Errorf("First stage should be parse, got %s", parseResult.Stage)
				}
				if parseResult.Passed != tt.expectParsePass {
					t.Errorf("Expected parse pass: %v, got: %v", tt.expectParsePass, parseResult.Passed)
				}
			}
		})
	}
}

// TestJavaValidator_validateParse tests parse validation
func TestJavaValidator_validateParse(t *testing.T) {
	validator := NewJavaValidator()

	tests := []struct {
		name         string
		code         string
		expectPass   bool
		expectErrors bool
	}{
		{
			name:         "Valid code",
			code:         `public void test() { return; }`,
			expectPass:   true,
			expectErrors: false,
		},
		{
			name:         "Invalid syntax - missing closing brace",
			code:         `public void test() { return;`,
			expectPass:   false,
			expectErrors: true,
		},
		{
			name:         "Invalid syntax - unmatched parenthesis",
			code:         `public void test( { return; }`,
			expectPass:   false,
			expectErrors: true,
		},
		{
			name:         "Empty code",
			code:         ``,
			expectPass:   false,
			expectErrors: true,
		},
		{
			name:         "Full class",
			code:         `package com.example; public class Test { public void method() { } }`,
			expectPass:   true,
			expectErrors: false,
		},
		{
			name:         "Method with parameters",
			code:         `public int add(int a, int b) { return a + b; }`,
			expectPass:   true,
			expectErrors: false,
		},
		{
			name:         "Method with complex syntax",
			code:         `public void test() { if (x > 0) { return; } else { return; } }`,
			expectPass:   true,
			expectErrors: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.validateParse(tt.code)

			if result.Stage != validation.StageParse {
				t.Errorf("Expected stage %s, got %s", validation.StageParse, result.Stage)
			}

			if result.Passed != tt.expectPass {
				t.Errorf("Expected pass: %v, got: %v", tt.expectPass, result.Passed)
			}

			hasErrors := len(result.Errors) > 0
			if hasErrors != tt.expectErrors {
				t.Errorf("Expected errors: %v, got: %v (errors: %v)", tt.expectErrors, hasErrors, result.Errors)
			}

			if tt.expectPass && hasErrors {
				t.Errorf("Code should pass but has errors: %v", result.Errors)
			}

			if !tt.expectPass && !hasErrors {
				t.Error("Code should fail but has no errors")
			}
		})
	}
}

// TestJavaValidator_StageOrder tests that stages run in correct order
func TestJavaValidator_StageOrder(t *testing.T) {
	validator := NewJavaValidator()

	code := `public void test() { return; }`
	results, err := validator.ValidateFilledCode("Test.java", code)
	if err != nil {
		t.Fatalf("ValidateFilledCode failed: %v", err)
	}

	expectedOrder := []validation.ValidationStage{
		validation.StageParse,
	}

	if len(results) != len(expectedOrder) {
		t.Fatalf("Expected %d stages, got %d", len(expectedOrder), len(results))
	}

	for i, expected := range expectedOrder {
		if results[i].Stage != expected {
			t.Errorf("Stage %d: expected %s, got %s", i, expected, results[i].Stage)
		}
	}
}

// TestJavaValidator_EarlyExit tests that validation stops after parse failure
func TestJavaValidator_EarlyExit(t *testing.T) {
	validator := NewJavaValidator()

	code := `public void test( { return; }` // Invalid syntax
	results, err := validator.ValidateFilledCode("Test.java", code)
	if err != nil {
		t.Fatalf("ValidateFilledCode failed: %v", err)
	}

	// Should only have parse stage
	if len(results) != 1 {
		t.Errorf("Expected 1 stage (parse only), got %d", len(results))
	}

	if len(results) > 0 {
		if results[0].Stage != validation.StageParse {
			t.Errorf("Expected parse stage, got %s", results[0].Stage)
		}
		if results[0].Passed {
			t.Error("Parse should fail for invalid syntax")
		}
	}
}

// TestJavaValidator_NewJavaValidator tests validator creation
func TestJavaValidator_NewJavaValidator(t *testing.T) {
	validator := NewJavaValidator()

	if validator == nil {
		t.Fatal("Validator should not be nil")
	}

	if validator.parser == nil {
		t.Error("Parser should be initialized")
	}
}
