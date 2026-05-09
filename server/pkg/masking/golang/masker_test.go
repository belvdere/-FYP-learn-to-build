package golang

import (
	"strings"
	"testing"

	"example.com/fyp/pkg/masking"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoMasker_ErrorCheck(t *testing.T) {
	code := `package main

import "errors"

func doSomething() error {
	err := errors.New("oops")
	if err != nil {
		return err
	}
	return nil
}
`
	cfg := masking.DefaultMaskingConfig()
	cfg.ComplexityThreshold = 1
	m := NewGoMaskerWithConfig(cfg)

	result, err := m.MaskCode(code)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Masks)
	assert.Contains(t, result.MaskedCode, "[MASK:")
}

func TestGoMasker_ConditionMask(t *testing.T) {
	code := `package main

func check(x int, y int) bool {
	if x > 0 && y > 0 && x != y {
		return true
	}
	return false
}
`
	cfg := masking.DefaultMaskingConfig()
	cfg.ComplexityThreshold = 1
	m := NewGoMaskerWithConfig(cfg)

	result, err := m.MaskCode(code)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Masks)
}

func TestGoMasker_SessionInjection(t *testing.T) {
	code := `package main

func f() error {
	var err error
	if err != nil {
		return err
	}
	return nil
}
`
	cfg := masking.DefaultMaskingConfig()
	cfg.ComplexityThreshold = 1
	m := NewGoMaskerWithConfig(cfg)

	result, err := m.MaskCode(code)
	require.NoError(t, err)
	require.NotEmpty(t, result.Masks)

	// Inject session ID like MaskCodeWithStore does
	injected := strings.ReplaceAll(result.MaskedCode, "/* [MASK:id=", "/* [MASK:session=test-uuid id=")
	assert.Contains(t, injected, "session=test-uuid")
}

func TestGoMasker_SyntaxError(t *testing.T) {
	code := `package main

func broken( {
`
	m := NewGoMasker()
	_, err := m.MaskCode(code)
	assert.Error(t, err)
}
