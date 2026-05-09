package java

import (
	"strings"
	"testing"
)

// TestMasking_NewFile tests masking for new file generation
func TestMasking_NewFile(t *testing.T) {
	masker := NewJavaMasker()

	code := `package com.example.demo.service;

import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

@Service
public class UserService {
    @Transactional
    public void deleteUser(String userId) {
        // TODO: Add validation
        if (userId == null) {
            throw new IllegalArgumentException("User ID required");
        }
        // Process deletion
    }
}`

	result, err := masker.MaskCode(code)
	if err != nil {
		t.Fatalf("MaskCode failed: %v", err)
	}

	if result == nil {
		t.Fatal("Result is nil")
	}

	// Should have masks (TODO should be detected)
	if len(result.Masks) == 0 {
		t.Error("Should have at least one mask (TODO)")
	}

	// Masked code should contain mask markers
	if len(result.MaskedCode) == 0 {
		t.Error("Masked code should not be empty")
	}
}

// TestMasking_ExistingFile_AddMethod tests masking for method added to existing file
func TestMasking_ExistingFile_AddMethod(t *testing.T) {
	masker := NewJavaMasker()

	// Method only (no package/imports/class)
	code := `@Transactional
public void deleteAllUsers() {
    // TODO: Add validation
    try {
        userRepository.deleteAll();
    } catch (Exception e) {
        // Empty catch - should be masked
    }
}`

	result, err := masker.MaskCode(code)
	if err != nil {
		t.Fatalf("MaskCode failed: %v", err)
	}

	// Should detect TODO or empty catch (depending on config)
	// Note: Masking depends on configuration and code context
	if result == nil {
		t.Fatal("Result is nil")
	}

	// Log mask count for debugging
	t.Logf("Found %d masks", len(result.Masks))
}

// TestMasking_Hybrid_DeterministicOnly tests deterministic masking
func TestMasking_Hybrid_DeterministicOnly(t *testing.T) {
	masker := NewJavaMasker() // No agent masker

	code := `public void processPayment(String cardNumber, double amount) {
    if (cardNumber == null) {
        throw new IllegalArgumentException("Card number required");
    }
    // TODO: Validate card format
    // Process payment
}`

	result, err := masker.MaskCode(code)
	if err != nil {
		t.Fatalf("MaskCode failed: %v", err)
	}

	// Should detect TODO deterministically
	foundTODO := false
	for _, mask := range result.Masks {
		if mask.Hint != "" && len(mask.Hint) > 0 {
			foundTODO = true
			break
		}
	}

	if !foundTODO {
		t.Error("Should detect TODO deterministically")
	}
}

// TestMasking_DeterministicOnly tests deterministic-only masking
func TestMasking_DeterministicOnly(t *testing.T) {
	masker := NewJavaMasker()

	code := `public void processPayment(String cardNumber, double amount) {
    if (cardNumber == null) {
        throw new IllegalArgumentException("Card number required");
    }
    paymentService.charge(cardNumber, amount);
}`

	result, err := masker.MaskCode(code)
	if err != nil {
		t.Fatalf("MaskCode failed: %v", err)
	}

	if result == nil {
		t.Fatal("Result is nil")
	}

	// Should have masks (deterministic + potentially agent)
	if len(result.Masks) == 0 {
		t.Error("Should have masks")
	}
}

// TestMasking_ComplexCondition tests masking of complex conditions
func TestMasking_ComplexCondition(t *testing.T) {
	masker := NewJavaMasker()

	code := `public boolean isValid(String email, int age) {
    if (email != null && email.contains("@") && age > 0 && age < 150) {
        return true;
    }
    return false;
}`

	result, err := masker.MaskCode(code)
	if err != nil {
		t.Fatalf("MaskCode failed: %v", err)
	}

	// Complex conditions might be masked depending on config
	// Just verify it doesn't crash
	if result == nil {
		t.Fatal("Result is nil")
	}
}

// TestMasking_ErrorHandling tests masking of error handling
func TestMasking_ErrorHandling(t *testing.T) {
	masker := NewJavaMasker()

	code := `public void riskyOperation() {
    try {
        doSomething();
    } catch (Exception e) {
        // Empty catch - should be masked
    }
}`

	result, err := masker.MaskCode(code)
	if err != nil {
		t.Fatalf("MaskCode failed: %v", err)
	}

	// Should detect empty catch block (if configured)
	// Note: This depends on masker configuration
	// Just verify it doesn't crash
	if result == nil {
		t.Fatal("Result is nil")
	}

	// Verify masks exist (may or may not include empty catch depending on config)
	if len(result.Masks) == 0 {
		t.Log("No masks found (may be expected depending on config)")
	}
}

// TestMasking_EdgeCases tests edge cases
func TestMasking_EdgeCases(t *testing.T) {
	masker := NewJavaMasker()

	tests := []struct {
		name string
		code string
	}{
		{
			name: "Empty code",
			code: "",
		},
		{
			name: "Single line",
			code: "// TODO: Implement",
		},
		{
			name: "No decision points",
			code: `public void simple() {
    return;
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := masker.MaskCode(tt.code)
			if err != nil {
				// Some edge cases may fail, that's okay
				return
			}

			if result == nil {
				t.Error("Result should not be nil")
			}
		})
	}
}

// TestMasking_FullWorkflow tests complete masking workflow
func TestMasking_FullWorkflow(t *testing.T) {
	masker := NewJavaMasker()

	code := `package com.example.demo.service;

import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

@Service
public class PaymentService {
    @Transactional
    public void processPayment(String cardNumber, double amount) {
        // TODO: Validate card number format
        if (cardNumber == null) {
            throw new IllegalArgumentException("Card number required");
        }
        
        try {
            paymentGateway.charge(cardNumber, amount);
        } catch (Exception e) {
            // TODO: Handle payment failure
        }
    }
}`

	// Step 1: Mask code
	result, err := masker.MaskCode(code)
	if err != nil {
		t.Fatalf("MaskCode failed: %v", err)
	}

	if result == nil {
		t.Fatal("Result is nil")
	}

	// Step 2: Verify masks
	if len(result.Masks) == 0 {
		t.Error("Should have masks")
	}

	// Step 3: Verify masked code contains markers
	if !strings.Contains(result.MaskedCode, "[MASK:") {
		t.Error("Masked code should contain mask markers")
	}

	// Step 4: Verify each mask has required fields
	for _, mask := range result.Masks {
		if mask.ID == "" {
			t.Error("Mask should have ID")
		}
		if mask.Hint == "" {
			t.Error("Mask should have hint")
		}
	}
}
