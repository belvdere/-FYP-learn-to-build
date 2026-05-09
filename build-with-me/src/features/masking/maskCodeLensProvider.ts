/**
 * Mask CodeLens Provider
 * Scans open files for [MASK:...] markers in comment syntax for each language and provides:
 * - Inline CodeLens hints on each mask line
 * - "Validate All Masks" CodeLens at the top of files with masks
 * 
 * Tracks session IDs per file so the "Validate" button persists
 * even after the user fills in and removes the mask comments.
 */

import * as vscode from 'vscode';
import { ValidationClient, ValidationResult } from '../../infrastructure/clients/validationClient';

/**
 * Regexes to match mask markers with session ID:
 *   /* [MASK:session=<sessionId> id=<maskId> hint="<hint>"] *\/
 *   # [MASK:session=<sessionId> id=<maskId> hint="<hint>"]
 */
const MASK_REGEXES: RegExp[] = [
  /\/\* \[MASK:session=([^\s]+)\s+id=([^\s]+)\s+hint="([^"]*)"\] \*\//g,
  /# \[MASK:session=([^\s]+)\s+id=([^\s]+)\s+hint="([^"]*)"\]/g,
];

/**
 * Provides CodeLens hints for [MASK] markers in files.
 * Tracks session IDs so the "Validate" button stays visible
 * after mask comments are removed.
 */
export class MaskCodeLensProvider implements vscode.CodeLensProvider {
  private _onDidChangeCodeLenses = new vscode.EventEmitter<void>();
  readonly onDidChangeCodeLenses = this._onDidChangeCodeLenses.event;

  /**
   * Tracks file URI → session IDs discovered from mask comments.
   * Persists after mask comments are removed so the "Validate" button stays.
   */
  private _trackedSessions = new Map<string, Set<string>>();

  /**
   * Refresh CodeLenses (call after validation or edits)
   */
  refresh(): void {
    this._onDidChangeCodeLenses.fire();
  }

  /**
   * Get tracked session IDs for a file
   */
  getSessionIds(uri: vscode.Uri): Set<string> {
    return this._trackedSessions.get(uri.toString()) ?? new Set();
  }

  /**
   * Clear tracked sessions for a file (call after successful validation)
   */
  clearSessions(uri: vscode.Uri): void {
    this._trackedSessions.delete(uri.toString());
  }

  provideCodeLenses(
    document: vscode.TextDocument,
    _token: vscode.CancellationToken
  ): vscode.CodeLens[] {
    const codeLenses: vscode.CodeLens[] = [];
    const text = document.getText();
    const fileKey = document.uri.toString();

    // Scan for mask comments (supports block and line comment styles).
    for (const regex of MASK_REGEXES) {
      let match: RegExpExecArray | null;
      regex.lastIndex = 0;

      while ((match = regex.exec(text)) !== null) {
        const sessionId = match[1]; // UUID from database
        const _maskId = match[2];   // mask_xxxx (unused here, but available)
        const hint = match[3];
        const position = document.positionAt(match.index);
        const range = new vscode.Range(position, position);

        // Track the session ID (UUID) for this file — this is what the backend needs
        if (!this._trackedSessions.has(fileKey)) {
          this._trackedSessions.set(fileKey, new Set());
        }
        this._trackedSessions.get(fileKey)!.add(sessionId);

        // Add a CodeLens hint for this mask
        codeLenses.push(
          new vscode.CodeLens(range, {
            title: `🔲 [MASK] ${hint}`,
            command: '',
            arguments: []
          })
        );
      }
    }

    // Show "Validate" button if:
    //   - File currently has mask comments (unfilled), OR
    //   - File has tracked sessions from earlier (masks were filled/removed)
    const hasTrackedSessions = this._trackedSessions.has(fileKey)
      && this._trackedSessions.get(fileKey)!.size > 0;

    if (hasTrackedSessions) {
      const sessionCount = this._trackedSessions.get(fileKey)!.size;
      const hasMaskComments = codeLenses.length > 0;
      const statusLabel = hasMaskComments
        ? `Validate All Masks (${sessionCount} session${sessionCount > 1 ? 's' : ''})`
        : `Validate Filled Code (${sessionCount} session${sessionCount > 1 ? 's' : ''})`;

      const topRange = new vscode.Range(0, 0, 0, 0);
      codeLenses.unshift(
        new vscode.CodeLens(topRange, {
          title: statusLabel,
          command: 'fyp.validateMasks',
          arguments: [document.uri]
        })
      );
    }

    return codeLenses;
  }
}

/**
 * Diagnostics collection for mask validation results
 */
let diagnosticCollection: vscode.DiagnosticCollection;

/**
 * Register the CodeLens provider and validation command.
 * Call this from extension.ts activate().
 */
export function registerMaskCodeLens(context: vscode.ExtensionContext): void {
  const provider = new MaskCodeLensProvider();

  // Register for all file types (masks could be in any language)
  context.subscriptions.push(
    vscode.languages.registerCodeLensProvider({ scheme: 'file' }, provider)
  );

  // Create diagnostics collection
  diagnosticCollection = vscode.languages.createDiagnosticCollection('fyp-masks');
  context.subscriptions.push(diagnosticCollection);

  // Register validate command
  context.subscriptions.push(
    vscode.commands.registerCommand('fyp.validateMasks', async (uri?: vscode.Uri) => {
      const targetUri = uri || vscode.window.activeTextEditor?.document.uri;
      if (!targetUri) {
        vscode.window.showWarningMessage('No file open to validate');
        return;
      }

      const document = await vscode.workspace.openTextDocument(targetUri);
      await validateMasksInDocument(context, document, diagnosticCollection, provider);
    })
  );

  // Refresh CodeLenses when documents change
  context.subscriptions.push(
    vscode.workspace.onDidChangeTextDocument(() => {
      provider.refresh();
    })
  );
}

/**
 * Validate all masks in a document using the backend validation service.
 * Uses tracked session IDs from the provider (persists after mask comments removed).
 */
async function validateMasksInDocument(
  context: vscode.ExtensionContext,
  document: vscode.TextDocument,
  diagnostics: vscode.DiagnosticCollection,
  provider: MaskCodeLensProvider
): Promise<void> {
  const text = document.getText();
  const filePath = vscode.workspace.asRelativePath(document.uri);

  // Get session IDs from tracked sessions (survives mask comment removal)
  const sessionIds = provider.getSessionIds(document.uri);

  if (sessionIds.size === 0) {
    vscode.window.showInformationMessage('No mask sessions found for this file. Open a file with [MASK] markers first.');
    return;
  }

  // Check if masks still have unfilled placeholders
  const hasUnfilledMasks = text.includes('[MASK:session=');
  if (hasUnfilledMasks) {
    const proceed = await vscode.window.showWarningMessage(
      'Some [MASK] markers are still present. Validate anyway?',
      'Yes',
      'No'
    );
    if (proceed !== 'Yes') {
      return;
    }
  }

  // Validate using the backend
  const client = new ValidationClient(context);
  const allDiagnostics: vscode.Diagnostic[] = [];

  try {
    await vscode.window.withProgress(
      {
        location: vscode.ProgressLocation.Notification,
        title: 'Validating masks...',
        cancellable: false
      },
      async () => {
        for (const sessionId of sessionIds) {
          try {
            const result: ValidationResult = await client.validateCode({
              code: text,
              filePath,
              sessionId
            });

            if (!result.passed) {
              for (const stageResult of result.results) {
                if (stageResult.errors) {
                  for (const error of stageResult.errors) {
                    const line = error.line ? error.line - 1 : 0;
                    const range = new vscode.Range(line, 0, line, Number.MAX_SAFE_INTEGER);
                    const diagnostic = new vscode.Diagnostic(
                      range,
                      `[${stageResult.stage}] ${error.message}`,
                      error.severity === 'error'
                        ? vscode.DiagnosticSeverity.Error
                        : vscode.DiagnosticSeverity.Warning
                    );
                    diagnostic.source = 'FYP Mask Validation';
                    allDiagnostics.push(diagnostic);
                  }
                }
              }
            }
          } catch (err) {
            const message = err instanceof Error ? err.message : String(err);
            vscode.window.showErrorMessage(`Validation error for session ${sessionId}: ${message}`);
          }
        }
      }
    );
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    vscode.window.showErrorMessage(`Validation failed: ${message}`);
    return;
  }

  // Update diagnostics
  diagnostics.set(document.uri, allDiagnostics);

  // Show result summary
  if (allDiagnostics.length === 0) {
    vscode.window.showInformationMessage('No parse errors found. Good Job! 🎉 Ask Copilot to "validate your code.');
    // Clear tracked sessions — exercise is complete
    provider.clearSessions(document.uri);
  } else {
    const errorCount = allDiagnostics.filter(d => d.severity === vscode.DiagnosticSeverity.Error).length;
    const warnCount = allDiagnostics.filter(d => d.severity === vscode.DiagnosticSeverity.Warning).length;
    vscode.window.showWarningMessage(
      `Validation: ${errorCount} error(s), ${warnCount} warning(s). See Problems panel.`
    );
    // Keep tracked sessions so user can fix and re-validate
  }

  // Refresh CodeLenses
  provider.refresh();
}
