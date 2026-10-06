import type { Microservice } from './agentHub';
import type { ConfigField } from './chartConfig';

export interface AppHubEntry {
  name: string;
  displayName: string;
  description: string;
  version: string;
  namespace?: string;
  microservices: Microservice[];
  // Settings the application's chart lets the user change in the experiment builder.
  configurations: ConfigField[];
  isDeployed: boolean;
  runningServices?: string;
  helmReleaseName?: string;
}

export interface AppHubCategory {
  displayName: string;
  categoryDescription: string;
  applications: AppHubEntry[];
}

export interface AppHubStatus {
  id: string;
  name: string;
  repoURL: string;
  repoBranch: string;
  isAvailable: boolean;
  totalApps: number;
  deployedApps: number;
  isDefault: boolean;
  lastSyncedAt: string;
}
