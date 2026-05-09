/**
 * MCP Integration Helpers
 *
 * IMPORTANT:
 * MCP is stdio-based. GitHub Copilot must be the parent process that spawns `fypd serve-mcp`
 * so it can communicate over stdin/stdout. The extension should configure Copilot to spawn it,
 * rather than spawning it itself.
 *
 * Configuration is done via .vscode/mcp.json (workspace level) which is the standard
 * for VS Code 1.99+ with GitHub Copilot MCP support.
 */

import * as vscode from 'vscode';
import { getFypdBinaryPath } from '../../utils/processHelper';
import * as fs from 'fs';
import * as path from 'path';

type McpServerConfig = {
  type?: string;
  command?: string;
  args?: string[];
  env?: Record<string, string>;
  url?: string;
};

type McpConfig = {
  servers?: Record<string, McpServerConfig>;
};

function getMcpJsonPath(workspaceRoot: string): string {
  return path.join(workspaceRoot, '.vscode', 'mcp.json');
}

function getBuildWithMeMcpPath(workspaceRoot: string): string {
  return path.join(workspaceRoot, '.fyp', 'build-with-me-mcp.md');
}

function getBundledMcpReferencePath(extensionPath: string): string {
  return path.join(extensionPath, 'templates', 'build-with-me-mcp.md');
}

function extractMcpReferenceVersion(content: string): number {
  const match = content.match(/<!--\s*BWM_VERSION:\s*(\d+)\s*-->/);
  return match ? parseInt(match[1], 10) : 0;
}

/**
 * Read existing mcp.json if it exists
 */
function readMcpJson(workspaceRoot: string): McpConfig | null {
  const mcpPath = getMcpJsonPath(workspaceRoot);
  if (!fs.existsSync(mcpPath)) {
    return {};
  }
  try {
    const raw = fs.readFileSync(mcpPath, 'utf8');
    return JSON.parse(raw);
  } catch {
    return null;
  }
}

/**
 * Write MCP configuration to .vscode/mcp.json
 * This is the standard location for VS Code 1.99+ with GitHub Copilot MCP support
 */
function writeMcpJson(workspaceRoot: string, fypdPath: string): void {
  const dir = path.join(workspaceRoot, '.vscode');
  if (!fs.existsSync(dir)) {
    fs.mkdirSync(dir, { recursive: true });
  }

  const mcpPath = getMcpJsonPath(workspaceRoot);
  const existing = readMcpJson(workspaceRoot) || {};

  const config: McpConfig = {
    ...existing,
    servers: {
      ...(existing.servers || {}),
      fyp: {
        type: 'stdio',
        command: fypdPath,
        args: ['serve-mcp', '--workspace', workspaceRoot],
      }
    }
  };

  fs.writeFileSync(mcpPath, JSON.stringify(config, null, 2) + '\n', 'utf8');
}

/**
 * Write or update .fyp/build-with-me-mcp.md — the canonical MCP tools reference.
 * Users can point any AI agent at this file (Claude Code, Codex, etc.).
 * Uses BWM_VERSION to skip writes when the file is already up to date.
 */
function writeBuildWithMeMcpReference(workspaceRoot: string, extensionPath: string): void {
  const fyp = path.join(workspaceRoot, '.fyp');
  if (!fs.existsSync(fyp)) {
    fs.mkdirSync(fyp, { recursive: true });
  }

  const destPath = getBuildWithMeMcpPath(workspaceRoot);
  const srcPath = getBundledMcpReferencePath(extensionPath);

  if (!fs.existsSync(srcPath)) {
    console.warn('[FYP] Bundled build-with-me-mcp.md not found at:', srcPath);
    return;
  }

  const bundledContent = fs.readFileSync(srcPath, 'utf8');
  const bundledVersion = extractMcpReferenceVersion(bundledContent);

  if (fs.existsSync(destPath)) {
    const existingVersion = extractMcpReferenceVersion(fs.readFileSync(destPath, 'utf8'));
    if (bundledVersion <= existingVersion) {
      console.log(`[FYP] build-with-me-mcp.md is up to date (version ${existingVersion})`);
      return;
    }
    console.log(`[FYP] Updating build-with-me-mcp.md from version ${existingVersion} to ${bundledVersion}`);
  } else {
    console.log(`[FYP] Creating .fyp/build-with-me-mcp.md (version ${bundledVersion})`);
  }

  fs.writeFileSync(destPath, bundledContent, 'utf8');
}

/**
 * Returns true if workspace has MCP configured for FYP via .vscode/mcp.json
 */
export function isCopilotMcpConfigured(): boolean {
  const workspaceRoot = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
  if (!workspaceRoot) {
    return false;
  }

  const config = readMcpJson(workspaceRoot);
  if (!config) {
    return false;
  }

  const fyp = config?.servers?.fyp;
  return Boolean(fyp?.command && Array.isArray(fyp?.args));
}

/**
 * Configure GitHub Copilot MCP settings in the current workspace.
 * Creates .vscode/mcp.json and .fyp/build-with-me-mcp.md
 */
export async function configureCopilotMcpForWorkspace(context: vscode.ExtensionContext): Promise<void> {
  const workspaceRoot = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
  if (!workspaceRoot) {
    vscode.window.showWarningMessage('FYP: No workspace folder open');
    return;
  }

  // IMPORTANT: Use the extension-bundled fypd binary, not a workspace-local build.
  const fypdPath = getFypdBinaryPath(context.extensionUri.fsPath);
  if (!fypdPath) {
    vscode.window.showWarningMessage(
      `FYP: fypd binary not found in the extension install. Rebuild/reinstall the extension package.`
    );
    return;
  }

  try {
    // 1. Write .vscode/mcp.json (MCP server configuration)
    writeMcpJson(workspaceRoot, fypdPath);

    // 2. Write/update .fyp/build-with-me-mcp.md (agent-agnostic MCP reference)
    writeBuildWithMeMcpReference(workspaceRoot, context.extensionUri.fsPath);

    vscode.window.showInformationMessage(
      'FYP: Configured MCP server. Reload VS Code if needed.',
      'Reload Window'
    ).then(selection => {
      if (selection === 'Reload Window') {
        vscode.commands.executeCommand('workbench.action.reloadWindow');
      }
    });
  } catch (err) {
    vscode.window.showErrorMessage(
      `FYP: Failed to configure MCP: ${err instanceof Error ? err.message : String(err)}`
    );
  }
}

/**
 * Synchronously create .vscode/mcp.json if missing. Call early in activation
 * to avoid ENOENT when VS Code/Copilot tries to read it at startup.
 */
export function ensureMcpJsonSync(context: vscode.ExtensionContext): void {
  const workspaceRoot = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
  if (!workspaceRoot) {
    return;
  }
  const fypdPath = getFypdBinaryPath(context.extensionUri.fsPath);
  if (!fypdPath) {
    return;
  }
  if (!isCopilotMcpConfigured()) {
    try {
      writeMcpJson(workspaceRoot, fypdPath);
    } catch (_) {
      // Ignore; will retry in async autoConfigureMcpIfNeeded
    }
  }
}

/**
 * Auto-configure MCP on extension activation if not already configured
 */
export async function autoConfigureMcpIfNeeded(context: vscode.ExtensionContext): Promise<void> {
  const workspaceRoot = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
  if (!workspaceRoot) {
    return;
  }

  const fypdPath = getFypdBinaryPath(context.extensionUri.fsPath);
  if (!fypdPath) {
    return; // Binary not available, skip auto-config
  }

  // Auto-create mcp.json if missing
  if (!isCopilotMcpConfigured()) {
    try {
      writeMcpJson(workspaceRoot, fypdPath);
      console.log('[FYP] Auto-configured .vscode/mcp.json');
    } catch (err) {
      console.error('[FYP] Failed to auto-configure mcp.json:', err);
    }
  }

  // Auto-create or update .fyp/build-with-me-mcp.md
  try {
    writeBuildWithMeMcpReference(workspaceRoot, context.extensionUri.fsPath);
  } catch (err) {
    console.error('[FYP] Failed to write build-with-me-mcp.md:', err);
  }
}

/**
 * Backwards-compatible no-op: we no longer spawn MCP from the extension.
 */
export function startMCPServer(): void {
  // no-op
}

/**
 * Backwards-compatible no-op.
 */
export function stopMCPServer(): void {
  // no-op
}

/**
 * Backwards-compatible: always undefined (Copilot owns the process).
 */
export function getMCPServer(): undefined {
  return undefined;
}
