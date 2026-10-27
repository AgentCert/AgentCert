import React from 'react';
import { render, waitFor } from '@testing-library/react';
import type { ChaosEngine } from '@models';
import { useFaultCatalog } from '@hooks';
import { TargetApplicationTab } from '@views/ExperimentCreationFaultConfiguration/Tabs';
import Controller from '../TargetApplicationTab';

jest.mock('react-router-dom', () => ({ useParams: () => ({ experimentKey: 'test' }) }));
jest.mock('@api/core', () => ({
  kubeObjectSubscription: () => ({}),
  kubeNamespaceSubscription: () => ({})
}));
jest.mock('@hooks', () => ({ useFaultCatalog: jest.fn() }));
jest.mock('@services/experiment', () => {
  const handler = { getExperiment: async () => undefined };
  return { __esModule: true, default: { getInfrastructureTypeHandler: () => handler } };
});
jest.mock('@views/ExperimentCreationFaultConfiguration/Tabs', () => ({ TargetApplicationTab: jest.fn(() => null) }));

test('uses the canonical fault name for resource kinds and compatibility of a suffixed canvas instance', async () => {
  const canonical = 'modified-target-port-kubernetes-service';
  const catalog = {
    applications: [{ key: 'bookinfo', namespace: 'test' }],
    resolveApplication: jest.fn(() => undefined),
    isFaultCompatible: jest.fn((name: string) => name === canonical),
    faultWorkloadKinds: jest.fn((name: string) => (name === canonical ? ['service'] : undefined)),
    faultRequiredServices: jest.fn(() => undefined),
    faultKnownFailingTargets: jest.fn(() => [])
  };
  (useFaultCatalog as jest.Mock).mockReturnValue(catalog);
  render(
    <Controller
      faultName={`${canonical}-abc`}
      engineCR={{ spec: { experiments: [{ name: canonical }] } } as ChaosEngine}
      infrastructureID="test"
      setFaultData={jest.fn()}
    />
  );
  await waitFor(() => expect(catalog.faultWorkloadKinds).toHaveBeenCalledWith(canonical));
  expect(catalog.isFaultCompatible).toHaveBeenCalledWith(canonical, 'bookinfo');
  expect(catalog.faultRequiredServices).toHaveBeenCalledWith(canonical);
  expect(catalog.faultKnownFailingTargets).toHaveBeenCalledWith(canonical);
  const calls = (TargetApplicationTab as jest.Mock).mock.calls;
  expect(calls[calls.length - 1][0].allowedAppKinds).toEqual(['service']);
});
