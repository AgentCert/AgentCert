export interface AgentHubEntry {
  name: string;
  displayName: string;
  description: string;
  version: string;
  capabilities: string[];
  // Applications this agent may be paired with. `null`/undefined means the
  // agent declared no restriction and works with every application, including
  // ones onboarded after it; `[]` means it is deliberately not
  // application-targeted and is never offered for one.
  compatibleApplications?: string[] | null;
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
