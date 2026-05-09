/**
 * Settings Panel - Webview panel for extension settings and index management
 */

import * as vscode from 'vscode';
import * as path from 'path';
import * as fs from 'fs';
import { isCopilotMcpConfigured } from '../../infrastructure/services/mcpServer';

export interface SettingsPanelDependencies {
  statusBarItem: vscode.StatusBarItem;
  statusProvider: { getStatus(): string };
  // Legacy field kept for compatibility; MCP is now spawned by Copilot, not the extension.
  mcpServerGetter: () => unknown;
}

/**
 * Open the settings panel webview
 */
export function openSettingsPanel(
  context: vscode.ExtensionContext,
  deps: SettingsPanelDependencies
): void {
  const panel = vscode.window.createWebviewPanel(
    'fypSettings',
    'FYP Settings',
    vscode.ViewColumn.One,
    {
      enableScripts: true,
      retainContextWhenHidden: true
    }
  );

  panel.webview.html = getSettingsHtml(panel.webview, context.extensionUri);

  // Handle messages from the webview
  panel.webview.onDidReceiveMessage(
    async (message) => {
      switch (message.command) {
        case 'buildIndex':
          await vscode.commands.executeCommand('fyp.openIndexPanel');
          break;
        case 'rebuildIndex':
          await vscode.commands.executeCommand('fyp.openIndexPanel');
          break;
        case 'openGraph':
          await vscode.commands.executeCommand('fyp.openGraph');
          break;
        case 'checkStatus':
          // Check database status
          const workspaceRoot = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
          const dbPath = workspaceRoot ? path.join(workspaceRoot, '.fyp', 'index.db') : null;
          const dbExists = dbPath ? fs.existsSync(dbPath) : false;
          const dbSize = dbExists && dbPath ? fs.statSync(dbPath).size : 0;
          
          const mcpConfigured = isCopilotMcpConfigured();
          panel.webview.postMessage({
            command: 'status',
            data: {
              mcpRunning: mcpConfigured,
              indexStatus: deps.statusProvider.getStatus(),
              dbExists: dbExists,
              dbPath: dbPath || 'No workspace open',
              dbSize: dbSize
            }
          });
          break;
      }
    },
    undefined,
    context.subscriptions
  );
}

/**
 * Generate settings HTML content
 */
function getSettingsHtml(webview: vscode.Webview, extensionUri: vscode.Uri): string {
  return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>FYP Settings</title>
    <style>
        body {
            padding: 20px;
            font-family: var(--vscode-font-family);
            color: var(--vscode-foreground);
        }
        .section {
            margin-bottom: 30px;
            padding: 20px;
            border: 1px solid var(--vscode-panel-border);
            border-radius: 4px;
        }
        h1 {
            margin-top: 0;
            color: var(--vscode-foreground);
        }
        h2 {
            color: var(--vscode-descriptionForeground);
            font-size: 1.2em;
        }
        button {
            background: var(--vscode-button-background);
            color: var(--vscode-button-foreground);
            border: none;
            padding: 10px 20px;
            margin-right: 10px;
            cursor: pointer;
            border-radius: 2px;
        }
        button:hover {
            background: var(--vscode-button-hoverBackground);
        }
        .status {
            display: inline-block;
            padding: 5px 10px;
            border-radius: 3px;
            margin-left: 10px;
        }
        .status.ready {
            background: var(--vscode-testing-iconPassed);
            color: white;
        }
        .status.indexing {
            background: var(--vscode-editorWarning-foreground);
            color: white;
        }
        .status.error {
            background: var(--vscode-editorError-foreground);
            color: white;
        }
        .info {
            margin-top: 10px;
            padding: 10px;
            background: var(--vscode-textBlockQuote-background);
            border-left: 3px solid var(--vscode-textBlockQuote-border);
        }
    </style>
</head>
<body>
    <h1>🎓 FYP - Code Learning Assistant</h1>
    
    <div class="section">
        <h2>Index Management</h2>
        <p>Build and manage your local code knowledge base</p>
        <div>
            <button id="buildBtn" onclick="buildIndex()">🔨 Build Index</button>
            <button id="rebuildBtn" onclick="rebuildIndex()">🔄 Rebuild Index</button>
            <span id="indexStatus" class="status">Checking...</span>
        </div>
        <div class="info">
            <strong>What does indexing do?</strong><br>
            • Parses all Java files in your workspace<br>
            • Extracts classes, methods, and fields<br>
            • Builds call graph relationships<br>
            • Stores everything in .fyp/index.db<br>
            • Takes ~5-10 seconds for typical projects
        </div>
    </div>

    <div class="section">
        <h2>📊 Graph Editor</h2>
        <p>Visualize and annotate your codebase as an interactive graph</p>
        <div>
            <button onclick="openGraph()" style="background: linear-gradient(135deg, #3b82f6, #8b5cf6);">
                📊 Open Graph Editor
            </button>
        </div>
        <div class="info">
            <strong>What can you do in the Graph Editor?</strong><br>
            • Visualize code structure as an interactive graph<br>
            • Create virtual nodes for planned features<br>
            • Add descriptions and AI remarks for context<br>
            • Generate code for virtual nodes using AI<br>
            • Practice coding with masking exercises
        </div>
    </div>

    <div class="section">
        <h2>Database Info</h2>
        <div id="dbInfo">
            <p>Location: <code id="dbPath">Checking...</code></p>
            <p>Status: <span id="dbStatus" class="status">Checking...</span></p>
            <p id="dbSizeInfo" style="display:none;">Size: <span id="dbSize">-</span></p>
        </div>
    </div>

    <div class="section">
        <h2>GitHub Copilot Integration</h2>
        <p>MCP Server Status: <span id="mcpServerStatus" class="status">Unknown</span></p>
        <div class="info">
            The MCP server allows GitHub Copilot to use the masking workflow.<br>
            When active, Copilot can use the kg.maskCode tool for code learning exercises.
        </div>
    </div>

    <script>
        const vscode = acquireVsCodeApi();

        function buildIndex() {
            vscode.postMessage({ command: 'buildIndex' });
        }

        function rebuildIndex() {
            vscode.postMessage({ command: 'rebuildIndex' });
        }

        function openGraph() {
            vscode.postMessage({ command: 'openGraph' });
        }

        function checkStatus() {
            vscode.postMessage({ command: 'checkStatus' });
        }

        // Listen for status updates
        window.addEventListener('message', event => {
            const message = event.data;
            if (message.command === 'status') {
                const { mcpRunning, indexStatus, isIndexing, dbExists, dbPath, dbSize } = message.data;
                
                document.getElementById('mcpServerStatus').textContent = mcpRunning ? 'Running' : 'Stopped';
                document.getElementById('mcpServerStatus').className = 'status ' + (mcpRunning ? 'ready' : 'error');
                
                document.getElementById('indexStatus').textContent = indexStatus || 'Unknown';
                document.getElementById('indexStatus').className = 'status ' + (indexStatus || 'ready');
                
                // Update database info
                document.getElementById('dbPath').textContent = dbPath || 'No workspace open';
                const dbStatusEl = document.getElementById('dbStatus');
                const dbSizeInfo = document.getElementById('dbSizeInfo');
                const dbSizeEl = document.getElementById('dbSize');
                
                if (dbExists) {
                    dbStatusEl.textContent = '✓ Found';
                    dbStatusEl.className = 'status ready';
                    
                    // Show size info
                    dbSizeInfo.style.display = 'block';
                    const sizeMB = (dbSize / (1024 * 1024)).toFixed(2);
                    const sizeKB = (dbSize / 1024).toFixed(2);
                    dbSizeEl.textContent = dbSize > 1024 * 1024 ? sizeMB + ' MB' : sizeKB + ' KB';
                } else {
                    dbStatusEl.textContent = '✗ Not Found';
                    dbStatusEl.className = 'status error';
                    dbSizeInfo.style.display = 'none';
                }
                
                // Enable/disable buttons based on indexing state
                const buildBtn = document.getElementById('buildBtn');
                const rebuildBtn = document.getElementById('rebuildBtn');
                if (isIndexing) {
                    buildBtn.disabled = true;
                    rebuildBtn.disabled = true;
                    buildBtn.style.opacity = '0.5';
                    rebuildBtn.style.opacity = '0.5';
                } else {
                    buildBtn.disabled = false;
                    rebuildBtn.disabled = false;
                    buildBtn.style.opacity = '1';
                    rebuildBtn.style.opacity = '1';
                }
            } else if (message.command === 'indexing') {
                const { isIndexing } = message.data;
                const buildBtn = document.getElementById('buildBtn');
                const rebuildBtn = document.getElementById('rebuildBtn');
                const statusEl = document.getElementById('indexStatus');
                
                if (isIndexing) {
                    buildBtn.disabled = true;
                    rebuildBtn.disabled = true;
                    buildBtn.style.opacity = '0.5';
                    rebuildBtn.style.opacity = '0.5';
                    statusEl.textContent = 'Indexing...';
                    statusEl.className = 'status indexing';
                } else {
                    buildBtn.disabled = false;
                    rebuildBtn.disabled = false;
                    buildBtn.style.opacity = '1';
                    rebuildBtn.style.opacity = '1';
                    // Status will be updated by the next checkStatus call
                }
            }
        });

        // Check status on load
        checkStatus();
        setInterval(checkStatus, 5000);
    </script>
</body>
</html>`;
}
