import type { Audit } from './common';

// LLM provider of a model in the LLM Models registry.
export enum ModelProvider {
  AZURE_OPENAI = 'AZURE_OPENAI',
  OPENAI = 'OPENAI',
  GEMINI = 'GEMINI',
  OPENROUTER = 'OPENROUTER',
  OLLAMA = 'OLLAMA',
  OPENAI_COMPATIBLE = 'OPENAI_COMPATIBLE'
}

// Where a model stands between ACE's record and the LiteLLM gateway.
export enum ModelSyncStatus {
  PENDING = 'PENDING',
  ACTIVE = 'ACTIVE',
  DELETING = 'DELETING',
  NEEDS_KEY = 'NEEDS_KEY'
}

// A model a project onboarded. Its alias is the name agents call it by.
export interface ModelConfig extends Audit {
  projectID: string;
  modelID: string;
  alias: string;
  label: string;
  provider: ModelProvider;
  // LiteLLM model string, e.g. azure/gpt-4o
  model: string;
  apiBase?: string | null;
  apiVersion?: string | null;
  // Masked key, e.g. ****1234; the key itself is never returned.
  apiKeyHint?: string | null;
  syncStatus: ModelSyncStatus;
  lastError?: string | null;
}

// How LiteLLM reaches a model. apiKey is write-only.
export interface ModelConnectionInput {
  provider: ModelProvider;
  model: string;
  apiBase?: string;
  apiVersion?: string;
  apiKey?: string;
}

export interface ModelTestResult {
  ok: boolean;
  latencyMs?: number | null;
  error?: string | null;
}

// The GraphQL selection for ModelConfig.
export const MODEL_CONFIG_SELECTION = `
  projectID
  modelID
  alias
  label
  provider
  model
  apiBase
  apiVersion
  apiKeyHint
  syncStatus
  lastError
  createdAt
  updatedAt
  createdBy {
    username
  }
  updatedBy {
    username
  }
`;
