import * as assert from 'assert';
import * as path from 'path';
import { __test__, ScannedSymbol } from '../features/indexing/lspBridge';

suite('LSP Bridge Resolver Tests', () => {
  test('resolves cross-file callee by interval containment', () => {
    const symbols: ScannedSymbol[] = [
      {
        id: 'caller-id',
        name: 'A.Caller()',
        kind: 'function',
        filePath: '/tmp/ws/a.go',
        uri: 'file:///tmp/ws/a.go',
        line: 5,
        endLine: 12,
        col: 0,
      },
      {
        id: 'callee-id',
        name: 'B.Target()',
        kind: 'function',
        filePath: '/tmp/ws/b.go',
        uri: 'file:///tmp/ws/b.go',
        line: 20,
        endLine: 48,
        col: 0,
      },
    ];

    const intervals = __test__.buildSymbolIntervalIndex(symbols);
    const resolved = __test__.resolveSymbolIDByInterval('/tmp/ws/b.go', 34, intervals);
    assert.strictEqual(resolved, 'callee-id');
  });

  test('prefers the narrowest interval when symbols overlap', () => {
    const symbols: ScannedSymbol[] = [
      {
        id: 'wide',
        name: 'Pkg.Wide()',
        kind: 'function',
        filePath: '/tmp/ws/c.go',
        uri: 'file:///tmp/ws/c.go',
        line: 10,
        endLine: 100,
        col: 0,
      },
      {
        id: 'narrow',
        name: 'Pkg.Narrow()',
        kind: 'function',
        filePath: '/tmp/ws/c.go',
        uri: 'file:///tmp/ws/c.go',
        line: 24,
        endLine: 30,
        col: 0,
      },
    ];

    const intervals = __test__.buildSymbolIntervalIndex(symbols);
    const resolved = __test__.resolveSymbolIDByInterval('/tmp/ws/c.go', 26, intervals);
    assert.strictEqual(resolved, 'narrow');
  });

  test('normalizes paths and enforces workspace-root boundary', () => {
    const root = path.join(path.sep, 'tmp', 'ws');
    const insidePath = path.join(path.sep, 'tmp', 'ws', '.', 'sub', '..', 'b.go');
    const outsidePrefixTrap = path.join(path.sep, 'tmp', 'ws-other', 'b.go');

    assert.strictEqual(__test__.isPathWithinWorkspace(insidePath, root), true);
    assert.strictEqual(__test__.isPathWithinWorkspace(outsidePrefixTrap, root), false);
    assert.strictEqual(
      __test__.canonicalPath(path.join(path.sep, 'tmp', 'ws', '.', 'a.go')),
      __test__.canonicalPath(path.join(path.sep, 'tmp', 'ws', 'a.go'))
    );
  });
});

