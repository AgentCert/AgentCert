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
  categoryName: string;
  agents: AgentHubEntry[];
}

export interface AgentHubStatus {
  totalAgents: number;
  deployedAgents: number;
  categories: AgentHubCategory[];
}

export interface Microservice {
  name: string;
  description?: string;
  isRunning: boolean;
  readyReplicas: number;
  desiredReplicas: number;
}
