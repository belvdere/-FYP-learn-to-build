/**
 * API Stdio Server - Manages fypd serve-api-stdio process.
 * Spawns fypd with stdin/stdout; parses newline-delimited JSON for requests/responses.
 * graph-viz webview -> postMessage -> extension -> stdin -> fypd -> stdout -> extension -> postMessage.
 */

import * as vscode from 'vscode';
import { ChildProcess } from 'child_process';
import { EventEmitter } from 'events';
import { spawnFypd, getFypdBinaryPath } from '../../utils/processHelper';

let apiStdioProcess: ChildProcess | undefined;
let outputChannel: vscode.OutputChannel | undefined;
let requestId = 0;
let pendingRequests: Map<number, {
  resolve: (result: unknown) => void;
  reject: (error: Error) => void;
}> = new Map();
let responseBuffer = '';

// Event emitter for notifications from the server
const notificationEmitter = new EventEmitter();

/**
 * Notification types from the server
 */
export interface IndexProgressNotification {
  message: string;
  percent: number;
}

export interface IndexErrorNotification {
  message: string;
}

/**
 * Subscribe to notifications from the server
 */
export function onNotification(
  method: string,
  listener: (params: unknown) => void
): () => void {
  notificationEmitter.on(method, listener);
  return () => notificationEmitter.off(method, listener);
}

/**
 * Subscribe to index progress notifications
 */
export function onIndexProgress(
  listener: (progress: IndexProgressNotification) => void
): () => void {
  return onNotification('index.progress', listener as (params: unknown) => void);
}

/**
 * Subscribe to index error notifications
 */
export function onIndexError(
  listener: (error: IndexErrorNotification) => void
): () => void {
  return onNotification('index.error', listener as (params: unknown) => void);
}

/**
 * Get or create the output channel for API server logs
 */
function getOutputChannel(): vscode.OutputChannel {
  if (!outputChannel) {
    outputChannel = vscode.window.createOutputChannel('FYP API Stdio Server');
  }
  return outputChannel;
}

/**
 * Start the API stdio server
 */
export async function startApiStdioServer(
  context: vscode.ExtensionContext,
  options: { showOutput?: boolean } = {}
): Promise<boolean> {
  // If already running, just return
  if (apiStdioProcess && !apiStdioProcess.killed) {
    console.log('[FYP] API stdio server already running');
    return true;
  }

  // Reset module-level state so a restart after a crash starts clean
  requestId = 0;
  pendingRequests = new Map();
  responseBuffer = '';

  const workspaceRoot = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
  if (!workspaceRoot) {
    vscode.window.showWarningMessage('FYP: No workspace folder open');
    return false;
  }

  const bin = getFypdBinaryPath(context.extensionUri.fsPath);
  if (!bin) {
    vscode.window.showWarningMessage(
      `FYP: Server binary not found. Please build it first:\ncd server && make build`
    );
    return false;
  }

  const channel = getOutputChannel();
  channel.appendLine(`[${new Date().toISOString()}] Starting API stdio server...`);
  channel.appendLine(`Binary: ${bin}`);
  channel.appendLine(`Workspace: ${workspaceRoot}`);

  try {
    apiStdioProcess = spawnFypd(
      bin,
      ['serve-api-stdio', '--workspace', workspaceRoot],
      workspaceRoot,
      { stdio: ['pipe', 'pipe', 'pipe'] }
    );

    // Parse stdout: fypd sends one JSON object per line (response or notification)
    apiStdioProcess.stdout?.on('data', (data: Buffer) => {
      responseBuffer += data.toString();
      // Split by newline; keep incomplete line in buffer for next chunk
      const lines = responseBuffer.split('\n');
      responseBuffer = lines.pop() || ''; // Keep incomplete line in buffer
      
      for (const line of lines) {
        if (!line.trim()) {
          continue;
        }
        
        try {
          const message = JSON.parse(line);
          handleMessage(message);
        } catch (err) {
          channel.appendLine(`[parse error] ${line}`);
          console.error('[fypd-api-stdio] Failed to parse message:', err);
        }
      }
    });

    // Handle stderr - log messages
    apiStdioProcess.stderr?.on('data', (data: Buffer) => {
      const msg = data.toString().trim();
      if (msg) {
        channel.appendLine(`[stderr] ${msg}`);
        console.log('[fypd-api-stdio]', msg);
      }
    });

    // Shared teardown: reject all pending requests and reset module state.
    // The flag prevents the error + exit events from both running teardown.
    let tornDown = false;
    const teardown = (reason: string) => {
      if (tornDown) {
        return;
      }
      tornDown = true;
      responseBuffer = '';
      for (const [, handler] of pendingRequests) {
        handler.reject(new Error(reason));
      }
      pendingRequests.clear();
      apiStdioProcess = undefined;
    };

    apiStdioProcess.on('error', (err) => {
      channel.appendLine(`[error] ${err.message}`);
      console.error('[fypd-api-stdio error]', err);
      teardown(`Server error: ${err.message}`);
    });

    apiStdioProcess.on('exit', (code) => {
      channel.appendLine(`[${new Date().toISOString()}] Server exited with code ${code}`);
      console.log('[fypd-api-stdio exited]', code);
      teardown(`Server exited with code ${code}`);
    });

    // Wait a moment to check if server started successfully
    await new Promise(resolve => setTimeout(resolve, 500));

    if (apiStdioProcess && !apiStdioProcess.killed) {
      console.log('[FYP] API stdio server started successfully');
      
      if (options.showOutput) {
        channel.show();
      }
      
      return true;
    } else {
      return false;
    }
  } catch (error) {
    const msg = error instanceof Error ? error.message : String(error);
    channel.appendLine(`[error] Failed to start: ${msg}`);
    vscode.window.showErrorMessage(`FYP: Failed to start API stdio server: ${msg}`);
    return false;
  }
}

/**
 * Handle a message from the stdio server (either response or notification)
 */
function handleMessage(message: {
  id?: number;
  method?: string;
  params?: unknown;
  result?: unknown;
  error?: { code?: number; message: string; data?: unknown }
}) {
  // id present = response to a pending request; no id = notification (e.g. index.progress)
  if (message.id !== undefined) {
    handleResponse(message);
  } else if (message.method) {
    // This is a notification
    handleNotification(message.method, message.params);
  } else {
    console.warn('[fypd-api-stdio] Unknown message format:', message);
  }
}

/**
 * Handle a response to a request
 */
function handleResponse(response: { id?: number; result?: unknown; error?: { code?: number; message: string; data?: unknown } }) {
  if (response.id === undefined) {
    return;
  }

  const handler = pendingRequests.get(response.id);
  if (!handler) {
    console.warn('[fypd-api-stdio] No handler for response id:', response.id);
    return;
  }

  pendingRequests.delete(response.id);

  if (response.error) {
    const detail = response.error.data !== null && response.error.data !== undefined
      ? `: ${JSON.stringify(response.error.data)}`
      : '';
    handler.reject(new Error(`${response.error.message}${detail}`));
  } else {
    handler.resolve(response.result);
  }
}

/**
 * Handle a notification from the server
 */
function handleNotification(method: string, params: unknown) {
  const channel = getOutputChannel();
  channel.appendLine(`[notification] ${method}: ${JSON.stringify(params)}`);
  
  // Emit the notification for listeners
  notificationEmitter.emit(method, params);
}

/**
 * Write data to a writable stream, respecting backpressure.
 * Returns a promise that resolves once the OS buffer has drained (safe to write more).
 * write() returning false means the internal buffer is full — data IS queued, but
 * we should wait for 'drain' before sending more to avoid unbounded memory buffering.
 */
function drainWrite(stream: NodeJS.WritableStream, data: string): Promise<void> {
  return new Promise<void>((resolve, reject) => {
    const flushed = stream.write(data, (err) => {
      if (err) { reject(err); }
    });
    if (flushed) {
      resolve();
    } else {
      // Backpressure: OS buffer full — wait until drained, then resolve
      stream.once('drain', resolve);
      stream.once('error', reject);
    }
  });
}

/**
 * Send a request to the API stdio server.
 * @param timeoutMs Override the default 30s timeout (e.g. for large index.storeEdges payloads)
 */
export async function sendApiRequest<T = unknown>(
  method: string,
  params: Record<string, unknown> = {},
  timeoutMs = 30_000
): Promise<T> {
  if (!apiStdioProcess || apiStdioProcess.killed) {
    throw new Error('API stdio server is not running');
  }

  const id = ++requestId;

  const request = { id, method, params };

  return new Promise<T>((resolve, reject) => {
    const timeoutHandle = setTimeout(() => {
      if (pendingRequests.has(id)) {
        pendingRequests.delete(id);
        reject(new Error(`Request timed out: ${method}`));
      }
    }, timeoutMs);

    pendingRequests.set(id, {
      resolve: (result: unknown) => { clearTimeout(timeoutHandle); resolve(result as T); },
      reject: (err: Error) => { clearTimeout(timeoutHandle); reject(err); },
    });

    const stdin = apiStdioProcess!.stdin;
    if (!stdin) {
      clearTimeout(timeoutHandle);
      pendingRequests.delete(id);
      reject(new Error('API server stdin not available'));
      return;
    }

    const data = JSON.stringify(request) + '\n';
    drainWrite(stdin, data).catch((err: Error) => {
      clearTimeout(timeoutHandle);
      pendingRequests.delete(id);
      reject(err);
    });
  });
}

/**
 * Stop the API stdio server
 */
export function stopApiStdioServer(): void {
  if (apiStdioProcess) {
    // Reject pending requests before killing so callers unblock immediately
    for (const [, handler] of pendingRequests) {
      handler.reject(new Error('Server stopped'));
    }
    pendingRequests.clear();
    responseBuffer = '';

    apiStdioProcess.kill();
    apiStdioProcess = undefined;
    console.log('[FYP] API stdio server stopped');

    if (outputChannel) {
      outputChannel.appendLine(`[${new Date().toISOString()}] Server stopped`);
    }
  }
}

/**
 * Get the API stdio server process (for monitoring)
 */
export function getApiStdioServer(): ChildProcess | undefined {
  return apiStdioProcess;
}

/**
 * Check if the API stdio server is running
 */
export function isApiStdioServerRunning(): boolean {
  return apiStdioProcess !== undefined && !apiStdioProcess.killed;
}

/**
 * Show the API stdio server output
 */
export function showApiStdioOutput(): void {
  getOutputChannel().show();
}

/**
 * Refresh the API stdio server's database connection.
 * This is called after indexing completes to ensure the server sees the latest data.
 * Uses the db.refresh API call which is much faster than restarting the process.
 * Only refreshes if the server is currently running.
 */
export async function refreshApiStdioDatabase(): Promise<boolean> {
  // Only refresh if server is running
  if (!isApiStdioServerRunning()) {
    console.log('[FYP] API stdio server not running, no refresh needed');
    return true;
  }

  const channel = getOutputChannel();
  channel.appendLine(`[${new Date().toISOString()}] Refreshing database connection...`);
  console.log('[FYP] Refreshing API stdio server database connection...');

  try {
    const result = await sendApiRequest<{ status: string; message: string }>('db.refresh');
    
    channel.appendLine(`[${new Date().toISOString()}] Database refreshed: ${result.message}`);
    console.log('[FYP] Database connection refreshed successfully');
    return true;
  } catch (error) {
    const msg = error instanceof Error ? error.message : String(error);
    channel.appendLine(`[${new Date().toISOString()}] Failed to refresh database: ${msg}`);
    console.error('[FYP] Failed to refresh database connection:', msg);
    return false;
  }
}
