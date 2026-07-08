export interface CatalogAppDescription {
  short: string;
  long: string;
  suitableFor: string[];
  notSuitableFor: string[];
}

export interface CatalogChartRef {
  repo: string;
  chart: string;
  version: string;
}

export interface CatalogNamespaceSpec {
  default: string;
  configurable: boolean;
}

export interface CatalogInstallSpec {
  method: string;
  folder?: string;
  chartRef?: CatalogChartRef;
  namespace: CatalogNamespaceSpec;
  timeout: string;
  wait: boolean;
}

export interface CatalogHealthProbeSpec {
  urlTemplate: string;
  expectedStatus: string;
  initialDelaySeconds: number;
  periodSeconds: number;
  failureThreshold: number;
}

export interface CatalogLoadTestSpec {
  enabled: boolean;
  method?: string;
  image?: string;
  args?: string[];
}

export interface CatalogMicroserviceSpec {
  name: string;
  displayName: string;
  description?: string;
  k8sLabel: string;
  k8sKind: string;
  k8sNamespace: string;
  criticality: string;
  relevantFaults: string[];
  dependsOn: string[];
}

export interface FaultCompatibilityEntry {
  faultName: string;
  compatible: boolean;
  notes?: string;
  recommendedTargets: string[];
}

export interface CatalogAppInput {
  key: string;
  displayName: string;
  description?: string;
  type: string;
  required: boolean;
  default?: string;
  helmPath: string;
  values?: string[];
  min?: number;
  max?: number;
  unit?: string;
  advanced: boolean;
}

export interface ApplicationSpec {
  name: string;
  displayName: string;
  version: string;
  tier: 'official' | 'community';
  domain: string;
  capabilityDomains: string[];
  tags: string[];
  description: CatalogAppDescription;
  install: CatalogInstallSpec;
  healthProbe: CatalogHealthProbeSpec;
  loadTest: CatalogLoadTestSpec;
  microservices: CatalogMicroserviceSpec[];
  faultCompatibility: FaultCompatibilityEntry[];
  inputs: CatalogAppInput[];
  schemaVersion: string;
}
