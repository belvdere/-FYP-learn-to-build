package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"example.com/fyp/pkg/db"
	"example.com/fyp/pkg/segmentation"
	"example.com/fyp/pkg/types"
	"example.com/fyp/pkg/validation"
)

// handleResourcesList lists available resources
func (s *Server) handleResourcesList(req *Request) {
	s.logFeatureUsage("resources/list", "called", nil)

	// No resources in simplified mode
	resources := []Resource{}

	result := map[string]interface{}{
		"resources": resources,
	}

	s.sendResponse(req.ID, result)
	s.logFeatureUsage("resources/list", "success", map[string]interface{}{
		"count": len(resources),
	})
}

// handleResourcesRead reads a specific resource
func (s *Server) handleResourcesRead(req *Request) {
	s.logFeatureUsage("resources/read", "called", nil)

	// No resources in simplified mode
	s.sendError(req.ID, InvalidParams, "No resources available", nil)
}

// handleToolsList lists available tools
func (s *Server) handleToolsList(req *Request) {
	s.logFeatureUsage("tools/list", "called", nil)

	tools := []Tool{
		{
			Name:        "kg.getSnapshot",
			Description: "Retrieve a graph snapshot by ID. Returns context (virtual nodes, context nodes, relationships) to inform your response. Use when the user mentions a snapshot ID (e.g., 'snap_abc123'). Use as additional context only; proceed with whatever the user asked.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"snapshotId": map[string]interface{}{
						"type":        "string",
						"description": "The snapshot ID (e.g., 'snap_abc123')",
					},
				},
				"required": []string{"snapshotId"},
			},
		},
		{
			Name:        "kg.maskCode",
			Description: "Mask generated code for a learning exercise. Use this when the user's prompt contains '--mask' or asks for 'masking'. Returns masked code with [MASK] placeholders. You MUST write the returned masked code to the specified file path, then tell the user to fill in the [MASK] placeholders. When the user is done filling, they will ask you to validate.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"code": map[string]interface{}{
						"type":        "string",
						"description": "The generated code to mask (full class or methods)",
					},
					"filePath": map[string]interface{}{
						"type":        "string",
						"description": "Target file path for the code (e.g., 'src/main/java/AuthService.java' or 'src/app/service.py')",
					},
					"language": map[string]interface{}{
						"type":        "string",
						"description": "Programming language (optional; detected from file extension when omitted)",
					},
				},
				"required": []string{"code", "filePath"},
			},
		},
		{
			Name:        "kg.validateFilledCode",
			Description: "Validate user-filled masked code. Use when the user says 'validate' or 'check my masks'. Returns parse results and original code. You MUST compare the filled code against the original and give semantic feedback (hints, congratulations) in the same response — do not stop at 'parse passed' and offer comparison as a separate step.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"sessionId": map[string]interface{}{
						"type":        "string",
						"description": "The mask session ID returned by kg.maskCode",
					},
					"filledCode": map[string]interface{}{
						"type":        "string",
						"description": "The user's filled code (read from the file after they fill in the masks)",
					},
					"filePath": map[string]interface{}{
						"type":        "string",
						"description": "The file path of the code being validated",
					},
				},
				"required": []string{"sessionId", "filledCode", "filePath"},
			},
		},
	}

	result := map[string]interface{}{
		"tools": tools,
	}

	s.sendResponse(req.ID, result)
	s.logFeatureUsage("tools/list", "success", map[string]interface{}{
		"count": len(tools),
	})
}

// handleToolsCall handles tool invocations
func (s *Server) handleToolsCall(req *Request) {
	// Parse params
	paramsData, err := json.Marshal(req.Params)
	if err != nil {
		s.logFeatureUsage("tools/call", "error", map[string]interface{}{
			"error": "failed to marshal params",
		})
		s.sendError(req.ID, InvalidParams, "Invalid params", err.Error())
		return
	}

	var params ToolCallParams
	if err := json.Unmarshal(paramsData, &params); err != nil {
		s.logFeatureUsage("tools/call", "error", map[string]interface{}{
			"error": "failed to unmarshal params",
		})
		s.sendError(req.ID, InvalidParams, "Invalid params", err.Error())
		return
	}

	s.logFeatureUsage("tools/call", "called", map[string]interface{}{
		"tool": params.Name,
	})

	// Route to specific tool handler
	switch params.Name {
	case "kg.getSnapshot":
		s.handleGetSnapshotTool(req.ID, params.Arguments)
	case "kg.maskCode":
		s.handleMaskCodeTool(req.ID, params.Arguments)
	case "kg.validateFilledCode":
		s.handleValidateFilledCodeTool(req.ID, params.Arguments)
	default:
		s.logFeatureUsage("tools/call", "error", map[string]interface{}{
			"tool":  params.Name,
			"error": "tool not found",
		})
		s.sendError(req.ID, MethodNotFound, "Tool not found", params.Name)
	}
}

// handleMaskCodeTool handles kg.maskCode tool
// This tool segments code, masks each method, and returns the masked code directly to Copilot
func (s *Server) handleMaskCodeTool(id interface{}, args map[string]interface{}) {
	// Extract code
	code, ok := args["code"].(string)
	if !ok || code == "" {
		s.logFeatureUsage("kg.maskCode", "error", map[string]interface{}{
			"error": "missing or invalid 'code' parameter",
		})
		s.sendError(id, InvalidParams, "Missing or invalid 'code' parameter", nil)
		return
	}

	// Extract filePath
	filePath, ok := args["filePath"].(string)
	if !ok || filePath == "" {
		s.logFeatureUsage("kg.maskCode", "error", map[string]interface{}{
			"error": "missing or invalid 'filePath' parameter",
		})
		s.sendError(id, InvalidParams, "Missing or invalid 'filePath' parameter", nil)
		return
	}

	s.logFeatureUsage("kg.maskCode", "called", map[string]interface{}{
		"filePath":   filePath,
		"codeLength": len(code),
	})

	// Get the language plugin for this file
	plugin, err := s.langController.GetPlugin(filePath)
	if err != nil {
		s.logFeatureUsage("kg.maskCode", "error", map[string]interface{}{
			"error": fmt.Sprintf("unsupported language: %v", err),
		})
		content := []ContentItem{
			{
				Type: "text",
				Text: fmt.Sprintf("Unsupported language for file %s: %v", filePath, err),
			},
		}
		toolResult := ToolResult{Content: content, IsError: true}
		s.sendResponse(id, toolResult)
		return
	}

	// Get masker and segmenter from plugin
	masker := plugin.Masker()
	segmenter := plugin.Segmenter()

	s.logFeatureUsage("kg.maskCode", "info", map[string]interface{}{
		"language": plugin.Language(),
	})

	// 1. Segment the code into methods
	methods, err := segmenter.SegmentClass(code)
	if err != nil {
		s.logFeatureUsage("kg.maskCode", "error", map[string]interface{}{
			"error": fmt.Sprintf("segmentation failed: %v", err),
		})
		// Return error message to Copilot
		content := []ContentItem{
			{
				Type: "text",
				Text: fmt.Sprintf("Failed to segment code: %v\n\nPlease try with valid %s code.", err, plugin.Language()),
			},
		}
		toolResult := ToolResult{Content: content, IsError: true}
		s.sendResponse(id, toolResult)
		return
	}

	if len(methods) == 0 {
		s.logFeatureUsage("kg.maskCode", "error", map[string]interface{}{
			"error": "no methods found in code",
		})
		content := []ContentItem{
			{
				Type: "text",
				Text: "No callable methods/functions were found in the provided code.",
			},
		}
		toolResult := ToolResult{Content: content, IsError: true}
		s.sendResponse(id, toolResult)
		return
	}

	// 2. Determine which methods are new/modified by comparing against the existing file on disk.
	//    Only new or modified methods should be masked; existing unchanged methods are left as-is.
	existingMethodBodies := getExistingMethodBodies(filePath, s.workspaceRoot, segmenter)

	type maskedMethodInfo struct {
		Name      string `json:"name"`
		SessionID string `json:"sessionId"`
		StartLine int    `json:"startLine"`
		EndLine   int    `json:"endLine"`
	}

	var methodInfos []maskedMethodInfo
	maskedMethodBodies := make(map[int]string) // startLine -> maskedCode

	for _, method := range methods {
		// Skip methods that already exist on disk unchanged
		if existingBody, exists := existingMethodBodies[method.Name]; exists {
			if normalizeWhitespace(existingBody) == normalizeWhitespace(method.Body) {
				s.logFeatureUsage("kg.maskCode", "info", map[string]interface{}{
					"method":  method.Name,
					"skipped": "unchanged existing method",
				})
				continue
			}
		}

		// Mask the method body
		maskedResult, methodSessionID, err := masker.MaskCodeWithStore(method.Body, fmt.Sprintf("%s#%s", filePath, method.Name), s.store)
		if err != nil {
			s.logFeatureUsage("kg.maskCode", "warning", map[string]interface{}{
				"method": method.Name,
				"error":  fmt.Sprintf("masking failed: %v", err),
			})
			// Masking failed: keep original body, skip adding to methodInfos so no
			// orphan session ID is returned to Copilot (no [MASK] markers to fill anyway).
			maskedMethodBodies[method.StartLine] = method.Body
			continue
		}

		methodInfos = append(methodInfos, maskedMethodInfo{
			Name:      method.Name,
			SessionID: methodSessionID,
			StartLine: method.StartLine,
			EndLine:   method.EndLine,
		})

		maskedMethodBodies[method.StartLine] = maskedResult
	}

	// 3. If no methods were masked (all methods existed on disk unchanged), let Copilot know
	if len(methodInfos) == 0 {
		s.logFeatureUsage("kg.maskCode", "info", map[string]interface{}{
			"filePath": filePath,
			"skipped":  "all methods unchanged on disk",
		})
		content := []ContentItem{
			{
				Type: "text",
				Text: "All methods in the submitted code already exist on disk and are unchanged. No masking was applied.\n\nIf this is newly generated code, make sure the file has not already been written to disk before calling kg.maskCode.",
			},
		}
		toolResult := ToolResult{Content: content, IsError: false}
		s.sendResponse(id, toolResult)
		return
	}

	// 4. Reassemble full code with masked methods (only new/modified methods are replaced)
	maskedCode := reassembleCode(code, methods, maskedMethodBodies)

	s.logFeatureUsage("kg.maskCode", "success", map[string]interface{}{
		"methodsCount":   len(methodInfos),
		"totalMethods":   len(methods),
		"skippedMethods": len(methods) - len(methodInfos),
		"filePath":       filePath,
	})

	// 4. Resolve filePath to absolute so Copilot can write to it directly
	absFilePath := filePath
	if !filepath.IsAbs(filePath) {
		absFilePath = filepath.Join(s.workspaceRoot, filePath)
	}

	// 5. Return masked code directly to Copilot
	var methodList string
	for _, m := range methodInfos {
		methodList += fmt.Sprintf("- `%s` (session: %s, lines %d-%d)\n", m.Name, m.SessionID, m.StartLine, m.EndLine)
	}

	responseText := fmt.Sprintf(`## Masked Code Ready

**File (absolute path — use this when writing):** %s
**Methods masked:** %d
%s

### Instructions
1. Write the masked code below to the **absolute file path** above.
   **IMPORTANT:** If the file does not exist yet, use create_file (NOT apply_patch). Create parent directories first if needed.
2. Tell the user to fill in the [MASK] placeholders with their implementation
3. When the user says "validate", read the file and call kg.validateFilledCode with the session IDs above

### Masked Code

`+"```"+`
%s
`+"```"+`
`, absFilePath, len(methodInfos), methodList, maskedCode)

	content := []ContentItem{
		{
			Type: "text",
			Text: responseText,
		},
	}

	toolResult := ToolResult{
		Content: content,
		IsError: false,
	}

	s.sendResponse(id, toolResult)
}

// handleValidateFilledCodeTool handles kg.validateFilledCode tool
// Runs parse validation and returns the original code from the session so Copilot can do semantic comparison.
func (s *Server) handleValidateFilledCodeTool(id interface{}, args map[string]interface{}) {
	// Extract parameters
	sessionID, ok := args["sessionId"].(string)
	if !ok || sessionID == "" {
		s.logFeatureUsage("kg.validateFilledCode", "error", map[string]interface{}{
			"error": "missing or invalid 'sessionId' parameter",
		})
		s.sendError(id, InvalidParams, "Missing or invalid 'sessionId' parameter", nil)
		return
	}

	filledCode, ok := args["filledCode"].(string)
	if !ok || filledCode == "" {
		s.logFeatureUsage("kg.validateFilledCode", "error", map[string]interface{}{
			"error": "missing or invalid 'filledCode' parameter",
		})
		s.sendError(id, InvalidParams, "Missing or invalid 'filledCode' parameter", nil)
		return
	}

	filePath, ok := args["filePath"].(string)
	if !ok || filePath == "" {
		s.logFeatureUsage("kg.validateFilledCode", "error", map[string]interface{}{
			"error": "missing or invalid 'filePath' parameter",
		})
		s.sendError(id, InvalidParams, "Missing or invalid 'filePath' parameter", nil)
		return
	}

	s.logFeatureUsage("kg.validateFilledCode", "called", map[string]interface{}{
		"sessionId":  sessionID,
		"filePath":   filePath,
		"codeLength": len(filledCode),
	})

	// Get language plugin for validation
	plugin, err := s.langController.GetPlugin(filePath)
	if err != nil {
		s.logFeatureUsage("kg.validateFilledCode", "error", map[string]interface{}{
			"error": fmt.Sprintf("unsupported language: %v", err),
		})
		content := []ContentItem{
			{
				Type: "text",
				Text: fmt.Sprintf("Unsupported language for file %s: %v", filePath, err),
			},
		}
		toolResult := ToolResult{Content: content, IsError: true}
		s.sendResponse(id, toolResult)
		return
	}

	// Run parse-only validation
	validator := plugin.Validator()
	results, err := validator.ValidateFilledCode(filePath, filledCode)
	if err != nil {
		s.logFeatureUsage("kg.validateFilledCode", "error", map[string]interface{}{
			"error": fmt.Sprintf("validation failed: %v", err),
		})
		content := []ContentItem{
			{
				Type: "text",
				Text: fmt.Sprintf("Validation failed: %v", err),
			},
		}
		toolResult := ToolResult{Content: content, IsError: true}
		s.sendResponse(id, toolResult)
		return
	}

	// Check parse pass
	parsePassed := checkValidationPass(results)

	// Fetch original code from session for Copilot to compare
	session, sessionErr := s.store.GetMaskSession(sessionID)
	var originalCode string
	if sessionErr == nil && session != nil {
		originalCode = session.OriginalCode
	}

	// Atomically update filled code and status in one DB call
	status := db.StatusFilled
	if parsePassed {
		status = db.StatusValidated
	}
	if err := s.store.UpdateFilledCodeAndStatus(sessionID, filledCode, status); err != nil {
		s.logFeatureUsage("kg.validateFilledCode", "warning", map[string]interface{}{
			"error": fmt.Sprintf("failed to update session: %v", err),
		})
	}

	s.logFeatureUsage("kg.validateFilledCode", "success", map[string]interface{}{
		"sessionId":   sessionID,
		"parsePassed": parsePassed,
		"status":      status,
	})

	// Build response for Copilot
	var resultText string

	// Parse results
	for _, result := range results {
		icon := "PASS"
		if !result.Passed {
			icon = "FAIL"
		}
		resultText += fmt.Sprintf("### Stage: %s [%s]\n", result.Stage, icon)

		if result.Errors != nil {
			for _, e := range result.Errors {
				resultText += fmt.Sprintf("- Error: %s\n", e.Message)
			}
		}
		resultText += "\n"
	}

	if !parsePassed {
		resultText += "## Parse Failed\n\nThe code has syntax errors. Guide the user to fix the syntax issues above.\n\n"
	} else {
		resultText += "## Parse Passed\n\nThe code is syntactically valid.\n\n"

		// Include original code so Copilot can compare semantically
		if originalCode != "" {
			resultText += "## Original Code (for your comparison only)\n\n"
			resultText += "**You MUST compare** the user's filled code against the original below and give semantic feedback in this same response:\n"
			resultText += "- If correct: congratulate them\n"
			resultText += "- If different: give hints without revealing the original\n"
			resultText += "Do NOT stop at 'parse passed' and offer comparison as a follow-up. Do it now.\n\n"
			resultText += "```\n" + originalCode + "\n```\n\n"
		} else {
			resultText += "**Note:** Could not retrieve original code from session. Inform the user that parse validation passed.\n\n"
		}
	}

	content := []ContentItem{
		{
			Type: "text",
			Text: resultText,
		},
	}

	toolResult := ToolResult{
		Content: content,
		IsError: false,
	}

	s.sendResponse(id, toolResult)
}

// checkValidationPass checks if validation passed based on parse results.
func checkValidationPass(results []validation.ValidationResult) bool {
	for _, result := range results {
		if result.Stage == validation.StageParse && !result.Passed {
			return false
		}
	}
	// At least one parse result must exist
	for _, result := range results {
		if result.Stage == validation.StageParse {
			return true
		}
	}
	return false
}

// reassembleCode replaces method bodies with masked versions using line numbers
func reassembleCode(originalCode string, methods []segmentation.MethodSegment, maskedBodies map[int]string) string {
	lines := splitLines(originalCode)

	// Process methods from bottom to top to avoid index shifting
	for i := len(methods) - 1; i >= 0; i-- {
		method := methods[i]
		maskedCode, ok := maskedBodies[method.StartLine]
		if !ok {
			continue
		}

		startIdx := method.StartLine - 1 // Convert to 0-indexed
		endIdx := method.EndLine - 1
		if startIdx < 0 || endIdx >= len(lines) {
			continue
		}

		// Replace the lines with masked code
		maskedLines := splitLines(maskedCode)
		newLines := make([]string, 0, len(lines)-((endIdx-startIdx)+1)+len(maskedLines))
		newLines = append(newLines, lines[:startIdx]...)
		newLines = append(newLines, maskedLines...)
		newLines = append(newLines, lines[endIdx+1:]...)
		lines = newLines
	}

	return joinLines(lines)
}

// splitLines splits code into lines (preserves empty lines, drops trailing newline).
func splitLines(code string) []string {
	lines := strings.Split(code, "\n")
	// Drop a trailing empty element produced by a trailing newline
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// joinLines joins lines back into code.
func joinLines(lines []string) string {
	return strings.Join(lines, "\n")
}

// getExistingMethodBodies reads the file from disk, segments it, and returns
// a map of method name -> method body for methods already present on disk.
// Returns an empty map if the file doesn't exist or can't be parsed.
func getExistingMethodBodies(filePath, workspaceRoot string, segmenter segmentation.Segmenter) map[string]string {
	absPath := filePath
	if !filepath.IsAbs(filePath) {
		absPath = filepath.Join(workspaceRoot, filePath)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return map[string]string{}
	}

	existingMethods, err := segmenter.SegmentClass(string(data))
	if err != nil {
		return map[string]string{}
	}

	result := make(map[string]string, len(existingMethods))
	for _, m := range existingMethods {
		result[m.Name] = m.Body
	}
	return result
}

// normalizeWhitespace trims leading/trailing whitespace from each line and
// removes blank lines for comparison purposes. This ignores indentation and
// blank-line differences without collapsing whitespace inside string literals,
// which strings.Fields would incorrectly destroy (e.g. "a\nb" != "a b").
func normalizeWhitespace(s string) string {
	lines := strings.Split(s, "\n")
	var normalized []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}
	return strings.Join(normalized, "\n")
}

// handleGetSnapshotTool handles kg.getSnapshot tool
// This tool retrieves a snapshot and returns it as formatted markdown for Copilot
func (s *Server) handleGetSnapshotTool(id interface{}, args map[string]interface{}) {
	// Extract snapshotId
	snapshotID, ok := args["snapshotId"].(string)
	if !ok || snapshotID == "" {
		s.logFeatureUsage("kg.getSnapshot", "error", map[string]interface{}{
			"error": "missing or invalid 'snapshotId' parameter",
		})
		s.sendError(id, InvalidParams, "Missing or invalid 'snapshotId' parameter", nil)
		return
	}

	s.logFeatureUsage("kg.getSnapshot", "called", map[string]interface{}{
		"snapshotId": snapshotID,
	})

	// Retrieve snapshot from store
	snapshot, err := s.store.GetSnapshot(snapshotID)
	if err != nil {
		s.logFeatureUsage("kg.getSnapshot", "error", map[string]interface{}{
			"snapshotId": snapshotID,
			"error":      err.Error(),
		})

		var errorText string
		if strings.Contains(err.Error(), "not found") {
			errorText = fmt.Sprintf("Snapshot not found: %s\n\nPlease check the snapshot ID and try again.", snapshotID)
		} else {
			errorText = fmt.Sprintf("Failed to retrieve snapshot: %v", err)
		}

		content := []ContentItem{
			{
				Type: "text",
				Text: errorText,
			},
		}
		toolResult := ToolResult{Content: content, IsError: true}
		s.sendResponse(id, toolResult)
		return
	}

	// Use custom prompt if set, otherwise generate from snapshot
	var markdownContent string
	isCustomPrompt := false
	if snapshot.CustomPrompt != "" {
		markdownContent = snapshot.CustomPrompt
		isCustomPrompt = true
	} else {
		markdownContent = types.FormatSnapshotAsMarkdown(snapshot)
	}

	// Optional: optimize prompt via TextGrad when FYP_OPTIMIZE_PROMPTS=1
	textgradUsed := false
	if os.Getenv("FYP_OPTIMIZE_PROMPTS") == "1" {
		if optimized, err := runPromptOptimizer(s.workspaceRoot, markdownContent); err == nil {
			markdownContent = optimized
			textgradUsed = true
			s.logFeatureUsage("kg.getSnapshot", "optimized", map[string]interface{}{
				"textgrad": "used",
			})
		} else {
			s.logFeatureUsage("kg.getSnapshot", "optimize_fallback", map[string]interface{}{
				"textgrad": "failed",
				"error":    err.Error(),
			})
		}
	} else {
		s.logFeatureUsage("kg.getSnapshot", "textgrad_skipped", map[string]interface{}{
			"textgrad": "disabled",
			"reason":   "FYP_OPTIMIZE_PROMPTS is not 1",
		})
	}

	s.logFeatureUsage("kg.getSnapshot", "success", map[string]interface{}{
		"snapshotId":       snapshotID,
		"virtualNodeCount": len(snapshot.VirtualNodes),
		"contextNodeCount": len(snapshot.ContextNodes),
		"edgeCount":        len(snapshot.Edges),
		"isCustomPrompt":   isCustomPrompt,
		"textgradUsed":     textgradUsed,
	})

	content := []ContentItem{
		{
			Type: "text",
			Text: markdownContent,
		},
	}

	toolResult := ToolResult{
		Content: content,
		IsError: false,
	}

	s.sendResponse(id, toolResult)
}
