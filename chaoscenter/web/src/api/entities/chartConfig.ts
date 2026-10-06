// How the experiment builder renders a chart setting.
export enum ConfigFieldType {
  STRING = 'STRING',
  TEXT = 'TEXT',
  INTEGER = 'INTEGER',
  NUMBER = 'NUMBER',
  BOOLEAN = 'BOOLEAN',
  SELECT = 'SELECT',
  // An LLM model alias picked from listAgentModelOptions.
  MODEL = 'MODEL'
}

// The JSON type a setting's value is written as. It is the type of the chart's
// own default, so a setting stored as "60" (an env var) stays a string.
export enum ConfigValueKind {
  STRING = 'STRING',
  INTEGER = 'INTEGER',
  NUMBER = 'NUMBER',
  BOOLEAN = 'BOOLEAN'
}

// A setting an agent or application chart offers to the experiment builder,
// declared in the `configurations` block of the chart's values.yaml.
export interface ConfigField {
  // Helm values path, e.g. agent.config.SCAN_INTERVAL
  key: string;
  label: string;
  description?: string | null;
  type: ConfigFieldType;
  valueKind: ConfigValueKind;
  // The chart's default value, as text
  defaultValue: string;
  required: boolean;
  min?: number | null;
  max?: number | null;
  pattern?: string | null;
  options?: string[] | null;
  group?: string | null;
  advanced: boolean;
}

// The GraphQL selection for ConfigField, shared by the AgentHub and AppHub queries.
export const CONFIG_FIELD_SELECTION = `
  key
  label
  description
  type
  valueKind
  defaultValue
  required
  min
  max
  pattern
  options
  group
  advanced
`;
