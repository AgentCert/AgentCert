import { InfrastructureType } from '@api/entities';
import {
  getFormattedFileName,
  generateUpgradeInfrastructureName,
  generateUpgradeInfrastructureFileName
} from '../getFormattedFileName';
import { getInfrastructureTypeFromExperimentKind } from '../getInfrastructureTypeFromExperimentKind';
import { isValidNodeType } from '../nodes';
import { getHash } from '../getHash';
import { getColorBasedOnResilienceScore } from '../colors';

describe('getFormattedFileName', () => {
  test('lowercases and replaces non-word runs with single hyphen', () => {
    expect(getFormattedFileName('  Pod Delete!! Fault  ')).toBe('pod-delete-fault');
  });
});

describe('generateUpgradeInfrastructureName', () => {
  test('builds yml filename', () => {
    expect(generateUpgradeInfrastructureName({ infrastructureName: 'infra', latestVersion: '1.2.3' })).toBe(
      'infra-upgrade-v1.2.3.yml'
    );
  });
});

describe('generateUpgradeInfrastructureFileName', () => {
  test('wraps kubectl apply command', () => {
    expect(generateUpgradeInfrastructureFileName({ infrastructureName: 'infra', latestVersion: '1.0.0' })).toBe(
      'kubectl apply -f infra-upgrade-v1.0.0.yml'
    );
  });
});

describe('getInfrastructureTypeFromExperimentKind', () => {
  test('undefined manifest -> KUBERNETES', () => {
    expect(getInfrastructureTypeFromExperimentKind(undefined)).toBe(InfrastructureType.KUBERNETES);
  });
  test('Workflow -> KUBERNETES', () => {
    expect(getInfrastructureTypeFromExperimentKind({ kind: 'Workflow' } as any)).toBe(InfrastructureType.KUBERNETES);
  });
  test('CronWorkflow -> KUBERNETES', () => {
    expect(getInfrastructureTypeFromExperimentKind({ kind: 'CronWorkflow' } as any)).toBe(
      InfrastructureType.KUBERNETES
    );
  });
  test('unknown kind -> KUBERNETES (default)', () => {
    expect(getInfrastructureTypeFromExperimentKind({ kind: 'Something' } as any)).toBe(InfrastructureType.KUBERNETES);
  });
});

describe('isValidNodeType', () => {
  test('chaosengine is valid (case-insensitive)', () => {
    expect(isValidNodeType('ChaosEngine')).toBe(true);
  });
  test('other types invalid', () => {
    expect(isValidNodeType('pod')).toBe(false);
  });
});

describe('getHash', () => {
  test('no length -> uuid', () => {
    const h = getHash();
    expect(h).toMatch(/^[0-9a-f-]{36}$/);
  });
  test('short length uses date-based suffix of given length', () => {
    const h = getHash(6);
    expect(h).toHaveLength(6);
  });
  test('long length uses uuid slice', () => {
    const h = getHash(16);
    expect(h).toHaveLength(16);
    expect(h).not.toContain('-');
  });
  test('prefix is prepended', () => {
    expect(getHash(6, 'exp')).toMatch(/^exp-.{6}$/);
  });
});

describe('getColorBasedOnResilienceScore', () => {
  test('undefined -> grey palette', () => {
    const p = getColorBasedOnResilienceScore(undefined);
    expect(p.primary).toBeDefined();
    expect(p.background).toBeDefined();
  });
  test('100 -> green', () => {
    expect(getColorBasedOnResilienceScore(100).primary).toBeDefined();
  });
  test('between 0 and 100 -> orange', () => {
    const p = getColorBasedOnResilienceScore(55);
    expect(p.primary).toBeDefined();
  });
  test('0 or below -> red', () => {
    expect(getColorBasedOnResilienceScore(0).primary).toBeDefined();
  });
});
