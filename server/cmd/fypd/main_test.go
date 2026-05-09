package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func buildBinary(t *testing.T) string {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "fypd")

	serverDir := filepath.Join("..", "..")
	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/fypd")
	cmd.Dir = serverDir

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build binary: %v\n%s", err, string(out))
	}
	return binaryPath
}

func runCommand(binaryPath string, args ...string) (string, int) {
	cmd := exec.Command(binaryPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return string(out), ee.ExitCode()
		}
		return string(out), 1
	}
	return string(out), 0
}

func TestVersionCommand(t *testing.T) {
	binary := buildBinary(t)
	out, code := runCommand(binary, "version")
	if code != 0 {
		t.Fatalf("version failed: %s", out)
	}
	if !strings.Contains(out, "fypd version") {
		t.Fatalf("unexpected version output: %s", out)
	}
}

func TestHelpCommand(t *testing.T) {
	binary := buildBinary(t)
	out, code := runCommand(binary, "help")
	if code != 0 {
		t.Fatalf("help failed: %s", out)
	}
	if !strings.Contains(out, "Usage:") {
		t.Fatalf("unexpected help output: %s", out)
	}
	if strings.Contains(out, "index           Index a workspace") {
		t.Fatalf("deprecated index command still shown in help")
	}
	if strings.Contains(out, "callers") || strings.Contains(out, "callees") || strings.Contains(out, "validate-code") {
		t.Fatalf("removed commands still shown in help")
	}
}

func TestUnknownCommand(t *testing.T) {
	binary := buildBinary(t)
	out, code := runCommand(binary, "does-not-exist")
	if code == 0 {
		t.Fatalf("expected non-zero exit for unknown command")
	}
	if !strings.Contains(out, "Unknown command") {
		t.Fatalf("unexpected output: %s", out)
	}
}
