/**
 * Webview Transport - Message passing with VS Code extension.
 * When graph-viz runs inside a webview, it cannot access Node.js or spawn processes.
 * All backend calls go: postMessage -> extension -> fypd stdin -> response via postMessage.
 */

// Declare VS Code API types
declare function acquireVsCodeApi(): {
  postMessage(message: unknown): void;
  getState(): unknown;
  setState(state: unknown): void;
};

// Response types
interface ApiResponse {
  type: 'apiResponse';
  id: number;
  result?: unknown;
  error?: { message: string };
}

interface ConfigMessage {
  type: 'config';
  useStdioApi?: boolean;
}

type IncomingMessage = ApiResponse | ConfigMessage;

// Cached VS Code API instance
let vscodeApi: ReturnType<typeof acquireVsCodeApi> | null = null;

/**
 * Check if running inside VS Code webview
 */
export function isVSCodeWebview(): boolean {
  try {
    return typeof acquireVsCodeApi !== 'undefined';
  } catch {
    return false;
  }
}

/**
 * Check if we should use the stdio API (message passing)
 */
export function shouldUseStdioApi(): boolean {
  if (typeof window !== 'undefined') {
    return (window as any).__USE_STDIO_API__ === true;
  }
  return false;
}

/**
 * Get the VS Code API instance (cached)
 */
export function getVSCodeApi(): ReturnType<typeof acquireVsCodeApi> | null {
  if (!isVSCodeWebview()) {
    return null;
  }
  if (!vscodeApi) {
    vscodeApi = acquireVsCodeApi();
  }
  return vscodeApi;
}

/**
 * WebviewTransport class - handles request/response over postMessage
 */
class WebviewTransport {
  private requestId = 0;
  private pending = new Map<number, {
    resolve: (result: unknown) => void;
    reject: (error: Error) => void;
    timeout: ReturnType<typeof setTimeout>;
  }>();
  private readonly REQUEST_TIMEOUT = 30000; // 30 seconds

  constructor() {
    this.setupMessageListener();
  }

  /**
   * Set up the message listener for responses from VS Code extension
   */
  private setupMessageListener(): void {
    if (typeof window === 'undefined') return;

    window.addEventListener('message', (event: MessageEvent<IncomingMessage>) => {
      const message = event.data;
      
      if (message.type === 'apiResponse') {
        this.handleResponse(message);
      } else if (message.type === 'config') {
        console.log('[WebviewTransport] Received config:', message);
      }
    });
  }

  /**
   * Handle a response from the VS Code extension
   */
  private handleResponse(response: ApiResponse): void {
    const handler = this.pending.get(response.id);
    if (!handler) {
      console.warn('[WebviewTransport] No handler for response id:', response.id);
      return;
    }

    // Clear timeout and remove from pending
    clearTimeout(handler.timeout);
    this.pending.delete(response.id);

    if (response.error) {
      handler.reject(new Error(response.error.message));
    } else {
      handler.resolve(response.result);
    }
  }

  /**
   * Send a request to the VS Code extension and wait for response
   */
  async request<T = unknown>(method: string, params: Record<string, unknown> = {}): Promise<T> {
    const api = getVSCodeApi();
    if (!api) {
      throw new Error('VS Code API not available');
    }

    const id = ++this.requestId;

    return new Promise<T>((resolve, reject) => {
      // Set up timeout
      const timeout = setTimeout(() => {
        this.pending.delete(id);
        reject(new Error(`Request timeout: ${method}`));
      }, this.REQUEST_TIMEOUT);

      // Store pending request
      this.pending.set(id, {
        resolve: resolve as (result: unknown) => void,
        reject,
        timeout
      });

      // Send message to VS Code extension
      api.postMessage({
        type: 'apiRequest',
        id,
        method,
        params
      });
    });
  }

  /**
   * Cancel all pending requests (e.g., on unmount)
   */
  cancelAll(): void {
    for (const [, handler] of this.pending) {
      clearTimeout(handler.timeout);
      handler.reject(new Error('Request cancelled'));
    }
    this.pending.clear();
  }
}

// Singleton instance
let transportInstance: WebviewTransport | null = null;

/**
 * Get the WebviewTransport singleton
 */
export function getWebviewTransport(): WebviewTransport {
  if (!transportInstance) {
    transportInstance = new WebviewTransport();
  }
  return transportInstance;
}

/**
 * Send an API request via the webview transport
 * This is the main function used by graphApi.ts
 */
export async function sendRequest<T = unknown>(
  method: string,
  params: Record<string, unknown> = {}
): Promise<T> {
  return getWebviewTransport().request<T>(method, params);
}

/**
 * Post a message to VS Code (for non-API messages like showInfo, openMasking, etc.)
 */
export function postMessage(message: unknown): void {
  const api = getVSCodeApi();
  if (api) {
    api.postMessage(message);
  } else {
    console.warn('[WebviewTransport] Cannot post message - VS Code API not available');
  }
}
