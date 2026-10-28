import { ModelConfig, ModelProvider, ModelSyncStatus } from '@api/entities';
import {
  INITIAL_MODEL_FORM_VALUES,
  PLATFORM_STATUS,
  hasUnsettledModels,
  suggestAlias,
  toConnection,
  toModelRows
} from '../modelConfigUtils';

const config = (alias: string, status = ModelSyncStatus.ACTIVE): ModelConfig => ({
  projectID: 'p1',
  modelID: `id-${alias}`,
  alias,
  label: alias,
  provider: ModelProvider.AZURE_OPENAI,
  model: 'azure/gpt-4o',
  apiKeyHint: '****1234',
  syncStatus: status
});

describe('toConnection', () => {
  test('keeps the fields its provider uses, trimmed', () => {
    expect(
      toConnection({
        ...INITIAL_MODEL_FORM_VALUES,
        provider: ModelProvider.AZURE_OPENAI,
        model: ' gpt-4o ',
        apiBase: ' https://acme.openai.azure.com ',
        apiVersion: '2024-08-01-preview',
        apiKey: ' key '
      })
    ).toEqual({
      provider: ModelProvider.AZURE_OPENAI,
      model: 'gpt-4o',
      apiBase: 'https://acme.openai.azure.com',
      apiVersion: '2024-08-01-preview',
      apiKey: 'key'
    });
  });

  test('drops a version the provider ignores and empty optional fields', () => {
    expect(
      toConnection({ ...INITIAL_MODEL_FORM_VALUES, provider: ModelProvider.OLLAMA, model: 'qwen2.5:7b', apiVersion: 'x' })
    ).toEqual({ provider: ModelProvider.OLLAMA, model: 'qwen2.5:7b' });
  });
});

describe('suggestAlias', () => {
  test.each([
    ['qwen2.5:32b-instruct', 'qwen2.5-32b-instruct'],
    ['azure/GPT-4o', 'gpt-4o'],
    ['anthropic/claude-3.5-sonnet', 'claude-3.5-sonnet'],
    ['  My Model!! ', 'my-model'],
    ['', '']
  ])('%p -> %p', (model, alias) => {
    expect(suggestAlias(model)).toBe(alias);
  });
});

describe('toModelRows', () => {
  test('lists the project models, then the platform models not already listed', () => {
    const rows = toModelRows(
      [config('gpt-4o-bench'), config('lost-one', ModelSyncStatus.NEEDS_KEY)],
      [
        { alias: 'gemini-2.5-flash', label: 'gemini-2.5-flash (Gemini)', provider: 'Gemini', isDefault: true },
        // Studio's list also carries the project's active models.
        { alias: 'gpt-4o-bench', label: 'gpt-4o-bench (Azure OpenAI)', provider: 'Azure OpenAI', isDefault: false }
      ]
    );
    expect(rows.map(row => [row.alias, row.status, row.provider])).toEqual([
      ['gpt-4o-bench', ModelSyncStatus.ACTIVE, 'Azure OpenAI'],
      ['lost-one', ModelSyncStatus.NEEDS_KEY, 'Azure OpenAI'],
      ['gemini-2.5-flash', PLATFORM_STATUS, 'Gemini']
    ]);
    expect(rows[2].isDefault).toBe(true);
    expect(rows[2].config).toBeUndefined();
    expect(rows[0].label).toBeUndefined();
  });
});

test('hasUnsettledModels', () => {
  expect(hasUnsettledModels(undefined)).toBe(false);
  expect(hasUnsettledModels([config('a'), config('b', ModelSyncStatus.NEEDS_KEY)])).toBe(false);
  expect(hasUnsettledModels([config('a'), config('b', ModelSyncStatus.PENDING)])).toBe(true);
  expect(hasUnsettledModels([config('c', ModelSyncStatus.DELETING)])).toBe(true);
});
