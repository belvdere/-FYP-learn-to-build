/**
 * Status Provider - Tree view provider for index and MCP server status
 */

import * as vscode from 'vscode';
import { ChildProcess } from 'child_process';
import * as fs from 'fs';
import * as path from 'path';
import { isCopilotMcpConfigured } from '../../infrastructure/services/mcpServer';

/**
 * Tree view provider for showing FYP status
 */
export class IndexStatusProvider implements vscode.TreeDataProvider<vscode.TreeItem> {
  private _onDidChangeTreeData = new vscode.EventEmitter<vscode.TreeItem | undefined | null | void>();
  readonly onDidChangeTreeData = this._onDidChangeTreeData.event;
  private status: string = 'unknown';
  private mcpServerGetter: () => ChildProcess | undefined;

  constructor(mcpServerGetter: () => ChildProcess | undefined) {
    this.mcpServerGetter = mcpServerGetter;
  }

  setStatus(status: string): void {
    this.status = status;
    this._onDidChangeTreeData.fire();
  }

  getStatus(): string {
    return this.status;
  }

  refresh(): void {
    this._onDidChangeTreeData.fire();
  }

  getTreeItem(element: vscode.TreeItem): vscode.TreeItem {
    return element;
  }

  getChildren(): vscode.TreeItem[] {
    const mcpConfigured = isCopilotMcpConfigured();

    // Best-effort: infer recent MCP activity from the log file (Copilot owns the process).
    const workspaceRoot = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
    let mcpDescription = mcpConfigured ? 'configured' : 'not configured';
    if (mcpConfigured && workspaceRoot) {
      try {
        const logPath = path.join(workspaceRoot, '.fyp', 'mcp-server.log');
        if (fs.existsSync(logPath)) {
          const st = fs.statSync(logPath);
          const ageMs = Date.now() - st.mtimeMs;
          // If log was written recently, MCP is likely active.
          if (ageMs < 2 * 60 * 1000) {
            mcpDescription = 'active';
          } else {
            mcpDescription = 'configured (no recent activity)';
          }
        } else {
          mcpDescription = 'configured (no log yet)';
        }
      } catch {
        // Ignore filesystem errors; keep configured/not configured.
      }
    }
    return [
      new StatusItem('Index Status', this.status, vscode.TreeItemCollapsibleState.None),
      new StatusItem('MCP (Copilot)', mcpDescription, vscode.TreeItemCollapsibleState.None),
      new ActionItem('Open Index Panel', 'fyp.openIndexPanel', 'database'),
      new ActionItem('Open Graph Editor', 'fyp.openGraph', 'graph')
    ];
  }
}

/**
 * Tree item for status display
 */
class StatusItem extends vscode.TreeItem {
  constructor(
    public readonly label: string,
    private status: string,
    public readonly collapsibleState: vscode.TreeItemCollapsibleState
  ) {
    super(label, collapsibleState);
    this.description = status;
    this.iconPath = new vscode.ThemeIcon(
      status === 'running' || status === 'ready' ? 'pass' : 
      status === 'indexing' ? 'sync~spin' : 'circle-outline'
    );
  }
}

/**
 * Tree item for clickable actions
 */
class ActionItem extends vscode.TreeItem {
  constructor(
    public readonly label: string,
    commandId: string,
    iconName: string
  ) {
    super(label, vscode.TreeItemCollapsibleState.None);
    this.iconPath = new vscode.ThemeIcon(iconName);
    this.command = {
      command: commandId,
      title: label
    };
  }
}
