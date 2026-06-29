import { ProbeType, ProbeStatus, FaultProbeStatus, InfrastructureType } from '@api/entities';
import {
  getProbeDetails,
  getProbeDetailsFromParsedProbe,
  getProbeProperties,
  getProbePropertiesFromParsedProbe,
  getNormalizedProbeName,
  getIcon,
  getInfraIcon,
  getProbeStatusIcon,
  calculateTotalProbeStatusFromChaosData
} from '../probes';

describe('getNormalizedProbeName', () => {
  test.each([
    [ProbeType.HTTP, 'HTTP'],
    [ProbeType.CMD, 'Command'],
    [ProbeType.PROM, 'Prometheus'],
    [ProbeType.K8S, 'Kubernetes']
  ])('%s -> %s', (type, expected) => {
    expect(getNormalizedProbeName(type)).toBe(expected);
  });
});

describe('getIcon', () => {
  test('HTTP -> http-probe', () => {
    expect(getIcon(ProbeType.HTTP)).toBe('http-probe');
  });
  test('CMD -> cmd-probe', () => {
    expect(getIcon(ProbeType.CMD)).toBe('cmd-probe');
  });
  test('default -> custom-approval', () => {
    expect(getIcon(ProbeType.PROM)).toBe('custom-approval');
  });
});

describe('getInfraIcon', () => {
  test('KUBERNETES -> custom-approval', () => {
    expect(getInfraIcon(InfrastructureType.KUBERNETES)).toBe('custom-approval');
  });
});

describe('getProbeStatusIcon', () => {
  test.each([
    [ProbeStatus.Completed, 'check'],
    [ProbeStatus.Running, 'dry-run'],
    [ProbeStatus.Error, 'error'],
    [ProbeStatus.Queued, 'waiting'],
    [ProbeStatus.NA, 'codebase-invalid']
  ])('%s -> %s', (status, icon) => {
    expect(getProbeStatusIcon(status)).toBe(icon);
  });
});

describe('getProbeDetails', () => {
  test('extracts httpProbe inputs flattened, "-" for empty', () => {
    const out = getProbeDetails({
      'httpProbe/inputs': { url: 'http://x', insecureSkipVerify: false, method: { get: { criteria: '' } } }
    } as any);
    const map = Object.fromEntries(out);
    expect(map.url).toBe('http://x');
    expect(map.insecureSkipVerify).toBe('false');
  });
});

describe('getProbeDetailsFromParsedProbe', () => {
  test('k8s probe inputs', () => {
    const out = getProbeDetailsFromParsedProbe({
      type: ProbeType.K8S,
      k8sProperties: { version: 'v1', resource: 'pods', operation: 'present' }
    } as any);
    const map = Object.fromEntries(out);
    expect(map.version).toBe('v1');
    expect(map.resource).toBe('pods');
  });

  test('prom probe inputs', () => {
    const out = getProbeDetailsFromParsedProbe({
      type: ProbeType.PROM,
      promProperties: { endpoint: 'http://prom', query: 'up' }
    } as any);
    const map = Object.fromEntries(out);
    expect(map.endpoint).toBe('http://prom');
    expect(map.query).toBe('up');
  });
});

describe('getProbeProperties', () => {
  test('flattens runProperties', () => {
    const out = getProbeProperties({ runProperties: { probeTimeout: '10s', interval: '1s' } } as any);
    const map = Object.fromEntries(out);
    expect(map.probeTimeout).toBe('10s');
    expect(map.interval).toBe('1s');
  });
});

describe('getProbePropertiesFromParsedProbe', () => {
  test('only allow-listed property keys are returned for PROM', () => {
    const out = getProbePropertiesFromParsedProbe({
      type: ProbeType.PROM,
      promProperties: { probeTimeout: '5s', interval: '1s', endpoint: 'http://x', query: 'up' }
    } as any);
    const map = Object.fromEntries(out);
    expect(map.probeTimeout).toBe('5s');
    expect(map.interval).toBe('1s');
    // detail-only keys excluded
    expect(map.endpoint).toBeUndefined();
    expect(map.query).toBeUndefined();
  });
});

describe('calculateTotalProbeStatusFromChaosData', () => {
  test('returns zeros without chaosData', () => {
    expect(calculateTotalProbeStatusFromChaosData(undefined)).toEqual([0, 0, 0]);
  });

  test('counts passed/failed/na', () => {
    const chaosData: any = {
      chaosResult: {
        status: {
          probeStatuses: [
            { status: { verdict: FaultProbeStatus.PASSED } },
            { status: { verdict: FaultProbeStatus.PASSED } },
            { status: { verdict: FaultProbeStatus.FAILED } },
            { status: { verdict: FaultProbeStatus.AWAITED } }
          ]
        }
      }
    };
    expect(calculateTotalProbeStatusFromChaosData(chaosData)).toEqual([2, 1, 1]);
  });
});
