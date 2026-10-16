import { cloneDeep, merge } from 'lodash-es';
import type { PipelineGraphState } from '@components/PipelineDiagram/types';

export function normalizeStepKey(rawName: string | undefined): string {
  const name = (rawName ?? '').trim();
  if (!name) return '';

  // Builder display labels decorate install steps as "install-agent: <folder>".
  const withoutDisplaySuffix = name.split(':')[0].trim();

  // Argo run node names can include wrapper prefixes/suffixes, e.g.
  // "wf[0].install-agent(0)". Keep only the semantic step name.
  const terminalStepMatch = withoutDisplaySuffix.match(/([a-z0-9-]+)(?:\(\d+\))?$/i);
  if (terminalStepMatch?.[1]) {
    return terminalStepMatch[1].toLowerCase();
  }

  return withoutDisplaySuffix.toLowerCase();
}

export function stepIdentity(step: PipelineGraphState): string {
  return normalizeStepKey(step.identifier || step.id || step.name);
}

/**
 * Overlay live Argo run status (`graphData`, from transformArgoData) onto the
 * ordered step list parsed from the experiment manifest (`steps`), returning one
 * node per step plus any runtime-only nodes that have no manifest counterpart.
 *
 * The matched-key set is captured pre-merge on purpose: merge({}, step, runtime)
 * lets the runtime node's `identifier` (an Argo node key such as "<wf>-<hash>")
 * clobber the manifest step's `identifier` (its template name). Rebuilding the
 * matched set from the merged objects therefore produces hash strings that never
 * equal normalizeStepKey(runtimeNode.name), so every runtime node slips through
 * the "runtime only" filter and the entire workflow is appended a second time
 * after the last step (observed on ITBench experiments: install/uninstall/
 * uninstall-all rendered twice).
 */
export function mergeManifestAndRuntimeSteps(
  steps: PipelineGraphState[],
  graphData: PipelineGraphState[]
): PipelineGraphState[] {
  const deepCopySteps = cloneDeep(steps);
  const deepCopyGraphData = cloneDeep(graphData);

  const runtimeByStepKey = new Map<string, PipelineGraphState>();
  deepCopyGraphData.forEach(step => {
    const key = normalizeStepKey(step.name);
    if (key) {
      runtimeByStepKey.set(key, step);
    }
  });

  const matchedRuntimeKeys = new Set<string>();
  const mergedFromManifest = deepCopySteps.map(step => {
    const key = stepIdentity(step);
    const runtime = runtimeByStepKey.get(key);
    if (runtime) matchedRuntimeKeys.add(key);
    return runtime ? merge({}, step, runtime) : step;
  });

  const runtimeOnlySteps = deepCopyGraphData.filter(step => !matchedRuntimeKeys.has(normalizeStepKey(step.name)));
  return [...mergedFromManifest, ...runtimeOnlySteps];
}
