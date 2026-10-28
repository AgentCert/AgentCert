import type { SelectOption } from '@harnessio/uicore';
import { ModelConfig, ModelConnectionInput, ModelProvider, ModelSyncStatus, ModelTestResult } from '@api/entities';
import type { AgentModelOption } from '@api/core';
import type { StringKeys } from '@strings';

// What the Add Model form asks for, per provider. The server applies the same
// rules (pkg/model_config/provider.go) and is the one that enforces them.
export interface ProviderRule {
  label: string;
  // Added by the server when the model is entered without it.
  modelPrefix: string;
  // What the provider calls the model field.
  modelLabel: StringKeys;
  modelPlaceholder: string;
  apiBasePlaceholder: string;
  apiBaseRequired: boolean;
  needsVersion: boolean;
  needsKey: boolean;
}

export const PROVIDER_RULES: Record<ModelProvider, ProviderRule> = {
  [ModelProvider.AZURE_OPENAI]: {
    label: 'Azure OpenAI',
    modelPrefix: 'azure/',
    modelLabel: 'modelDeploymentName',
    modelPlaceholder: 'gpt-4o',
    apiBasePlaceholder: 'https://my-resource.openai.azure.com',
    apiBaseRequired: true,
    needsVersion: true,
    needsKey: true
  },
  [ModelProvider.OPENAI]: {
    label: 'OpenAI',
    modelPrefix: 'openai/',
    modelLabel: 'modelName',
    modelPlaceholder: 'gpt-4o-mini',
    apiBasePlaceholder: 'https://api.openai.com/v1 (default)',
    apiBaseRequired: false,
    needsVersion: false,
    needsKey: true
  },
  [ModelProvider.GEMINI]: {
    label: 'Gemini',
    modelPrefix: 'gemini/',
    modelLabel: 'modelName',
    modelPlaceholder: 'gemini-2.5-flash',
    apiBasePlaceholder: 'https://generativelanguage.googleapis.com/v1beta (default)',
    apiBaseRequired: false,
    needsVersion: false,
    needsKey: true
  },
  [ModelProvider.OPENROUTER]: {
    label: 'OpenRouter',
    modelPrefix: 'openrouter/',
    modelLabel: 'modelName',
    modelPlaceholder: 'anthropic/claude-3.5-sonnet',
    apiBasePlaceholder: 'https://openrouter.ai/api/v1 (default)',
    apiBaseRequired: false,
    needsVersion: false,
    needsKey: true
  },
  [ModelProvider.OLLAMA]: {
    label: 'Ollama',
    modelPrefix: 'ollama_chat/',
    modelLabel: 'modelName',
    modelPlaceholder: 'qwen2.5:32b-instruct',
    apiBasePlaceholder: 'http://ollama.ace.svc.cluster.local:11434',
    apiBaseRequired: true,
    needsVersion: false,
    needsKey: false
  },
  [ModelProvider.OPENAI_COMPATIBLE]: {
    label: 'OpenAI-compatible',
    modelPrefix: 'openai/',
    modelLabel: 'modelName',
    modelPlaceholder: 'meta-llama/Llama-3.1-8B-Instruct',
    apiBasePlaceholder: 'http://my-vllm:8000/v1',
    apiBaseRequired: true,
    needsVersion: false,
    needsKey: false
  }
};

export const PROVIDER_OPTIONS: SelectOption[] = Object.entries(PROVIDER_RULES).map(([value, rule]) => ({
  label: rule.label,
  value
}));

export const ALIAS_PATTERN = /^[a-z0-9][a-z0-9._-]{1,62}$/;

export interface ModelFormValues {
  provider: ModelProvider;
  alias: string;
  label: string;
  model: string;
  apiBase: string;
  apiVersion: string;
  apiKey: string;
}

export const INITIAL_MODEL_FORM_VALUES: ModelFormValues = {
  provider: ModelProvider.AZURE_OPENAI,
  alias: '',
  label: '',
  model: '',
  apiBase: '',
  apiVersion: '',
  apiKey: ''
};

// The connection a form describes: trimmed, without the fields its provider
// ignores, and without empty optional ones.
export function toConnection(values: ModelFormValues): ModelConnectionInput {
  const rule = PROVIDER_RULES[values.provider];
  const connection: ModelConnectionInput = { provider: values.provider, model: values.model.trim() };
  const apiBase = values.apiBase.trim();
  const apiVersion = values.apiVersion.trim();
  const apiKey = values.apiKey.trim();
  if (apiBase) connection.apiBase = apiBase;
  if (rule.needsVersion && apiVersion) connection.apiVersion = apiVersion;
  if (apiKey) connection.apiKey = apiKey;
  return connection;
}

// A connection test and the form values it was run on. Its result belongs to
// those values only: once one of them changes, the model is untested.
export interface ConnectionTest {
  connection: string;
  result: ModelTestResult;
}

// The identity of the connection a form describes, to compare with a test's.
export function connectionKey(values: ModelFormValues): string {
  return JSON.stringify(toConnection(values));
}

// An alias proposed from a model name, e.g. "qwen2.5:32b-instruct" ->
// "qwen2.5-32b-instruct" (the platform derives its Ollama aliases the same way).
export function suggestAlias(model: string): string {
  return model
    .trim()
    .toLowerCase()
    .replace(/^[a-z_]+\//, '')
    .replace(/[^a-z0-9._-]+/g, '-')
    .replace(/-{2,}/g, '-')
    .replace(/^[^a-z0-9]+/, '')
    .slice(0, 63)
    .replace(/[-._]+$/, '');
}

export const PLATFORM_STATUS = 'PLATFORM';

// One row of the LLM Models table: a project model, or a platform model,
// which every project can use and nobody edits here.
export interface ModelRow {
  alias: string;
  // Shown under the alias when it adds something.
  label?: string;
  provider: string;
  model?: string;
  status: ModelSyncStatus | typeof PLATFORM_STATUS;
  keyHint?: string | null;
  lastError?: string | null;
  isDefault: boolean;
  config?: ModelConfig;
}

// The project's models, then the platform models Studio also offers. Studio's
// list includes the project's active models, so those are not repeated.
export function toModelRows(configs: ModelConfig[], options: AgentModelOption[]): ModelRow[] {
  const projectAliases = new Set(configs.map(config => config.alias));
  const defaultAlias = options.find(option => option.isDefault)?.alias;
  const projectRows: ModelRow[] = configs.map(config => ({
    alias: config.alias,
    label: config.label !== config.alias ? config.label : undefined,
    provider: PROVIDER_RULES[config.provider]?.label ?? config.provider,
    model: config.model,
    status: config.syncStatus,
    keyHint: config.apiKeyHint,
    lastError: config.lastError,
    isDefault: config.alias === defaultAlias,
    config
  }));
  const platformRows: ModelRow[] = options
    .filter(option => !projectAliases.has(option.alias))
    .map(option => ({
      alias: option.alias,
      provider: option.provider,
      status: PLATFORM_STATUS,
      isDefault: option.isDefault
    }));
  return [...projectRows, ...platformRows];
}

// Registering and deleting settle on their own; the page refreshes until then.
export function hasUnsettledModels(configs: ModelConfig[] | undefined): boolean {
  return (configs ?? []).some(
    config => config.syncStatus === ModelSyncStatus.PENDING || config.syncStatus === ModelSyncStatus.DELETING
  );
}
