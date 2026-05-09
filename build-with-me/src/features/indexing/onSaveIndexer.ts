import * as path from 'path';
import * as vscode from 'vscode';
import { runOnSaveLSPIndex } from './lspBridge';
import { getPluginForFilePath } from './languageLSPPlugins';
import { requestGraphRefreshIfVisible } from '../../ui/panels/graphPanel';
import { isFullIndexRunning, queueFileForPostIndex } from '../../ui/panels/indexPanel';

const DEBOUNCE_MS = 900;

export function registerOnSaveIndexing(
  context: vscode.ExtensionContext,
  ensureApiServer: () => Promise<boolean>
): void {
  const debounceTimers = new Map<string, NodeJS.Timeout>();
  const queuedFiles: string[] = [];
  const queuedSet = new Set<string>();
  let processing = false;

  const enqueueFile = (filePath: string) => {
    if (queuedSet.has(filePath)) {
      return;
    }
    queuedSet.add(filePath);
    queuedFiles.push(filePath);
    void processQueue();
  };

  const processQueue = async () => {
    if (processing) {
      return;
    }
    processing = true;

    try {
      while (queuedFiles.length > 0) {
        const filePath = queuedFiles.shift();
        if (!filePath) {
          continue;
        }
        queuedSet.delete(filePath);

        const enabled = vscode.workspace.getConfiguration('fyp').get<boolean>('indexOnSave', true);
        if (!enabled) {
          continue;
        }
        const plugin = getPluginForFilePath(filePath);
        if (!plugin) {
          continue;
        }
        const readiness = await plugin.checkReady();
        if (!readiness.ready) {
          vscode.window.setStatusBarMessage(`$(sync~spin) FYP on-save skipped: ${readiness.reason ?? 'language server not ready'}`, 4000);
          continue;
        }

        // Fix 4: suppress on-save during full index to avoid overwriting stale data
        if (isFullIndexRunning()) {
          queueFileForPostIndex(filePath);
          vscode.window.setStatusBarMessage('$(sync~spin) FYP: on-save queued (full index running)', 4000);
          continue;
        }

        const started = await ensureApiServer();
        if (!started) {
          continue;
        }

        try {
          await runOnSaveLSPIndex(filePath);
          requestGraphRefreshIfVisible();
        } catch (err) {
          const msg = err instanceof Error ? err.message : String(err);
          console.warn('[FYP] on-save indexing failed:', msg);
          vscode.window.setStatusBarMessage(`$(warning) FYP on-save indexing failed: ${path.basename(filePath)}`, 6000);
        }
      }
    } finally {
      processing = false;
    }
  };

  context.subscriptions.push(
    vscode.workspace.onDidSaveTextDocument((doc) => {
      if (doc.uri.scheme !== 'file') {
        return;
      }

      const enabled = vscode.workspace.getConfiguration('fyp').get<boolean>('indexOnSave', true);
      if (!enabled) {
        return;
      }

      const filePath = doc.uri.fsPath;
      const plugin = getPluginForFilePath(filePath);
      if (!plugin) {
        return;
      }

      const existing = debounceTimers.get(filePath);
      if (existing) {
        clearTimeout(existing);
      }

      const timer = setTimeout(() => {
        debounceTimers.delete(filePath);
        enqueueFile(filePath);
      }, DEBOUNCE_MS);

      debounceTimers.set(filePath, timer);
    })
  );

  context.subscriptions.push({
    dispose: () => {
      for (const timer of debounceTimers.values()) {
        clearTimeout(timer);
      }
      debounceTimers.clear();
      queuedFiles.length = 0;
      queuedSet.clear();
    },
  });
}
