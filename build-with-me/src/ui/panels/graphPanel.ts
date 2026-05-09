/**
 * Graph Panel - Webview for graph visualization.
 * Loads bundled graph-viz React app; bridges postMessage (webview) <-> JSON-RPC (stdio).
 * Extension is the only path between webview and fypd backend.
 */

import * as vscode from 'vscode';
import * as path from 'path';
import * as fs from 'fs';
import { 
  startApiStdioServer, 
  stopApiStdioServer, 
  sendApiRequest, 
  isApiStdioServerRunning 
} from '../../infrastructure/services/apiStdioServer';

let currentPanel: vscode.WebviewPanel | undefined;

export interface GraphPanelConfig {
  // backendUrl is no longer needed - we use stdio communication
}

/**
 * Open or reveal the graph panel webview
 */
export async function openGraphPanel(
  context: vscode.ExtensionContext,
  config: GraphPanelConfig = {}
): Promise<void> {
  const column = vscode.ViewColumn.One;

  // If we already have a panel, reveal it
  if (currentPanel) {
    currentPanel.reveal(column);
    return;
  }

  // Start the API stdio server if not running
  if (!isApiStdioServerRunning()) {
    const started = await startApiStdioServer(context);
    if (!started) {
      vscode.window.showErrorMessage('Failed to start API server. Please check the output for details.');
      return;
    }
  }

  // Path to bundled graph-viz assets
  const graphVizPath = vscode.Uri.joinPath(context.extensionUri, 'media', 'graph-viz');

  // Create a new panel
  currentPanel = vscode.window.createWebviewPanel(
    'fypGraph',
    'FYP Graph Editor',
    column,
    {
      enableScripts: true,
      retainContextWhenHidden: true,
      localResourceRoots: [graphVizPath]
    }
  );

  // Set the HTML content
  currentPanel.webview.html = getGraphHtml(currentPanel.webview, context);

  // Handle messages from the webview
  currentPanel.webview.onDidReceiveMessage(
    async (message) => {
      switch (message.type) {
        case 'apiRequest':
          // Route API requests to the stdio server
          await handleApiRequest(message, currentPanel!);
          break;
        case 'showInfo':
          vscode.window.showInformationMessage(message.text);
          break;
        case 'showError':
          vscode.window.showErrorMessage(message.text);
          break;
        case 'ready':
          // Webview is ready, send configuration
          currentPanel?.webview.postMessage({
            type: 'config',
            useStdioApi: true  // Tell graph-viz to use message passing
          });
          break;
      }
    },
    undefined,
    context.subscriptions
  );

  // Handle panel disposal
  currentPanel.onDidDispose(
    () => {
      currentPanel = undefined;
      // Note: We don't stop the API server here - it can be reused if panel is reopened
    },
    undefined,
    context.subscriptions
  );
}

// Request graph-viz webview to refresh from backend when panel is visible.
export function requestGraphRefreshIfVisible(): void {
  if (!currentPanel || !currentPanel.visible) {
    return;
  }
  currentPanel.webview.postMessage({ type: 'graphRefreshRequested' });
}

/**
 * Bridge: receive apiRequest from webview, forward to fypd via sendApiRequest, post response back.
 * Guards postMessage calls so a panel closed while a request is in-flight doesn't throw or leak.
 */
async function handleApiRequest(
  message: { id: number; method: string; params: Record<string, unknown> },
  panel: vscode.WebviewPanel
): Promise<void> {
  let result: unknown;
  let errorMessage: string | undefined;

  try {
    result = await sendApiRequest(message.method, message.params ?? {});
  } catch (error) {
    errorMessage = error instanceof Error ? error.message : String(error);
  }

  // The panel may have been disposed while the request was in-flight — guard against that.
  try {
    if (errorMessage !== undefined) {
      panel.webview.postMessage({
        type: 'apiResponse',
        id: message.id,
        error: { message: errorMessage }
      });
    } else {
      panel.webview.postMessage({
        type: 'apiResponse',
        id: message.id,
        result
      });
    }
  } catch {
    // Panel was disposed before we could post the response; nothing to do.
  }
}

/**
 * Generate the HTML content for the graph webview
 * Loads the bundled React app directly
 */
function getGraphHtml(
  webview: vscode.Webview,
  context: vscode.ExtensionContext
): string {
  // Get URIs for bundled assets
  const graphVizPath = vscode.Uri.joinPath(context.extensionUri, 'media', 'graph-viz');
  const scriptUri = webview.asWebviewUri(vscode.Uri.joinPath(graphVizPath, 'assets', 'index.js'));
  const styleUri = webview.asWebviewUri(vscode.Uri.joinPath(graphVizPath, 'assets', 'index.css'));
  
  // Check if bundled assets exist
  const assetsExist = fs.existsSync(path.join(context.extensionPath, 'media', 'graph-viz', 'assets', 'index.js'));
  
  if (!assetsExist) {
    return getErrorHtml();
  }

  // Use a nonce for script security
  const nonce = getNonce();

  return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <meta http-equiv="Content-Security-Policy" content="
      default-src 'none';
      style-src ${webview.cspSource} 'unsafe-inline';
      script-src ${webview.cspSource} 'nonce-${nonce}';
      font-src ${webview.cspSource};
      img-src ${webview.cspSource} data: https:;
      connect-src ${webview.cspSource};
    ">
    <title>FYP Graph Editor</title>
    <link rel="stylesheet" href="${styleUri}">
    <style>
      /* VS Code theme integration */
      :root {
        --vscode-font: var(--vscode-font-family);
      }
      body {
        margin: 0;
        padding: 0;
        overflow: hidden;
      }
      #root {
        width: 100vw;
        height: 100vh;
      }
    </style>
</head>
<body>
    <div id="root"></div>
    
    <!-- Signal to use stdio API via message passing -->
    <script nonce="${nonce}">
      window.__USE_STDIO_API__ = true;
    </script>
    
    <script nonce="${nonce}" type="module" src="${scriptUri}"></script>
</body>
</html>`;
}

/**
 * Generate error HTML when bundled assets are not found
 */
function getErrorHtml(): string {
  return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Graph Editor - Setup Required</title>
    <style>
      body {
        font-family: var(--vscode-font-family);
        color: var(--vscode-foreground);
        background: var(--vscode-editor-background);
        display: flex;
        align-items: center;
        justify-content: center;
        height: 100vh;
        margin: 0;
        padding: 20px;
        box-sizing: border-box;
      }
      .error-container {
        max-width: 600px;
        text-align: center;
      }
      h2 {
        color: var(--vscode-errorForeground);
        margin-bottom: 16px;
      }
      p {
        margin-bottom: 16px;
        line-height: 1.6;
      }
      code {
        display: block;
        background: var(--vscode-textCodeBlock-background);
        padding: 16px;
        border-radius: 6px;
        margin: 16px 0;
        font-family: var(--vscode-editor-font-family);
        font-size: 13px;
        text-align: left;
        white-space: pre-wrap;
      }
    </style>
</head>
<body>
    <div class="error-container">
      <h2>Graph Editor Not Built</h2>
      <p>The graph-viz assets are not bundled with the extension. Please build them:</p>
      <code># From the project root
cd graph-viz
npm install
npm run build

# Then rebuild the extension
cd ../build-with-me
npm run build</code>
      <p>After building, restart VS Code or reload the window.</p>
    </div>
</body>
</html>`;
}

/**
 * Generate a nonce for CSP
 */
function getNonce(): string {
  let text = '';
  const possible = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
  for (let i = 0; i < 32; i++) {
    text += possible.charAt(Math.floor(Math.random() * possible.length));
  }
  return text;
}

/**
 * Get the current panel if it exists
 */
export function getGraphPanel(): vscode.WebviewPanel | undefined {
  return currentPanel;
}
