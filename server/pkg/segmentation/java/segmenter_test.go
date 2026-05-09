package java

import (
	"strings"
	"testing"
)

func TestJavaSegmenter_SegmentClass(t *testing.T) {
	tests := []struct {
		name           string
		code           string
		expectMethods  int
		expectNames    []string
		expectStatic   []bool
		expectRetTypes []string
	}{
		{
			name: "Simple class with multiple methods",
			code: `public class Calculator {
				public static int add(int a, int b) {
					return a + b;
				}
				
				public int multiply(int a, int b) {
					return a * b;
				}
				
				private void log(String message) {
					System.out.println(message);
				}
			}`,
			expectMethods:  3,
			expectNames:    []string{"add", "multiply", "log"},
			expectStatic:   []bool{true, false, false},
			expectRetTypes: []string{"int", "int", "void"},
		},
		{
			name: "Spring service class",
			code: `package com.example.demo;
			
			import org.springframework.stereotype.Service;
			
			@Service
			public class SegmentationProcessor {
				public void processSegmentation(Map<String, Object> message) {
					String id = (String) message.get("id");
					if (id == null) return;
				}
				
				private void handleError(String id, Throwable ex) {
					logger.error("Error: {}", ex.getMessage());
				}
			}`,
			expectMethods:  2,
			expectNames:    []string{"processSegmentation", "handleError"},
			expectStatic:   []bool{false, false},
			expectRetTypes: []string{"void", "void"},
		},
		{
			name: "Class with constructor (should not be extracted as method)",
			code: `public class User {
				private String name;
				
				public User(String name) {
					this.name = name;
				}
				
				public String getName() {
					return name;
				}
			}`,
			expectMethods:  1, // Only getName, not constructor
			expectNames:    []string{"getName"},
			expectStatic:   []bool{false},
			expectRetTypes: []string{"String"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segmenter := NewJavaSegmenter()
			methods, err := segmenter.SegmentClass(tt.code)

			if err != nil {
				t.Fatalf("SegmentClass() error = %v", err)
			}

			if len(methods) != tt.expectMethods {
				t.Errorf("Expected %d methods, got %d", tt.expectMethods, len(methods))
			}

			for i := 0; i < len(methods) && i < len(tt.expectNames); i++ {
				if methods[i].Name != tt.expectNames[i] {
					t.Errorf("Method %d: expected name %s, got %s", i, tt.expectNames[i], methods[i].Name)
				}
				if methods[i].IsStatic != tt.expectStatic[i] {
					t.Errorf("Method %d (%s): expected static=%v, got %v", i, methods[i].Name, tt.expectStatic[i], methods[i].IsStatic)
				}
				if methods[i].ReturnType != tt.expectRetTypes[i] {
					t.Errorf("Method %d (%s): expected return type %s, got %s", i, methods[i].Name, tt.expectRetTypes[i], methods[i].ReturnType)
				}
				if methods[i].Body == "" {
					t.Errorf("Method %d (%s): body is empty", i, methods[i].Name)
				}
				if methods[i].StartLine == 0 || methods[i].EndLine == 0 {
					t.Errorf("Method %d (%s): invalid line numbers (start=%d, end=%d)", i, methods[i].Name, methods[i].StartLine, methods[i].EndLine)
				}
			}
		})
	}
}

func TestJavaSegmenter_SegmentClass_Errors(t *testing.T) {
	tests := []struct {
		name        string
		code        string
		expectError string
	}{
		{
			name:        "Invalid syntax",
			code:        "public class Invalid {",
			expectError: "syntax errors",
		},
		{
			name:        "No methods - empty class",
			code:        "public class Empty { }",
			expectError: "no methods found",
		},
		{
			name:        "No methods - just a variable",
			code:        "public int x = 5;",
			expectError: "no methods found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segmenter := NewJavaSegmenter()
			_, err := segmenter.SegmentClass(tt.code)

			if err == nil {
				t.Fatalf("Expected error containing '%s', got nil", tt.expectError)
			}
		})
	}
}

func TestJavaSegmenter_MethodBody(t *testing.T) {
	code := `public class Test {
		public static int add(int a, int b) {
			return a + b;
		}
	}`

	segmenter := NewJavaSegmenter()
	methods, err := segmenter.SegmentClass(code)

	if err != nil {
		t.Fatalf("SegmentClass() error = %v", err)
	}

	if len(methods) != 1 {
		t.Fatalf("Expected 1 method, got %d", len(methods))
	}

	method := methods[0]

	// Body should be the complete method including signature
	if !strings.Contains(method.Body, "public static int add") {
		t.Errorf("Method body should contain signature, got: %s", method.Body)
	}
	if !strings.Contains(method.Body, "return a + b") {
		t.Errorf("Method body should contain implementation, got: %s", method.Body)
	}

	// Signature should be clean
	expectedSig := "public static int add(int a, int b)"
	if method.Signature != expectedSig {
		t.Errorf("Expected signature '%s', got '%s'", expectedSig, method.Signature)
	}
}
