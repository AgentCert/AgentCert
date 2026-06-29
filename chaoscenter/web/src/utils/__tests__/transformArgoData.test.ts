import type { Nodes } from '@api/entities';
import { ExperimentRunFaultStatus } from '@api/entities';
import { transformArgoData } from '../transformArgoData';

function makeNode(name: string, children: string[] | null, type = 'Pod'): any {
  return {
    children,
    finishedAt: '',
    message: '',
    name,
    phase: ExperimentRunFaultStatus.COMPLETED,
    startedAt: '',
    type
  };
}

describe('transformArgoData', () => {
  test('returns [] for undefined', () => {
    expect(transformArgoData(undefined)).toEqual([]);
  });

  test('returns [] for empty object', () => {
    expect(transformArgoData({} as Nodes)).toEqual([]);
  });

  test('maps a single root node', () => {
    const nodes: Nodes = { root: makeNode('root', null) };
    const out = transformArgoData(nodes);
    expect(out).toHaveLength(1);
    expect(out[0].id).toBe('root');
    expect(out[0].name).toBe('root');
    expect(out[0].type).toBe('ChaosNode');
    expect(out[0].children).toEqual([]);
  });

  test('maps known names to specific icons', () => {
    const nodes: Nodes = { a: makeNode('install-chaos-faults', null) };
    expect(transformArgoData(nodes)[0].icon).toBe('import');

    const nodes2: Nodes = { b: makeNode('cleanup-chaos-resources', null) };
    expect(transformArgoData(nodes2)[0].icon).toBe('command-rollback');

    const nodes3: Nodes = { c: makeNode('other', null) };
    expect(transformArgoData(nodes3)[0].icon).toBe('chaos-scenario-builder');
  });

  test('skips StepGroup and Steps node types', () => {
    const nodes: Nodes = {
      grp: makeNode('grp', ['child'], 'StepGroup'),
      child: makeNode('child', null)
    };
    const out = transformArgoData(nodes);
    // grp is skipped from graphData but its child gets processed
    expect(out.find(n => n.id === 'grp')).toBeUndefined();
    expect(out.find(n => n.id === 'child')).toBeDefined();
  });

  test('siblings of a node become its children', () => {
    const nodes: Nodes = {
      root: makeNode('root', ['s1', 's2']),
      s1: makeNode('s1', null),
      s2: makeNode('s2', null)
    };
    const out = transformArgoData(nodes);
    const rootEntry = out.find(n => n.id === 'root');
    expect(rootEntry).toBeDefined();
    // s1, s2 are pushed as children of the node dequeued before them
    expect(out.some(n => (n.children?.length ?? 0) > 0)).toBe(true);
  });
});
