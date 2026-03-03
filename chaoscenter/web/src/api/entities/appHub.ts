import type { Microservice } from './agentHub';

export interface AppHubEntry {
  name: string;
  displayName: string;
  description: string;
  version: string;
  namespace?: string;
  microservices: Microservice[];
  isDeployed: boolean;
  runningServices: number;
  helmReleaseName?: string;
}

export interface AppHubCategory {
  categoryName: string;
  applications: AppHubEntry[];
}

export interface AppHubStatus {
  totalApps: number;
  deployedApps: number;
  categories: AppHubCategory[];
}
