import { ConfigField, ConfigFieldType, ConfigValueKind } from '@api/entities';
import {
  buildSettingsSchema,
  fromFormValues,
  isSettingModified,
  PLATFORM_DEFAULT_MODEL,
  rangeHint,
  settingFieldName,
  toFormValues
} from '../chartSettingsFormUtils';

const field = (overrides: Partial<ConfigField> & Pick<ConfigField, 'key' | 'type' | 'valueKind'>): ConfigField => ({
  label: overrides.key,
  defaultValue: '',
  required: false,
  advanced: false,
  ...overrides
});

// One field of each shape the charts declare.
const fields: ConfigField[] = [
  field({
    key: 'agent.config.SCAN_INTERVAL',
    type: ConfigFieldType.INTEGER,
    valueKind: ConfigValueKind.STRING,
    defaultValue: '60',
    min: 1
  }),
  field({
    key: 'agent.config.MODEL_ALIAS',
    type: ConfigFieldType.MODEL,
    valueKind: ConfigValueKind.STRING,
    defaultValue: 'gpt-4o'
  }),
  field({
    key: 'replicas',
    type: ConfigFieldType.INTEGER,
    valueKind: ConfigValueKind.INTEGER,
    defaultValue: '1',
    min: 1,
    max: 5
  }),
  field({
    key: 'loadGenerator.enabled',
    type: ConfigFieldType.BOOLEAN,
    valueKind: ConfigValueKind.BOOLEAN,
    defaultValue: 'true'
  }),
  field({
    key: 'agent.config.GOAL',
    type: ConfigFieldType.TEXT,
    valueKind: ConfigValueKind.STRING,
    defaultValue: 'Find it',
    required: true
  }),
  field({
    key: 'agent.config.LOG_LEVEL',
    type: ConfigFieldType.SELECT,
    valueKind: ConfigValueKind.STRING,
    defaultValue: 'INFO',
    options: ['DEBUG', 'INFO']
  }),
  field({
    key: 'memory',
    type: ConfigFieldType.STRING,
    valueKind: ConfigValueKind.STRING,
    defaultValue: '512Mi',
    pattern: '^[1-9][0-9]*(Mi|Gi)$'
  })
];

const formWith = (overrides: Record<number, string | number | boolean>): Record<string, string | number | boolean> => {
  const values = toFormValues(fields, {});
  Object.entries(overrides).forEach(([index, value]) => (values[settingFieldName(Number(index))] = value));
  return values;
};

const getString = ((key: string) => key) as never;

describe('toFormValues', () => {
  test('starts from the chart defaults, with the model on the platform default', () => {
    expect(toFormValues(fields, {})).toEqual({
      setting0: '60',
      setting1: PLATFORM_DEFAULT_MODEL,
      setting2: '1',
      setting3: true,
      setting4: 'Find it',
      setting5: 'INFO',
      setting6: '512Mi'
    });
  });

  test('uses what the step already sets', () => {
    const values = toFormValues(fields, {
      'agent.config.MODEL_ALIAS': 'qwen2.5-7b',
      replicas: 3,
      'loadGenerator.enabled': false
    });
    expect(values.setting1).toBe('qwen2.5-7b');
    expect(values.setting2).toBe(3);
    expect(values.setting3).toBe(false);
  });
});

describe('fromFormValues', () => {
  test('writes every setting typed like the chart default, but not a platform-default model', () => {
    expect(fromFormValues(fields, formWith({ 0: 90, 2: '3', 3: false }))).toEqual({
      'agent.config.SCAN_INTERVAL': '90',
      replicas: 3,
      'loadGenerator.enabled': false,
      'agent.config.GOAL': 'Find it',
      'agent.config.LOG_LEVEL': 'INFO',
      memory: '512Mi'
    });
  });

  test('writes a chosen model, pins numeric defaults and keeps empty optional text', () => {
    const optional = field({ key: 'note', type: ConfigFieldType.TEXT, valueKind: ConfigValueKind.STRING });
    const values = fromFormValues([...fields, optional], { ...formWith({ 1: 'qwen2.5-7b', 0: '' }), setting7: '' });
    expect(values['agent.config.MODEL_ALIAS']).toBe('qwen2.5-7b');
    expect(values['agent.config.SCAN_INTERVAL']).toBe('60');
    expect(values.note).toBe('');
  });

  test('normalises integer inputs before writing string environment variables', () => {
    expect(fromFormValues(fields, formWith({ 0: '1e2' }))['agent.config.SCAN_INTERVAL']).toBe('100');
  });
});

describe('buildSettingsSchema', () => {
  const schema = buildSettingsSchema(fields, getString);
  const errorsFor = (overrides: Record<number, string | number | boolean>): string[] => {
    try {
      schema.validateSync(formWith(overrides), { abortEarly: false });
      return [];
    } catch (error) {
      return (error as { errors: string[] }).errors;
    }
  };

  test('accepts the chart defaults', () => {
    expect(errorsFor({})).toEqual([]);
  });

  test('accepts an emptied optional number, which falls back to the default', () => {
    expect(errorsFor({ 0: '' })).toEqual([]);
  });

  test.each([
    ['below the minimum', { 0: 0 }, 'settingMustBeAtLeast'],
    ['above the maximum', { 2: 9 }, 'settingMustBeAtMost'],
    ['a fraction for a whole number', { 2: 2.5 }, 'settingMustBeAWholeNumber'],
    ['text for a number', { 0: 'soon' }, 'settingMustBeANumber'],
    ['an infinite number', { 0: 'Infinity' }, 'settingMustBeANumber'],
    ['an empty required setting', { 4: '   ' }, 'required'],
    ['an Argo expression', { 4: '{{workflow.uid}}' }, 'settingMustNotContainBraces'],
    ['an unknown option', { 5: 'TRACE' }, 'invalidSelection'],
    ['an empty option', { 5: '' }, 'invalidSelection'],
    ['an empty value with a pattern', { 6: '' }, 'settingMustMatch'],
    ['a value not matching the pattern', { 6: 'lots' }, 'settingMustMatch']
  ])('rejects %s', (_, overrides, message) => {
    expect(errorsFor(overrides)).toEqual([message]);
  });
});

describe('isSettingModified', () => {
  test('compares as text, because number inputs hand back numbers', () => {
    expect(isSettingModified(60, '60')).toBe(false);
    expect(isSettingModified('90', '60')).toBe(true);
    expect(isSettingModified(false, true)).toBe(true);
    expect(isSettingModified('', undefined)).toBe(false);
  });
});

describe('rangeHint', () => {
  test('describes whichever bounds a number setting has', () => {
    expect(rangeHint(fields[2])).toBe('1 – 5');
    expect(rangeHint(fields[0])).toBe('≥ 1');
    expect(rangeHint({ ...fields[0], min: null, max: 9 })).toBe('≤ 9');
    expect(rangeHint(fields[4])).toBeUndefined();
  });
});
