package mcp

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const defaultOptimizeTimeout = 30 * time.Second

// runPromptOptimizer runs the Python TextGrad optimizer on the given content.
// Returns optimized content on success, or error. When optimization fails,
// the caller should fall back to the original content.
func runPromptOptimizer(workspaceRoot string, content string) (string, error) {
	path, err := resolveOptimizerPath(workspaceRoot)
	if err != nil {
		return "", err
	}

	timeout := defaultOptimizeTimeout
	if s := os.Getenv("FYP_OPTIMIZE_TIMEOUT"); s != "" {
		if sec, err := strconv.Atoi(s); err == nil && sec > 0 {
			timeout = time.Duration(sec) * time.Second
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", path, "--steps", "1")
	cmd.Stdin = strings.NewReader(content)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Load .fyp/.env into subprocess environment (OPENAI_API_KEY etc.)
	cmd.Env = mergedEnv(workspaceRoot)

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("optimizer failed: %w (stderr: %s)", err, stderr.String())
	}

	return stdout.String(), nil
}

func resolveOptimizerPath(workspaceRoot string) (string, error) {
	if p := os.Getenv("FYP_PROMPT_OPTIMIZER_PATH"); p != "" {
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("FYP_PROMPT_OPTIMIZER_PATH=%s: %w", p, err)
		}
		return p, nil
	}

	// Try workspace-relative: when devving from monorepo, prompt-optimizer is at repo root
	candidates := []string{
		filepath.Join(workspaceRoot, "prompt-optimizer", "optimize.py"),
		filepath.Join(workspaceRoot, ".fyp", "prompt-optimizer", "optimize.py"),
	}

	// Try relative to executable (for packaged extension: extension/prompt-optimizer/optimize.py)
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		candidates = append(candidates,
			filepath.Join(execDir, "..", "prompt-optimizer", "optimize.py"),
			filepath.Join(execDir, "..", "..", "prompt-optimizer", "optimize.py"),
		)
	}

	for _, p := range candidates {
		if abs, err := filepath.Abs(p); err == nil {
			if _, err := os.Stat(abs); err == nil {
				return abs, nil
			}
		}
	}

	return "", fmt.Errorf("optimize.py not found: set FYP_PROMPT_OPTIMIZER_PATH or place prompt-optimizer in workspace")
}

// loadFypEnvIntoProcess loads workspaceRoot/.fyp/.env and sets vars in the current process.
// Enables FYP_OPTIMIZE_PROMPTS and OPENAI_API_KEY to be configured via .fyp/.env.
func loadFypEnvIntoProcess(workspaceRoot string) {
	envPath := filepath.Join(workspaceRoot, ".fyp", ".env")
	vars, err := loadEnvFromFile(envPath)
	if err != nil {
		return
	}
	for k, v := range vars {
		os.Setenv(k, v)
	}
}

// loadEnvFromFile parses a .env file (KEY=value, # comments) and returns env vars.
func loadEnvFromFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	vars := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if idx := strings.Index(line, "="); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+1:])
			if key != "" {
				// Remove surrounding quotes
				if len(val) >= 2 && (val[0] == '"' && val[len(val)-1] == '"' || val[0] == '\'' && val[len(val)-1] == '\'') {
					val = val[1 : len(val)-1]
				}
				vars[key] = val
			}
		}
	}
	return vars, scanner.Err()
}

// mergedEnv returns os.Environ() merged with vars from workspaceRoot/.fyp/.env.
// Vars from .env override existing environment.
func mergedEnv(workspaceRoot string) []string {
	envPath := filepath.Join(workspaceRoot, ".fyp", ".env")
	vars, err := loadEnvFromFile(envPath)
	if err != nil {
		return os.Environ()
	}
	if len(vars) == 0 {
		return os.Environ()
	}

	existing := make(map[string]string)
	for _, s := range os.Environ() {
		if idx := strings.Index(s, "="); idx > 0 {
			existing[s[:idx]] = s[idx+1:]
		}
	}
	for k, v := range vars {
		existing[k] = v
	}
	out := make([]string, 0, len(existing))
	for k, v := range existing {
		out = append(out, k+"="+v)
	}
	return out
}
