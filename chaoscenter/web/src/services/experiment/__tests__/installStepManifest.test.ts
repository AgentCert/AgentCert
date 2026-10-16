import { ChaosObjectStoreNameMap, Experiment } from '@db';
import type { KubernetesExperimentManifest, Workflow } from '@models';
import { KubernetesYamlService } from '../KubernetesYamlService';

// @utils re-exports the React test helpers, whose String component does not
// type-check under the TypeScript this package resolves to; the service only
// needs yamlStringify from it.
jest.mock('@utils', () => ({ yamlStringify: (value: unknown) => JSON.stringify(value) }));

// A ChaosHub template's install-agent step: the folder is a workflow
// parameter and the template passes its own --set values.
const templateManifest = (): KubernetesExperimentManifest =>
  ({
    kind: 'Workflow',
    apiVersion: 'argoproj.io/v1alpha1',
    metadata: { name: 'sock-shop' },
    spec: {
      entrypoint: 'main',
      arguments: {
        parameters: [
          { name: 'agentFolder', value: 'flash-agent' },
          { name: 'appNamespace', value: 'sock-shop' },
          { name: 'scanInterval', value: '60' }
        ]
      },
      templates: [
        { name: 'main', steps: [[{ name: 'install-agent', template: 'install-agent' }]] },
        {
          name: 'install-agent',
          container: {
            name: '',
            image: 'agentcert/agentcert-install-agent:latest',
            args: [
              '-folder={{workflow.parameters.agentFolder}}',
              '-namespace={{workflow.parameters.appNamespace}}',
              '-timeout={{workflow.parameters.installTimeout}}',
              '--set=agent.config.SCAN_INTERVAL={{workflow.parameters.scanInterval}}',
              '--set=agent.notifyId={{workflow.name}}'
            ]
          }
        }
      ]
    }
  } as unknown as KubernetesExperimentManifest);

const settings = {
  values: { 'agent.config.SCAN_INTERVAL': '90', 'agent.config.MODEL_ALIAS': 'qwen2.5-7b' },
  declaredKeys: ['agent.config.SCAN_INTERVAL', 'agent.config.MODEL_ALIAS']
};

async function setup(key: string, manifest = templateManifest()): Promise<KubernetesYamlService> {
  const service = new KubernetesYamlService();
  const experiment = { name: key, manifest } as unknown as Experiment;
  await (await service.db).put(ChaosObjectStoreNameMap.EXPERIMENTS, experiment, key);
  return service;
}

const agentArgs = (experiment: Experiment | undefined): string[] =>
  (experiment?.manifest as Workflow | undefined)?.spec.templates?.find(t => t.name === 'install-agent')?.container
    ?.args ?? [];

describe('install step settings in the manifest', () => {
  test('read a templated step back with its parameters resolved', async () => {
    const service = await setup('read-back');
    const experiment = await service.getExperiment('read-back');
    const selection = service.getInstallStepSelection(experiment?.manifest, 'agent');
    expect(selection?.folder).toBe('flash-agent');
    expect(selection?.namespace).toBe('sock-shop');
    expect(selection?.source?.args).toContain('--set=agent.config.SCAN_INTERVAL={{workflow.parameters.scanInterval}}');
  });

  test("re-applying the same chart keeps the template's args and records the settings", async () => {
    const service = await setup('same-chart');
    const experiment = await service.addInstallStepToManifest('same-chart', 'agent', {
      folder: 'flash-agent',
      namespace: 'sock-shop',
      settings
    });

    expect(agentArgs(experiment)).toEqual([
      '-folder={{workflow.parameters.agentFolder}}',
      '-namespace={{workflow.parameters.appNamespace}}',
      '-timeout={{workflow.parameters.installTimeout}}',
      '--set=agent.notifyId={{workflow.name}}',
      '-values-json={"agent":{"config":{"MODEL_ALIAS":"qwen2.5-7b","SCAN_INTERVAL":"90"}}}'
    ]);

    // The step reads the agentFolder parameter; it must not be pointed at itself.
    const parameters = (experiment?.manifest as Workflow | undefined)?.spec.arguments?.parameters;
    expect(parameters?.find(p => p.name === 'agentFolder')?.value).toBe('flash-agent');
  });

  test('a changed namespace replaces only the namespace arg', async () => {
    const service = await setup('new-namespace');
    const experiment = await service.addInstallStepToManifest('new-namespace', 'agent', {
      folder: 'flash-agent',
      namespace: 'agents',
      settings: { values: {}, declaredKeys: [] }
    });
    expect(agentArgs(experiment)).toContain('-namespace=agents');
    expect(agentArgs(experiment)).toContain('-timeout={{workflow.parameters.installTimeout}}');
  });

  test('choosing a different chart starts the step afresh', async () => {
    const service = await setup('other-chart');
    const experiment = await service.addInstallStepToManifest('other-chart', 'agent', {
      folder: 'sre-agent-crewai',
      namespace: 'sock-shop',
      settings: { values: { 'agent.config.GOAL': 'Find it' }, declaredKeys: ['agent.config.GOAL'] }
    });
    expect(agentArgs(experiment)).toEqual([
      '-folder=sre-agent-crewai',
      '-namespace=sock-shop',
      '-create-namespace',
      '-wait',
      '-values-json={"agent":{"config":{"GOAL":"Find it"}}}'
    ]);
  });

  test.each(['Workflow', 'CronWorkflow'])('updates split flags and preserves settings in a %s', async kind => {
    const workflow = templateManifest() as Workflow;
    const step = workflow.spec.templates?.find(t => t.name === 'install-agent');
    if (!step?.container) throw new Error('missing install step');
    step.container.args = ['--folder', 'flash-agent', '--namespace', 'sock-shop', '-timeout=30m'];
    const manifest = (
      kind === 'CronWorkflow'
        ? { ...workflow, kind, spec: { schedule: '0 * * * *', workflowSpec: workflow.spec } }
        : workflow
    ) as KubernetesExperimentManifest;
    const service = await setup(`split-${kind}`, manifest);
    const selection = service.getInstallStepSelection(manifest, 'agent');
    expect(selection?.folder).toBe('flash-agent');
    const experiment = await service.addInstallStepToManifest(`split-${kind}`, 'agent', {
      folder: 'flash-agent',
      namespace: 'agents',
      settings
    });
    const saved = JSON.parse(JSON.stringify(experiment?.manifest)) as KubernetesExperimentManifest;
    const updated = service.getInstallStepSelection(saved, 'agent');
    expect(updated?.namespace).toBe('agents');
    expect(updated?.source?.args).toContain('-timeout=30m');
    expect(updated?.source?.args).toContain(
      '-values-json={"agent":{"config":{"MODEL_ALIAS":"qwen2.5-7b","SCAN_INTERVAL":"90"}}}'
    );
    expect(updated?.source?.args).not.toContain('--namespace');
  });
});
