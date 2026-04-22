export interface AgentHubEntry {
  name: string;
  displayName: string;
  description: string;
  version: string;
  capabilities: string[];
  isDeployed: boolean;
  deploymentStatus: string;
  agentID?: string;
  namespace?: string;
  helmReleaseName?: string;
}

export interface AgentHubCategory {
  displayName: string;
  categoryDescription: string;
  agents: AgentHubEntry[];
}

export interface AgentHubStatus {
  id: string;
  name: string;
  repoURL: string;
  repoBranch: string;
  isAvailable: boolean;
  totalAgents: number;
  deployedAgents: number;
  isDefault: boolean;
  lastSyncedAt: string;
}

export interface Microservice {
  name: string;
  description?: string;
  isRunning: boolean;
  readyReplicas: number;
  desiredReplicas: number;
}
