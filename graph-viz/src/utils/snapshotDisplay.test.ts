import { describe, expect, it } from 'vitest';
import { buildSnapshotDisplayData } from './snapshotDisplay';
import { GraphData } from '../types/graph';
import { SnapshotDetail } from '../types/api';

describe('buildSnapshotDisplayData', () => {
  it('resolves directory/file context nodes by path and remaps edge endpoints', () => {
    const freshData: GraphData = {
      nodes: [
        {
          id: 'm1',
          label: 'UserService.save(User)',
          fileId: 'fhash',
          type: 'method',
          filePath: 'src/service/UserService.java',
          line: 12,
          isVirtual: false,
        },
      ],
      edges: [
        {
          id: 'fhash::m1',
          source: 'fhash',
          target: 'm1',
          count: 2,
        },
      ],
      files: [
        {
          id: 'fhash',
          path: 'src/service/UserService.java',
          name: 'UserService.java',
          directory: 'dhash',
          isVirtual: false,
        },
      ],
      directories: [
        {
          id: 'dhash',
          path: 'src/service',
          name: 'service',
          isVirtual: false,
        },
      ],
    };

    const snapshot: SnapshotDetail = {
      id: 'snap_1',
      name: 'test',
      createdAt: 0,
      virtualNodes: [],
      contextNodes: [
        {
          id: 'dir:src/service',
          label: 'service',
          type: 'directory',
          filePath: 'src/service',
          isVirtual: false,
        },
        {
          id: 'file:src/service/UserService.java',
          label: 'UserService.java',
          type: 'file',
          filePath: 'src/service/UserService.java',
          line: 0,
          isVirtual: false,
        },
        {
          id: 'm1',
          label: 'UserService.save(User)',
          type: 'method',
          filePath: 'src/service/UserService.java',
          line: 12,
          isVirtual: false,
        },
      ],
      edges: [
        {
          id: 'e1',
          sourceId: 'file:src/service/UserService.java',
          targetId: 'm1',
        },
      ],
    };

    const result = buildSnapshotDisplayData(freshData, snapshot);

    expect(result.nodes.some(n => n.id === 'dhash' && !n.isStaleSnapshot)).toBe(true);
    expect(result.nodes.some(n => n.id === 'fhash' && !n.isStaleSnapshot)).toBe(true);
    expect(result.nodes.some(n => n.id === 'dir:src/service')).toBe(false);
    expect(result.nodes.some(n => n.id === 'file:src/service/UserService.java')).toBe(false);

    expect(result.edges).toHaveLength(1);
    expect(result.edges[0].source).toBe('fhash');
    expect(result.edges[0].target).toBe('m1');
    expect(result.edges[0].count).toBe(2);
    expect(result.edges[0].isStaleSnapshot).toBeUndefined();
  });

  it('resolves legacy file placeholders serialized as class/interface with line=0', () => {
    const freshData: GraphData = {
      nodes: [],
      edges: [],
      files: [
        {
          id: 'fhash',
          path: 'src/service/Legacy.java',
          name: 'Legacy.java',
          directory: 'dhash',
          isVirtual: false,
        },
      ],
      directories: [
        {
          id: 'dhash',
          path: 'src/service',
          name: 'service',
          isVirtual: false,
        },
      ],
    };

    const snapshot: SnapshotDetail = {
      id: 'snap_legacy',
      createdAt: 0,
      virtualNodes: [],
      contextNodes: [
        {
          id: 'legacy-file-id',
          label: 'Legacy.java',
          type: 'class',
          filePath: 'src/service/Legacy.java',
          line: 0,
          isVirtual: false,
        },
      ],
      edges: [],
    };

    const result = buildSnapshotDisplayData(freshData, snapshot);

    expect(result.nodes).toHaveLength(1);
    expect(result.nodes[0].id).toBe('fhash');
    expect(result.nodes[0].isStaleSnapshot).toBeUndefined();
  });

  it('keeps truly missing context nodes as stale placeholders', () => {
    const freshData: GraphData = {
      nodes: [],
      edges: [],
      files: [],
      directories: [],
    };

    const snapshot: SnapshotDetail = {
      id: 'snap_stale',
      createdAt: 0,
      virtualNodes: [],
      contextNodes: [
        {
          id: 'missing.method',
          label: 'Missing.method()',
          type: 'method',
          filePath: 'src/missing.java',
          line: 10,
          isVirtual: false,
          parentId: 'missing.file',
          description: 'from snapshot',
        },
      ],
      edges: [
        {
          id: 'e1',
          sourceId: 'missing.method',
          targetId: 'missing.method',
          remarks: 'edge note',
        },
      ],
    };

    const result = buildSnapshotDisplayData(freshData, snapshot);

    expect(result.nodes).toHaveLength(1);
    expect(result.nodes[0].id).toBe('missing.method');
    expect(result.nodes[0].isStaleSnapshot).toBe(true);
    expect(result.nodes[0].annotation?.description).toBe('from snapshot');
    expect(result.nodes[0].parentId).toBe('missing.file');

    expect(result.edges).toHaveLength(1);
    expect(result.edges[0].source).toBe('missing.method');
    expect(result.edges[0].target).toBe('missing.method');
    expect(result.edges[0].isStaleSnapshot).toBe(true);
    expect(result.edges[0].annotation?.remarks).toBe('edge note');
  });

  it('hydrates fresh edge annotation from snapshot remarks when edge exists but remark is missing', () => {
    const freshData: GraphData = {
      nodes: [
        {
          id: 'a',
          label: 'A.m()',
          fileId: 'f1',
          type: 'method',
          filePath: 'src/A.java',
          line: 1,
          isVirtual: false,
        },
        {
          id: 'b',
          label: 'B.n()',
          fileId: 'f2',
          type: 'method',
          filePath: 'src/B.java',
          line: 1,
          isVirtual: false,
        },
      ],
      edges: [
        {
          id: 'a::b',
          source: 'a',
          target: 'b',
          count: 1,
        },
      ],
      files: [],
      directories: [],
    };

    const snapshot: SnapshotDetail = {
      id: 'snap_edge_remarks',
      createdAt: 0,
      virtualNodes: [],
      contextNodes: [
        { id: 'a', label: 'A.m()', type: 'method', isVirtual: false },
        { id: 'b', label: 'B.n()', type: 'method', isVirtual: false },
      ],
      edges: [
        { id: 'e1', sourceId: 'a', targetId: 'b', remarks: 'preserved note' },
      ],
    };

    const result = buildSnapshotDisplayData(freshData, snapshot);
    expect(result.edges).toHaveLength(1);
    expect(result.edges[0].isStaleSnapshot).toBeUndefined();
    expect(result.edges[0].annotation?.remarks).toBe('preserved note');
  });
});
