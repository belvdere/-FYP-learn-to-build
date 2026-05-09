import * as path from 'path';
import * as vscode from 'vscode';
import { sendApiRequest } from '../../infrastructure/services/apiStdioServer';

// Max edges per index.storeEdges / index.appendEdges call (~500 edges × ~150 bytes = ~75 KB per chunk)
const EDGE_BATCH_SIZE = 500;

export interface ScannedSymbol {
  id: string;
  name: string;
  kind: string;
  filePath: string;
  uri: string;
  line: number;
  endLine: number;
  col: number;
  signature?: string;
  bodyHash?: string;
}

export interface ResolvedEdge {
  callerSymbol: string;
  calleeSymbol: string;
  callType: string;
  filePath: string;
  line: number;
}

export interface ProgressUpdate {
  phase: 'scan' | 'lsp' | 'store' | 'done';
  message: string;
  percent: number;
}

export interface IndexStats {
  symbols: number;
  edges: number;
  durationMs: number;
}

interface IndexScanResponse {
  symbols: ScannedSymbol[];
}

interface IndexStoreResponse {
  success: boolean;
  stats?: IndexStats;
}

interface IndexScanFileResponse {
  symbols: ScannedSymbol[];
  changedCallerIds: string[];
  removedCallerIds: string[];
  fileHash: string;
}

interface ResolveLocation {
  filePath: string;
  line: number;
}

interface ResolveLocationResponse {
  resolved: Array<ResolveLocation & { symbolId?: string }>;
}

interface StoreFileDeltaResponse {
  success: boolean;
  stats?: IndexStats;
}

interface PendingLocationEdge {
  callerSymbol: string;
  calleeFilePath: string;
  calleeLine: number;
  filePath: string;
  line: number;
}

interface SymbolInterval {
  id: string;
  startLine: number;
  endLine: number;
}

interface EdgeResolutionStats {
  pending: number;
  resolved: number;
  unresolved: number;
}

interface ResolveEdgesResult {
  edges: ResolvedEdge[];
  stats: EdgeResolutionStats;
}

function edgeKey(edge: ResolvedEdge): string {
  return `${edge.callerSymbol}|${edge.calleeSymbol}|${edge.filePath}|${edge.line}`;
}

function canonicalPath(filePath: string): string {
  return path.normalize(path.resolve(filePath));
}

function isPathWithinWorkspace(filePath: string, workspaceRoot: string): boolean {
  const canonicalFile = canonicalPath(filePath);
  const canonicalRoot = canonicalPath(workspaceRoot);
  return canonicalFile === canonicalRoot || canonicalFile.startsWith(canonicalRoot + path.sep);
}

function locationKey(filePath: string, line: number): string {
  return `${canonicalPath(filePath)}:${line}`;
}

function buildSymbolIntervalIndex(symbols: ScannedSymbol[]): Map<string, SymbolInterval[]> {
  const map = new Map<string, SymbolInterval[]>();
  for (const sym of symbols) {
    if (!sym.filePath || sym.line <= 0 || !sym.id) {
      continue;
    }
    const startLine = sym.line;
    const endLine = sym.endLine > 0 ? Math.max(sym.endLine, startLine) : startLine;
    const fileKey = canonicalPath(sym.filePath);
    const intervals = map.get(fileKey) ?? [];
    intervals.push({ id: sym.id, startLine, endLine });
    map.set(fileKey, intervals);
  }

  // Stable ordering ensures deterministic tiebreaks.
  for (const intervals of map.values()) {
    intervals.sort((a, b) => {
      if (a.startLine !== b.startLine) {
        return a.startLine - b.startLine;
      }
      if (a.endLine !== b.endLine) {
        return a.endLine - b.endLine;
      }
      return a.id.localeCompare(b.id);
    });
  }

  return map;
}

function resolveSymbolIDByInterval(
  filePath: string,
  line: number,
  symbolIntervalsByFile: Map<string, SymbolInterval[]>
): string | undefined {
  if (!filePath || line <= 0) {
    return undefined;
  }

  const candidates = symbolIntervalsByFile.get(canonicalPath(filePath));
  if (!candidates || candidates.length === 0) {
    return undefined;
  }

  let best: SymbolInterval | undefined;
  let bestSpan = Number.POSITIVE_INFINITY;

  for (const candidate of candidates) {
    if (line < candidate.startLine || line > candidate.endLine) {
      continue;
    }

    const span = candidate.endLine - candidate.startLine;
    if (!best || span < bestSpan || (span === bestSpan && candidate.startLine < best.startLine)) {
      best = candidate;
      bestSpan = span;
    }
  }

  return best?.id;
}

async function collectOutgoingCallLocations(
  sym: ScannedSymbol,
  workspaceRoot: string
): Promise<PendingLocationEdge[]> {
  const uri = vscode.Uri.file(sym.filePath);
  const pos = new vscode.Position(Math.max(sym.line - 1, 0), Math.max(sym.col, 0));

  let items: vscode.CallHierarchyItem[];
  try {
    items = (await vscode.commands.executeCommand<vscode.CallHierarchyItem[]>(
      'vscode.prepareCallHierarchy',
      uri,
      pos
    )) ?? [];
  } catch {
    // LSP not ready for this file (e.g. tsserver "No Project", JDTLS still initializing)
    return [];
  }

  const edges: PendingLocationEdge[] = [];

  for (const item of items) {
    let outgoing: vscode.CallHierarchyOutgoingCall[];
    try {
      outgoing = (await vscode.commands.executeCommand<vscode.CallHierarchyOutgoingCall[]>(
        'vscode.provideOutgoingCalls',
        item
      )) ?? [];
    } catch {
      // LSP threw for this item (e.g. tsserver "No Project", JDTLS "Internal error") — skip
      continue;
    }

    for (const call of outgoing) {
      const calleePath = call.to.uri.fsPath;
      if (!isPathWithinWorkspace(calleePath, workspaceRoot)) {
        continue;
      }

      const calleeLine = call.to.selectionRange.start.line + 1;
      edges.push({
        callerSymbol: sym.id,
        calleeFilePath: calleePath,
        calleeLine,
        filePath: sym.filePath,
        line: (call.fromRanges[0]?.start.line ?? (sym.line - 1)) + 1,
      });
    }
  }

  return edges;
}

function resolveEdgesFromLocations(
  pendingEdges: PendingLocationEdge[],
  symbolIntervalsByFile: Map<string, SymbolInterval[]>
): ResolveEdgesResult {
  const edgeSet = new Map<string, ResolvedEdge>();
  const stats: EdgeResolutionStats = { pending: pendingEdges.length, resolved: 0, unresolved: 0 };

  for (const edge of pendingEdges) {
    const calleeSymbol = resolveSymbolIDByInterval(edge.calleeFilePath, edge.calleeLine, symbolIntervalsByFile);
    if (!calleeSymbol) {
      stats.unresolved++;
      continue;
    }
    stats.resolved++;

    const resolved: ResolvedEdge = {
      callerSymbol: edge.callerSymbol,
      calleeSymbol,
      callType: 'direct',
      filePath: edge.filePath,
      line: edge.line,
    };
    edgeSet.set(edgeKey(resolved), resolved);
  }

  return { edges: Array.from(edgeSet.values()), stats };
}

function resolveEdgesFromResolvedLocations(
  pendingEdges: PendingLocationEdge[],
  locationToSymbolID: Map<string, string>
): ResolvedEdge[] {
  const edgeSet = new Map<string, ResolvedEdge>();

  for (const edge of pendingEdges) {
    const calleeSymbol = locationToSymbolID.get(locationKey(edge.calleeFilePath, edge.calleeLine));
    if (!calleeSymbol) {
      continue;
    }

    const resolved: ResolvedEdge = {
      callerSymbol: edge.callerSymbol,
      calleeSymbol,
      callType: 'direct',
      filePath: edge.filePath,
      line: edge.line,
    };
    edgeSet.set(edgeKey(resolved), resolved);
  }

  return Array.from(edgeSet.values());
}

export async function runLSPIndex(
  rebuild: boolean,
  onProgress: (update: ProgressUpdate) => void,
  isCancelled: () => boolean
): Promise<IndexStats> {
  const workspaceRoot = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
  if (!workspaceRoot) {
    throw new Error('No workspace folder open');
  }

  const startedAt = Date.now();

  onProgress({ phase: 'scan', message: 'Scanning source files...', percent: 5 });
  const scan = await sendApiRequest<IndexScanResponse>('index.scan', { rebuild });
  const symbols = scan.symbols ?? [];
  const symbolIntervalsByFile = buildSymbolIntervalIndex(symbols);

  const allPendingEdges: PendingLocationEdge[] = [];

  for (let i = 0; i < symbols.length; i++) {
    if (isCancelled()) {
      throw new Error('Indexing cancelled');
    }

    const sym = symbols[i];
    onProgress({
      phase: 'lsp',
      message: `Resolving calls (${i + 1}/${symbols.length})`,
      percent: Math.min(89, Math.round(10 + (80 * (i + 1)) / Math.max(symbols.length, 1))),
    });

    const calls = await collectOutgoingCallLocations(sym, workspaceRoot);
    allPendingEdges.push(...calls);
  }

  if (isCancelled()) {
    throw new Error('Indexing cancelled');
  }

  const { edges, stats: resolutionStats } = resolveEdgesFromLocations(allPendingEdges, symbolIntervalsByFile);
  console.info(
    `[FYP][Index] resolved=${resolutionStats.resolved}/${resolutionStats.pending}, unresolved=${resolutionStats.unresolved}, edges=${edges.length}`
  );

  // Phase 3 — store (batched to stay under stdin buffer limits)
  const firstBatch = edges.slice(0, EDGE_BATCH_SIZE);
  const remainingEdges = edges.slice(EDGE_BATCH_SIZE);

  onProgress({ phase: 'store', message: 'Storing call graph...', percent: 92 });
  const result = await sendApiRequest<IndexStoreResponse>('index.storeEdges', {
    edges: firstBatch,
    symbols,
    rebuild,
  });
  if (!result.success) {
    throw new Error('Failed to store indexed edges');
  }

  // Append remaining edge batches
  for (let i = 0; i < remainingEdges.length; i += EDGE_BATCH_SIZE) {
    const batch = remainingEdges.slice(i, i + EDGE_BATCH_SIZE);
    const stored = Math.min(firstBatch.length + i + batch.length, edges.length);
    onProgress({
      phase: 'store',
      message: `Storing edges (${stored}/${edges.length})...`,
      percent: Math.round(92 + 7 * stored / edges.length),
    });
    const batchResult = await sendApiRequest<{ success: boolean }>('index.appendEdges', { edges: batch });
    if (!batchResult.success) {
      throw new Error('Failed to append edge batch');
    }
  }

  const durationMs = Date.now() - startedAt;
  const stats: IndexStats = {
    symbols: symbols.length,
    edges: edges.length,
    durationMs,
  };

  onProgress({ phase: 'done', message: 'Index complete', percent: 100 });
  return result.stats ?? stats;
}

export async function runOnSaveLSPIndex(filePath: string): Promise<IndexStats> {
  const workspaceRoot = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
  if (!workspaceRoot) {
    throw new Error('No workspace folder open');
  }

  const startedAt = Date.now();

  const scan = await sendApiRequest<IndexScanFileResponse>('index.scanFile', { filePath });
  const symbols = scan.symbols ?? [];
  const changedCallerIds = scan.changedCallerIds ?? [];
  const removedCallerIds = scan.removedCallerIds ?? [];

  const changedSet = new Set(changedCallerIds);
  const changedSymbols = symbols.filter((s) => changedSet.has(s.id));

  const allPendingEdges: PendingLocationEdge[] = [];
  for (const sym of changedSymbols) {
    const calls = await collectOutgoingCallLocations(sym, workspaceRoot);
    allPendingEdges.push(...calls);
  }

  const uniqueLocations = new Map<string, ResolveLocation>();
  for (const e of allPendingEdges) {
    const key = locationKey(e.calleeFilePath, e.calleeLine);
    if (!uniqueLocations.has(key)) {
      uniqueLocations.set(key, { filePath: e.calleeFilePath, line: e.calleeLine });
    }
  }

  const resolveResponse = await sendApiRequest<ResolveLocationResponse>('index.resolveSymbolsByLocation', {
    locations: Array.from(uniqueLocations.values()),
  });

  const locationToSymbolID = new Map<string, string>();
  for (const item of resolveResponse.resolved ?? []) {
    if (!item.symbolId) {
      continue;
    }
    locationToSymbolID.set(locationKey(item.filePath, item.line), item.symbolId);
  }

  const edges = resolveEdgesFromResolvedLocations(allPendingEdges, locationToSymbolID);

  const storeResult = await sendApiRequest<StoreFileDeltaResponse>('index.storeFileDelta', {
    filePath,
    fileHash: scan.fileHash,
    symbols,
    changedCallerIds,
    removedCallerIds,
    edges,
  });

  if (!storeResult.success) {
    throw new Error('Failed to store file delta');
  }

  const stats: IndexStats = {
    symbols: symbols.length,
    edges: edges.length,
    durationMs: Date.now() - startedAt,
  };

  return storeResult.stats ?? stats;
}

export const __test__ = {
  canonicalPath,
  isPathWithinWorkspace,
  buildSymbolIntervalIndex,
  resolveSymbolIDByInterval,
};
