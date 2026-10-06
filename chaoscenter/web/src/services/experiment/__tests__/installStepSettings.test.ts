import { ConfigField, ConfigFieldType, ConfigValueKind } from '@api/entities';
import {
  decodeValuesJSON,
  encodeValuesJSON,
  readInstallStepSettings,
  readInstallStepArg,
  readValuesJSON,
  resolveWorkflowParameter,
  writeInstallStepSettings
} from '../installStepSettings';

const field = (key: string, type = ConfigFieldType.STRING, valueKind = ConfigValueKind.STRING): ConfigField => ({
  key,
  label: key,
  type,
  valueKind,
  defaultValue: '',
  required: false,
  advanced: false
});

const fields = [
  field('agent.config.SCAN_INTERVAL', ConfigFieldType.INTEGER),
  field('agent.config.MODEL_ALIAS', ConfigFieldType.MODEL),
  field('agent.config.GOAL', ConfigFieldType.TEXT)
];

const parameters = [
  { name: 'agentFolder', value: 'flash-agent' },
  { name: 'scanInterval', value: '45' },
  { name: 'openaiModel', value: 'gpt-4o' }
];

describe('resolveWorkflowParameter', () => {
  test('replaces a whole-value parameter reference', () => {
    expect(resolveWorkflowParameter('{{workflow.parameters.agentFolder}}', parameters)).toBe('flash-agent');
    expect(resolveWorkflowParameter('{{ workflow.parameters.agentFolder }}', parameters)).toBe('flash-agent');
  });

  test('leaves literals, unknown parameters and embedded references alone', () => {
    expect(resolveWorkflowParameter('flash-agent', parameters)).toBe('flash-agent');
    expect(resolveWorkflowParameter('{{workflow.parameters.missing}}', parameters)).toBe(
      '{{workflow.parameters.missing}}'
    );
    expect(resolveWorkflowParameter('x-{{workflow.parameters.agentFolder}}', parameters)).toBe(
      'x-{{workflow.parameters.agentFolder}}'
    );
  });
});

describe('values documents', () => {
  test('round-trip settings through the nested JSON document Helm reads', () => {
    const values = { 'agent.config.SCAN_INTERVAL': '90', 'agent.config.GOAL': 'a, b: c', replicas: 3, enabled: false };
    const document = encodeValuesJSON(values);
    expect(JSON.parse(document)).toEqual({
      agent: { config: { SCAN_INTERVAL: '90', GOAL: 'a, b: c' } },
      replicas: 3,
      enabled: false
    });
    expect(decodeValuesJSON(document)).toEqual(values);
  });

  test('encode the same settings identically whatever their order', () => {
    expect(encodeValuesJSON({ b: 1, 'a.y': 2, 'a.x': 3 })).toBe(encodeValuesJSON({ 'a.x': 3, b: 1, 'a.y': 2 }));
  });

  test('decode anything but a JSON object as no settings', () => {
    ['', 'not json', '[]', 'null', '"x"'].forEach(document => expect(decodeValuesJSON(document)).toEqual({}));
  });

  test('read the document from either flag form', () => {
    expect(readValuesJSON(['-values-json={"a":1}'])).toEqual({ a: 1 });
    expect(readValuesJSON(['--values-json', '{"a":1}'])).toEqual({ a: 1 });
    expect(readValuesJSON(['-folder=x'])).toEqual({});
  });

  test('reads the final flag value, as the Go installers do', () => {
    expect(readInstallStepArg(['-folder=old', '--folder', 'new'], 'folder')).toBe('new');
    expect(readValuesJSON(['-values-json={"a":1}', '--values-json={"a":2}'])).toEqual({ a: 2 });
  });

  test('encoding a special object key cannot change JavaScript prototypes', () => {
    expect(JSON.parse(encodeValuesJSON({ '__proto__.polluted': 'x' }))).toEqual({
      ['__proto__']: { polluted: 'x' }
    });
    expect(Object.prototype).not.toHaveProperty('polluted');
  });
});

describe('readInstallStepSettings', () => {
  test('prefers the settings document over a template --set', () => {
    const args = [
      '--set=agent.config.SCAN_INTERVAL={{workflow.parameters.scanInterval}}',
      '-values-json={"agent":{"config":{"SCAN_INTERVAL":"90"}}}'
    ];
    expect(readInstallStepSettings(fields, { args, parameters })).toEqual({ 'agent.config.SCAN_INTERVAL': '90' });
  });

  test("adopts a ChaosHub template's --set values for declared settings, in both forms", () => {
    const args = [
      '--set=agent.config.SCAN_INTERVAL={{workflow.parameters.scanInterval}}',
      '--set',
      'agent.config.GOAL=Find the fault',
      '--set=agent.config.OTHER=ignored'
    ];
    expect(readInstallStepSettings(fields, { args, parameters })).toEqual({
      'agent.config.SCAN_INTERVAL': '45',
      'agent.config.GOAL': 'Find the fault'
    });
  });

  test('skips run-time expressions and never adopts a template model', () => {
    const args = [
      '--set=agent.config.GOAL={{workflow.uid}}',
      '--set=agent.config.MODEL_ALIAS={{workflow.parameters.openaiModel}}'
    ];
    expect(readInstallStepSettings(fields, { args, parameters })).toEqual({});
  });

  test('reads a model from the settings document', () => {
    const args = ['-values-json={"agent":{"config":{"MODEL_ALIAS":"qwen2.5-7b"}}}'];
    expect(readInstallStepSettings(fields, { args, parameters })).toEqual({ 'agent.config.MODEL_ALIAS': 'qwen2.5-7b' });
  });
});

describe('writeInstallStepSettings', () => {
  const declared = fields.map(f => f.key);

  test('replaces the settings document and the template --set args it supersedes', () => {
    const args = [
      '-folder={{workflow.parameters.agentFolder}}',
      '--set=agent.config.SCAN_INTERVAL={{workflow.parameters.scanInterval}}',
      '--set',
      'agent.config.GOAL=old',
      '--set=agent.secret.OPENAI_API_KEY={{workflow.parameters.openaiApiKey}}',
      '-values-json={"agent":{"config":{"GOAL":"older"}}}'
    ];
    expect(writeInstallStepSettings(args, { 'agent.config.SCAN_INTERVAL': '90' }, declared)).toEqual([
      '-folder={{workflow.parameters.agentFolder}}',
      '--set=agent.secret.OPENAI_API_KEY={{workflow.parameters.openaiApiKey}}',
      '-values-json={"agent":{"config":{"SCAN_INTERVAL":"90"}}}'
    ]);
  });

  test('records an explicit reset to defaults even when no settings remain', () => {
    expect(writeInstallStepSettings(['-folder=x', '-values-json={"a":1}'], {}, declared)).toEqual([
      '-folder=x',
      '-values-json={}'
    ]);
  });

  test('reads and replaces the Go installer single-dash set flag', () => {
    const args = ['-set=agent.config.SCAN_INTERVAL=45'];
    expect(readInstallStepSettings(fields, { args, parameters })).toEqual({ 'agent.config.SCAN_INTERVAL': '45' });
    expect(writeInstallStepSettings(args, { 'agent.config.SCAN_INTERVAL': '90' }, declared)).toEqual([
      '-values-json={"agent":{"config":{"SCAN_INTERVAL":"90"}}}'
    ]);
  });
});
