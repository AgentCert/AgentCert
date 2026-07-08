export interface AgentInputDefinition {
  key: string;
  displayName: string;
  type: 'secret' | 'string' | 'integer' | 'boolean' | 'enum';
  required: boolean;
  default?: string;
  description?: string;
  helmPath: string;
  values?: string[];
  advanced: boolean;
  group?: string;
}

export interface RequiredToolInput {
  name: string;
  purpose?: string;
  critical: boolean;
  minCallCount: number;
  maxCallCount?: number;
}

export interface WizardState {
  // Step 1 — Identity
  agentName: string;
  displayName: string;
  shortDescription: string;
  fullDescription: string;
  approach: string;
  ownerName: string;
  ownerEmail: string;
  ownerOrg: string;
  repositoryURL: string;
  tags: string[];

  // Step 2 — Docker Image
  dockerImage: string;
  cpuRequest: string;
  memoryRequest: string;
  cpuLimit: string;
  memoryLimit: string;

  // Step 3 — LLM Config
  llmDependent: boolean;
  selectedModelAlias: string;
  llmProvider: string;
  llmModel: string;
  llmApiKey: string;
  llmBaseURL: string;
  allowUserChoice: boolean;
  allowedModels: string[];
  defaultModel: string;

  // Step 4 — Config Inputs
  inputs: AgentInputDefinition[];

  // Step 5 — Capabilities & Tools
  capabilities: string[];
  requiredTools: RequiredToolInput[];
  evaluationMetrics: string[];

  // Step 6 — App Compatibility
  compatibilityMode: 'all' | 'specify';
  supportedApps: string[];
  unsupportedApps: string[];

  // Step 7 — Review
  tier: 'private' | 'community';
}

export const EVAL_METRICS = [
  { key: 'time_to_detect', label: 'Time to Detect', description: 'How quickly the agent detects the fault condition' },
  { key: 'time_to_mitigate', label: 'Time to Mitigate', description: 'How quickly the agent resolves the fault' },
  { key: 'tool_call_efficiency', label: 'Tool Call Efficiency', description: 'Ratio of useful tool calls to total tool calls' },
  { key: 'root_cause_accuracy', label: 'Root Cause Accuracy', description: 'Accuracy of root cause identification' },
  { key: 'remediation_correctness', label: 'Remediation Correctness', description: 'Correctness of the applied remediation' },
  { key: 'false_positive_rate', label: 'False Positive Rate', description: 'Rate of incorrect fault detections' },
  { key: 'blast_radius', label: 'Blast Radius', description: 'Scope of unintended side effects during remediation' },
];

export const APPROACH_OPTIONS = [
  { label: 'React Loop', value: 'react-loop' },
  { label: 'Plan and Execute', value: 'plan-and-execute' },
  { label: 'Chain of Thought', value: 'chain-of-thought' },
  { label: 'Rule Based', value: 'rule-based' },
  { label: 'Custom', value: 'custom' },
];

export const initialWizardState: WizardState = {
  agentName: '',
  displayName: '',
  shortDescription: '',
  fullDescription: '',
  approach: 'react-loop',
  ownerName: '',
  ownerEmail: '',
  ownerOrg: '',
  repositoryURL: '',
  tags: [],
  dockerImage: '',
  cpuRequest: '100m',
  memoryRequest: '128Mi',
  cpuLimit: '500m',
  memoryLimit: '512Mi',
  llmDependent: true,
  selectedModelAlias: '',
  llmProvider: 'openai',
  llmModel: 'gpt-4o',
  llmApiKey: '',
  llmBaseURL: '',
  allowUserChoice: false,
  allowedModels: [],
  defaultModel: '',
  inputs: [],
  capabilities: [],
  requiredTools: [],
  evaluationMetrics: [],
  compatibilityMode: 'all',
  supportedApps: [],
  unsupportedApps: [],
  tier: 'private',
};
