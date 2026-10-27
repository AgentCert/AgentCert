import { ChaosObjectStoreNameMap, Experiment } from '@db';
import { ConfigFieldType, ConfigValueKind } from '@api/entities';
import { FaultTunableInputType } from '@models';
import type { ChaosEngine, FaultData, KubernetesExperimentManifest, Workflow } from '@models';
import { fromFormValues, toFormValues } from '@components/ChartSettingsForm/chartSettingsFormUtils';
import { GetFaultTunablesOperation } from '../ExperimentYamlService';
import { KubernetesYamlService } from '../KubernetesYamlService';
import { readFaultSettings, readFaultSource, stampFaultSource } from '../faultSettings';

jest.mock('@utils', () => ({ yamlStringify: (value: unknown) => JSON.stringify(value) }));

const source = { hubID: 'hub', category: 'itbench', name: 'service-port', revision: 'a'.repeat(64) };
const engine = (): ChaosEngine =>
  ({
    kind: 'ChaosEngine',
    metadata: { annotations: { probeRef: '[]' } },
    spec: {
      chaosServiceAccount: 'litmus-admin',
      engineState: 'active',
      experiments: [
        {
          name: 'service-port',
          spec: {
            components: {
              env: [
                { name: 'BROKEN_TARGET_PORT', value: '9999' },
                { name: 'DEFAULT_HEALTH_CHECK', value: 'false' },
                { name: 'LIB_IMAGE', value: 'registry/helper:tag' },
                { name: 'CREDENTIAL', valueFrom: { secretKeyRef: { name: 'credentials', key: 'password' } } }
              ]
            }
          }
        }
      ]
    }
  } as ChaosEngine);

const field = {
  key: 'BROKEN_TARGET_PORT',
  label: 'Broken port',
  type: ConfigFieldType.INTEGER,
  valueKind: ConfigValueKind.STRING,
  defaultValue: '9999',
  required: true,
  min: 1,
  max: 65535,
  advanced: false
};

test.each(['Workflow', 'CronWorkflow'])('typed fault settings persist and reopen in a %s', async kind => {
  const service = new KubernetesYamlService();
  const original = engine();
  stampFaultSource(original, source);
  const workflow: Workflow = {
    kind: 'Workflow',
    metadata: { name: 'fault-settings' },
    spec: {
      entrypoint: 'main',
      templates: [
        { name: 'main', steps: [[{ name: 'inject', template: 'inject' }]] },
        {
          name: 'install-chaos-faults',
          inputs: {
            artifacts: [
              {
                name: 'inject',
                raw: {
                  data: JSON.stringify({
                    kind: 'ChaosExperiment',
                    spec: { definition: { image: 'agentcert/itbench-experiment:dev', env: [] } }
                  })
                }
              }
            ]
          }
        },
        {
          name: 'inject',
          metadata: { labels: { weight: '10' } },
          inputs: { artifacts: [{ name: 'inject', raw: { data: JSON.stringify(original) } }] }
        }
      ]
    }
  } as Workflow;
  const manifest = (
    kind === 'CronWorkflow'
      ? { ...workflow, kind, spec: { schedule: '0 * * * *', workflowSpec: workflow.spec } }
      : workflow
  ) as KubernetesExperimentManifest;
  await (await service.db).put(ChaosObjectStoreNameMap.EXPERIMENTS, { name: kind, manifest } as Experiment, kind);
  const data = service.getFaultData(manifest, 'inject');
  const form = toFormValues([field], readFaultSettings(data?.engineCR));
  form.setting0 = '4321';
  const chosen = fromFormValues([field], form);
  const updated = service.updateFaultTunablesInFaultData(data, {
    BROKEN_TARGET_PORT: { type: FaultTunableInputType.Text, value: chosen.BROKEN_TARGET_PORT }
  });
  if (!updated) throw new Error('fault missing');
  await service.updateExperimentManifestWithFaultData(kind, updated);
  const saved = await service.getExperiment(kind);
  const reopened = service.getFaultData(saved?.manifest as KubernetesExperimentManifest, 'inject');
  expect(readFaultSettings(reopened?.engineCR).BROKEN_TARGET_PORT).toBe('4321');
  expect(readFaultSource(reopened?.engineCR)).toEqual(source);
  expect(reopened?.engineCR?.spec?.experiments[0]?.spec.components?.env).toEqual([
    { name: 'BROKEN_TARGET_PORT', value: '4321' },
    { name: 'DEFAULT_HEALTH_CHECK', value: 'false' },
    { name: 'LIB_IMAGE', value: 'registry/helper:tag' },
    { name: 'CREDENTIAL', valueFrom: { secretKeyRef: { name: 'credentials', key: 'password' } } }
  ]);
});

test('reference keys and hidden variables survive a partial edit', () => {
  const service = new KubernetesYamlService();
  const data = { faultName: 'inject', engineCR: engine() } as FaultData;
  const tunables = service.getFaultTunables(data, GetFaultTunablesOperation.UpdatedEnvs);
  expect(tunables?.CREDENTIAL.valueFrom?.key).toBe('password');
  service.updateFaultTunablesInFaultData(data, {
    DEFAULT_HEALTH_CHECK: { type: FaultTunableInputType.Boolean, value: true }
  });
  expect(readFaultSettings(data.engineCR).DEFAULT_HEALTH_CHECK).toBe('true');
  expect(
    data.engineCR?.spec?.experiments[0].spec.components?.env?.find(e => e.name === 'CREDENTIAL')?.valueFrom
      ?.secretKeyRef?.key
  ).toBe('password');
});

test('creates an env list when the engine did not have one', () => {
  const service = new KubernetesYamlService();
  const data: FaultData = {
    faultName: 'inject',
    engineCR: { spec: { experiments: [{ name: 'service-port', spec: {} }] } } as ChaosEngine
  };
  service.updateFaultTunablesInFaultData(data, {
    BROKEN_TARGET_PORT: { type: FaultTunableInputType.Text, value: '4321' }
  });
  expect(readFaultSettings(data.engineCR)).toEqual({ BROKEN_TARGET_PORT: '4321' });
});

test('fault instances and invalid source annotations remain independent', () => {
  const first = engine();
  const second = engine();
  stampFaultSource(first, source);
  expect(readFaultSource(second)).toBeUndefined();
  first.metadata = { ...first.metadata, annotations: { 'agentcert.io/fault-source': '{invalid' } };
  expect(readFaultSource(first)).toBeUndefined();
});

test('empty optional numeric defaults remain empty instead of becoming zero', () => {
  const optional = { ...field, key: 'RAMP_TIME', defaultValue: '', required: false, min: 0 };
  expect(fromFormValues([optional], { setting0: '' })).toEqual({ RAMP_TIME: '' });
});
