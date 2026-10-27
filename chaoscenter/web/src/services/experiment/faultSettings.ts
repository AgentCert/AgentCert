import type { ChaosEngine } from '@models';
import type { ConfigValues } from './installStepSettings';

export const FAULT_SOURCE_ANNOTATION = 'agentcert.io/fault-source';

export interface FaultSource {
  hubID: string;
  category: string;
  name: string;
  revision: string;
}

export function readFaultSource(engine: ChaosEngine | undefined): FaultSource | undefined {
  const raw = engine?.metadata?.annotations?.[FAULT_SOURCE_ANNOTATION];
  if (!raw) return undefined;
  try {
    const source = JSON.parse(raw) as FaultSource;
    return source &&
      [source.hubID, source.category, source.name, source.revision].every(v => typeof v === 'string' && v)
      ? source
      : undefined;
  } catch {
    return undefined;
  }
}

export function stampFaultSource(engine: ChaosEngine, source: FaultSource): void {
  engine.metadata = {
    ...engine.metadata,
    annotations: { ...engine.metadata?.annotations, [FAULT_SOURCE_ANNOTATION]: JSON.stringify(source) }
  };
}

export function readFaultSettings(engine: ChaosEngine | undefined): ConfigValues {
  const values: ConfigValues = {};
  engine?.spec?.experiments?.[0]?.spec?.components?.env?.forEach(env => {
    if (env.value !== undefined && !env.valueFrom) values[env.name] = env.value;
  });
  return values;
}
