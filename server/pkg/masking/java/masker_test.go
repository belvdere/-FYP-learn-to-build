package java

import (
	"strings"
	"testing"
)

func TestMaskCode_Simple(t *testing.T) {
	masker := NewJavaMasker()

	code := `
public void validate(User user) {
    if (user.getEmail() != null && user.getEmail().contains("@")) {
        return true;
    }
    return false;
}
`

	result, err := masker.MaskCode(code)
	if err != nil {
		t.Fatalf("MaskCode failed: %v", err)
	}

	if result == nil {
		t.Fatal("Result is nil")
	}

	if !strings.Contains(result.MaskedCode, "[MASK:") {
		t.Error("Masked code doesn't contain mask markers")
	}

	if len(result.Masks) == 0 {
		t.Error("No masks identified")
	}

	// Check mask has required fields
	mask := result.Masks[0]
	if mask.ID == "" {
		t.Error("Mask ID is empty")
	}
	if mask.Hint == "" {
		t.Error("Mask hint is empty")
	}
	if mask.OriginalCode == "" {
		t.Error("Original code is empty")
	}
}

func TestMaskCode_NoDecisionPoints(t *testing.T) {
	masker := NewJavaMasker()

	code := `
public void simple() {
    System.out.println("Hello");
}
`

	result, err := masker.MaskCode(code)
	if err != nil {
		t.Fatalf("MaskCode failed: %v", err)
	}

	// Simple code might not have masks, which is okay
	if result == nil {
		t.Fatal("Result is nil")
	}
}
