import * as vscode from 'vscode';
import { startApiStdioServer, isApiStdioServerRunning } from '../../infrastructure/services/apiStdioServer';
import { runLSPIndex, runOnSaveLSPIndex, IndexStats } from '../../features/indexing/lspBridge';
import { startReadinessPoller, LSStatus } from '../../features/indexing/readinessPoller';
import { requestGraphRefreshIfVisible } from './graphPanel';

let panel: vscode.WebviewPanel | undefined;
let stopPoller: (() => void) | undefined;
let indexing = false;
let cancelRequested = false;

// Fix 4: track files saved during a full index so on-save can replay them after
const filesSavedDuringIndex: string[] = [];

export function isFullIndexRunning(): boolean {
  return indexing;
}

export function queueFileForPostIndex(filePath: string): void {
  filesSavedDuringIndex.push(filePath);
}

function drainPostIndexQueue(): string[] {
  const files = [...filesSavedDuringIndex];
  filesSavedDuringIndex.length = 0;
  return files;
}

function dedupeFileList(files: string[]): string[] {
  const seen = new Set<string>();
  const deduped: string[] = [];
  for (const file of files) {
    if (seen.has(file)) {
      continue;
    }
    seen.add(file);
    deduped.push(file);
  }
  return deduped;
}

function getHtml(): string {
  return `<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8" />
<meta name="viewport" content="width=device-width, initial-scale=1.0" />
<style>
body { font-family: var(--vscode-font-family); padding: 16px; color: var(--vscode-foreground); }
section { border: 1px solid var(--vscode-panel-border); border-radius: 6px; padding: 12px; margin-bottom: 12px; }
button { margin-right: 8px; padding: 6px 10px; }
.row { margin: 6px 0; }
.ok { color: #2ea043; }
.err { color: #f85149; }
#barWrap { width: 100%; height: 8px; background: var(--vscode-editorWidget-border); border-radius: 4px; overflow: hidden; }
#bar { width: 0%; height: 100%; background: var(--vscode-progressBar-background); transition: width 120ms ease; }
.small { opacity: .85; font-size: 12px; }
</style>
</head>
<body>
  <h2>Index Panel</h2>
  <section>
    <h3>Language Server Status</h3>
    <div id="ls">Checking...</div>
  </section>

  <section>
    <h3>Actions</h3>
    <button id="build">Build Index</button>
    <button id="rebuild">Rebuild Index</button>
    <button id="cancel" disabled>Cancel</button>
  </section>

  <section>
    <h3>Progress</h3>
    <div id="msg" class="row">Idle</div>
    <div id="barWrap"><div id="bar"></div></div>
    <div id="pct" class="small row">0%</div>
  </section>

  <section>
    <h3>Stats</h3>
    <div id="stats" class="small">No runs yet.</div>
  </section>

<script>
const vscode = acquireVsCodeApi();
const el = {
  ls: document.getElementById('ls'),
  build: document.getElementById('build'),
  rebuild: document.getElementById('rebuild'),
  cancel: document.getElementById('cancel'),
  msg: document.getElementById('msg'),
  bar: document.getElementById('bar'),
  pct: document.getElementById('pct'),
  stats: document.getElementById('stats')
};

let allReady = false;
let inFlight = false;

function setBusy(busy) {
  inFlight = busy;
  el.cancel.disabled = !busy;
  el.build.disabled = busy || !allReady;
  el.rebuild.disabled = busy || !allReady;
}

el.build.onclick = () => vscode.postMessage({ type: 'startIndex', rebuild: false });
el.rebuild.onclick = () => vscode.postMessage({ type: 'startIndex', rebuild: true });
el.cancel.onclick = () => vscode.postMessage({ type: 'cancelIndex' });

window.addEventListener('message', (event) => {
  const msg = event.data;

  if (msg.type === 'lsStatus') {
    const statuses = msg.statuses || [];
    allReady = statuses.length > 0 && statuses.every(s => s.ready);
    el.ls.innerHTML = statuses.map(s => {
      const icon = s.ready ? '🟢' : '🔴';
      const reason = s.ready ? 'ready' : (s.reason || 'not ready');
      const cls = s.ready ? 'ok' : 'err';
      return '<div class="row ' + cls + '">' + icon + ' ' + s.name + ' — ' + reason + '</div>';
    }).join('');
    if (!inFlight) setBusy(false);
  }

  if (msg.type === 'indexProgress') {
    setBusy(true);
    el.msg.textContent = msg.message || 'Indexing...';
    const pct = Math.max(0, Math.min(100, Number(msg.percent || 0)));
    el.bar.style.width = pct + '%';
    el.pct.textContent = pct + '%';
  }

  if (msg.type === 'indexComplete') {
    setBusy(false);
    const s = msg.stats || {};
    el.stats.textContent = 'Symbols: ' + (s.symbols ?? '-') + ' | Edges: ' + (s.edges ?? '-') + ' | Duration: ' + (s.durationMs ?? '-') + ' ms';
  }

  if (msg.type === 'indexError') {
    setBusy(false);
    el.msg.textContent = msg.message || 'Index failed';
  }
});
</script>
</body>
</html>`;
}

function postLSStatus(statuses: LSStatus[]): void {
  panel?.webview.postMessage({ type: 'lsStatus', statuses });
}

function postProgress(phase: string, message: string, percent: number): void {
  panel?.webview.postMessage({ type: 'indexProgress', phase, message, percent });
}

function postComplete(stats: IndexStats): void {
  panel?.webview.postMessage({ type: 'indexComplete', stats });
}

function postError(message: string): void {
  panel?.webview.postMessage({ type: 'indexError', message });
}

export async function openIndexPanel(context: vscode.ExtensionContext): Promise<void> {
  if (panel) {
    panel.reveal(vscode.ViewColumn.One);
    return;
  }

  panel = vscode.window.createWebviewPanel('fypIndexPanel', 'FYP Index Panel', vscode.ViewColumn.One, {
    enableScripts: true,
    retainContextWhenHidden: true,
  });

  panel.webview.html = getHtml();

  stopPoller = startReadinessPoller((statuses) => postLSStatus(statuses));

  panel.onDidDispose(() => {
    stopPoller?.();
    stopPoller = undefined;
    panel = undefined;
  });

  panel.webview.onDidReceiveMessage(async (message) => {
    if (message?.type === 'cancelIndex') {
      cancelRequested = true;
      return;
    }

    if (message?.type !== 'startIndex') {
      return;
    }

    if (indexing) {
      return;
    }

    cancelRequested = false;
    indexing = true;

    try {
      if (!isApiStdioServerRunning()) {
        const started = await startApiStdioServer(context);
        if (!started) {
          throw new Error('Failed to start API server');
        }
      }

      const stats = await runLSPIndex(Boolean(message.rebuild), (u) => {
        postProgress(u.phase, u.message, u.percent);
      }, () => cancelRequested);

      postComplete(stats);
      requestGraphRefreshIfVisible();

	      // Replay files saved during full index serially to avoid concurrent
	      // storeFileDelta writes racing each other.
	      for (;;) {
	        const batch = dedupeFileList(drainPostIndexQueue());
	        if (batch.length === 0) {
	          break;
	        }
	        for (const fp of batch) {
	          try {
	            await runOnSaveLSPIndex(fp);
	            requestGraphRefreshIfVisible();
	          } catch (err: unknown) {
	            console.warn('[FYP] post-index replay failed:', err);
	          }
	        }
	      }
	    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      postError(msg);
      vscode.window.showErrorMessage(`FYP indexing failed: ${msg}`);
    } finally {
      indexing = false;
    }
  }, undefined, context.subscriptions);
}
