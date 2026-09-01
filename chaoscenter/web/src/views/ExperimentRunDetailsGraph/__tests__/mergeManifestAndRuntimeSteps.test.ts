import type { PipelineGraphState } from '@components/PipelineDiagram/types';
import { mergeManifestAndRuntimeSteps, normalizeStepKey, stepIdentity } from '../stepGraphMerge';

// Ordered steps as parsed from the experiment manifest (KubernetesYamlService
// .getFaultsFromExperimentManifest): identifier is the Argo template name,
// name may carry a builder display suffix ("install-agent: <folder>").
function manifestStep(name: string, display?: string): PipelineGraphState {
  return {
    id: name,
    identifier: name,
    name: display ?? name,
    type: 'ChaosNode',
    data: {}
  } as PipelineGraphState;
}

// Runtime node as produced by transformArgoData: id/identifier are the Argo
// node KEY ("<wf>-<hash>"), name is the short step name.
function runtimeNode(stepName: string, key: string): PipelineGraphState {
  return {
    id: key,
    identifier: key,
    name: stepName,
    type: 'ChaosNode',
    icon: 'chaos-scenario-builder',
    data: { name: stepName },
    status: 'Succeeded',
    children: []
  } as PipelineGraphState;
}

const ITBENCH_STEPS = [
  'install-application',
  'install-agent',
  'install-chaos-faults',
  'scaled-to-zero-kubernetes-workload-ttp',
  'uninstall-agent-2lj',
  'uninstall-application-drl',
  'dynamic-pre-cleanup-wait',
  'cleanup-chaos-resources',
  'uninstall-all'
];

describe('mergeManifestAndRuntimeSteps', () => {
  test('a fully-executed workflow renders each step exactly once (regression: no double)', () => {
    const steps = ITBENCH_STEPS.map(n =>
      manifestStep(
        n,
        n === 'install-agent'
          ? 'install-agent: sre-agent-comprehensive'
          : n === 'install-application'
          ? 'install-application: bookinfo'
          : n
      )
    );
    const graphData = ITBENCH_STEPS.map((n, i) => runtimeNode(n, `q-123-${1000 + i}`));

    const merged = mergeManifestAndRuntimeSteps(steps, graphData);

    expect(merged).toHaveLength(ITBENCH_STEPS.length);
    expect(merged.map(s => normalizeStepKey(s.name))).toEqual(ITBENCH_STEPS);
  });

  test('runtime status is folded onto the matching manifest step', () => {
    const steps = [manifestStep('install-application', 'install-application: bookinfo')];
    const graphData = [runtimeNode('install-application', 'q-123-1000')];

    const merged = mergeManifestAndRuntimeSteps(steps, graphData);

    expect(merged).toHaveLength(1);
    expect(merged[0].status).toBe('Succeeded');
    // node key from the runtime side is preserved so click-to-select can index
    // executionData.nodes[id]
    expect(merged[0].id).toBe('q-123-1000');
  });

  test('a partially-executed workflow keeps the full manifest step list, no duplicates', () => {
    const steps = ITBENCH_STEPS.map(n => manifestStep(n));
    const graphData = [runtimeNode('install-application', 'q-123-1000')];

    const merged = mergeManifestAndRuntimeSteps(steps, graphData);

    expect(merged).toHaveLength(ITBENCH_STEPS.length);
    expect(merged.filter(s => normalizeStepKey(s.name) === 'install-application')).toHaveLength(1);
  });

  test('a runtime node with no manifest counterpart is appended once', () => {
    const steps = [manifestStep('install-application')];
    const graphData = [runtimeNode('install-application', 'q-123-1000'), runtimeNode('surprise-fault', 'q-123-1001')];

    const merged = mergeManifestAndRuntimeSteps(steps, graphData);

    expect(merged.map(s => normalizeStepKey(s.name))).toEqual(['install-application', 'surprise-fault']);
  });

  test('empty runtime data yields the manifest steps unchanged', () => {
    const steps = ITBENCH_STEPS.map(n => manifestStep(n));
    expect(mergeManifestAndRuntimeSteps(steps, [])).toHaveLength(ITBENCH_STEPS.length);
  });
});

describe('stepIdentity / normalizeStepKey', () => {
  test('strips builder display suffix and argo wrapper', () => {
    expect(normalizeStepKey('install-agent: sre-agent-comprehensive')).toBe('install-agent');
    expect(normalizeStepKey('wf[0].install-agent(0)')).toBe('install-agent');
    expect(stepIdentity(manifestStep('scaled-to-zero-kubernetes-workload-ttp'))).toBe(
      'scaled-to-zero-kubernetes-workload-ttp'
    );
  });
});
