/**
 * FYP Extension - Entry Point
 * A code learning assistant that integrates with GitHub Copilot via MCP tools.
 * 
 * Architecture:
 * - Uses Copilot's native agent mode for code generation
 * - Exposes kg.maskCode MCP tool for masking workflow (returns masked code to Copilot)
 * - Provides indexing for graph-viz (call graph only, no embeddings)
 */

import * as vscode from 'vscode';
import * as fs from 'fs';
import * as path from 'path';
import { startMCPServer, stopMCPServer, getMCPServer, isCopilotMcpConfigured, configureCopilotMcpForWorkspace, autoConfigureMcpIfNeeded, ensureMcpJsonSync } from './infrastructure/services/mcpServer';
import { stopApiStdioServer, startApiStdioServer, isApiStdioServerRunning } from './infrastructure/services/apiStdioServer';
import { IndexStatusProvider } from './ui/providers/statusProvider';
import { openGraphPanel } from './ui/panels/graphPanel';
import { openIndexPanel } from './ui/panels/indexPanel';
import { registerMaskCodeLens } from './features/masking/maskCodeLensProvider';
import { registerOnSaveIndexing } from './features/indexing/onSaveIndexer';

let statusBarItem: vscode.StatusBarItem;
let indexStatusProvider: IndexStatusProvider;

/**
 * Extension activation - registers commands, starts API stdio server, configures MCP.
 */
export function activate(context: vscode.ExtensionContext): void {
  console.log('FYP extension activating...');

  const workspaceRoot = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;

  // Create .vscode/mcp.json sync if missing (avoids ENOENT when Copilot reads it at startup)
  ensureMcpJsonSync(context);

  // Initialize status bar
  statusBarItem = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Right, 100);
  statusBarItem.command = 'fyp.showCommands';
  statusBarItem.text = '$(database) FYP: Ready';
  statusBarItem.show();
  context.subscriptions.push(statusBarItem);

  // Initialize index status tree view
  indexStatusProvider = new IndexStatusProvider(getMCPServer);
  vscode.window.registerTreeDataProvider('fypIndexStatus', indexStatusProvider);

  const updateSidebarStatusFromWorkspace = () => {
    if (!workspaceRoot) {
      indexStatusProvider.setStatus('unknown');
      indexStatusProvider.refresh();
      return;
    }

    // Otherwise infer readiness from the DB presence.
    const dbPath = path.join(workspaceRoot, '.fyp', 'index.db');
    const dbExists = fs.existsSync(dbPath);
    indexStatusProvider.setStatus(dbExists ? 'ready' : 'unknown');
    indexStatusProvider.refresh();
  };

  // Initialize status once on activation
  updateSidebarStatusFromWorkspace();

  // Refresh on config changes (important for Copilot MCP config)
  context.subscriptions.push(
    vscode.workspace.onDidChangeConfiguration((e) => {
      if (e.affectsConfiguration('github.copilot.mcp') || e.affectsConfiguration('fyp')) {
        updateSidebarStatusFromWorkspace();
      }
    })
  );

  // Watch index DB + MCP log changes so the sidebar updates
  if (workspaceRoot) {
    const indexDbWatcher = vscode.workspace.createFileSystemWatcher(
      new vscode.RelativePattern(workspaceRoot, '.fyp/index.db')
    );
    const mcpLogWatcher = vscode.workspace.createFileSystemWatcher(
      new vscode.RelativePattern(workspaceRoot, '.fyp/mcp-server.log')
    );
    const workspaceSettingsWatcher = vscode.workspace.createFileSystemWatcher(
      new vscode.RelativePattern(workspaceRoot, '.vscode/settings.json')
    );

    // Wrap in try-catch to suppress harmless SQLite journal file errors
    // SQLite creates/deletes journal files during transactions, which can trigger
    // race conditions with file watchers
    const onFsEvent = (uri: vscode.Uri) => {
      try {
        // Ignore journal files (temporary SQLite files)
        if (uri.fsPath.includes('-journal') || uri.fsPath.includes('-wal')) {
          return;
        }
        updateSidebarStatusFromWorkspace();
      } catch (err) {
        // Ignore file system errors for temporary files
        const errMsg = err instanceof Error ? err.message : String(err);
        if (errMsg.includes('journal') || errMsg.includes('ENOENT')) {
          return;
        }
        console.error('[FYP] File watcher error:', err);
      }
    };
    
    indexDbWatcher.onDidCreate(onFsEvent, null, context.subscriptions);
    indexDbWatcher.onDidChange(onFsEvent, null, context.subscriptions);
    indexDbWatcher.onDidDelete(onFsEvent, null, context.subscriptions);
    mcpLogWatcher.onDidCreate(onFsEvent, null, context.subscriptions);
    mcpLogWatcher.onDidChange(onFsEvent, null, context.subscriptions);
    workspaceSettingsWatcher.onDidCreate(onFsEvent, null, context.subscriptions);
    workspaceSettingsWatcher.onDidChange(onFsEvent, null, context.subscriptions);

    context.subscriptions.push(indexDbWatcher, mcpLogWatcher, workspaceSettingsWatcher);
  }

  // Auto-configure MCP and check status
  const config = vscode.workspace.getConfiguration('fyp');
  if (config.get('autoStartMCP', true)) {
    // Auto-create .vscode/mcp.json and .github/copilot-instructions.md if missing
    autoConfigureMcpIfNeeded(context).then(() => {
      statusBarItem.text = isCopilotMcpConfigured()
        ? '$(check) FYP: MCP Configured'
        : '$(warning) FYP: MCP Not Configured';
    });
  }

  // Start API stdio server on activation (required for indexing, validation, and graph)
  if (workspaceRoot) {
    startApiStdioServer(context).then((started) => {
      if (started) {
        console.log('[FYP] API stdio server started on activation');
      } else {
        console.warn('[FYP] Failed to start API stdio server on activation');
      }
    }).catch((err) => {
      console.error('[FYP] Error starting API stdio server:', err);
    });
  }

  // Register mask CodeLens provider (detects [MASK] markers in files)
  registerMaskCodeLens(context);

  // Register on-save incremental indexing
  registerOnSaveIndexing(context, async () => {
    if (isApiStdioServerRunning()) {
      return true;
    }
    return startApiStdioServer(context);
  });

  // Register commands
  registerCoreCommands(context, updateSidebarStatusFromWorkspace);

  // Register graph editor command
  context.subscriptions.push(
    vscode.commands.registerCommand('fyp.openGraph', async () => {
      await openGraphPanel(context, {});
    })
  );

  console.log('FYP extension activated successfully');
}

/**
 * Register core extension commands
 */
function registerCoreCommands(
  context: vscode.ExtensionContext,
  updateStatus: () => void
): void {
  // Show commands menu
  context.subscriptions.push(
    vscode.commands.registerCommand('fyp.showCommands', async () => {
      const items = [
        { label: '$(database) Open Index Panel', command: 'fyp.openIndexPanel' },
        { label: '$(gear) Configure MCP', command: 'fyp.configureMCP' },
        { label: '$(graph) Open Graph Viewer', command: 'fyp.openGraph' }
      ];
      
      const selected = await vscode.window.showQuickPick(items, {
        placeHolder: 'Select a command'
      });
      
      if (selected) {
        vscode.commands.executeCommand(selected.command);
      }
    })
  );

  // Open index panel
  context.subscriptions.push(
    vscode.commands.registerCommand('fyp.openIndexPanel', async () => {
      await openIndexPanel(context);
      updateStatus();
    })
  );

  // Configure MCP
  context.subscriptions.push(
    vscode.commands.registerCommand('fyp.configureMCP', async () => {
      await configureCopilotMcpForWorkspace(context);
      updateStatus();
    })
  );

}

/**
 * Extension deactivation
 */
export function deactivate(): void {
  stopApiStdioServer();
  stopMCPServer();
  console.log('FYP extension deactivated');
}
