export type ContributeMethod = 'quick' | 'full' | 'private';
export type InstallMethod = 'external-helm' | 'helm' | 'manifests';
export type LoadTestMethod = 'built-in' | 'standard' | 'custom-job' | 'skip';

export interface DiscoveredService {
  name: string;
  label: string;
  kind: 'deployment' | 'statefulset' | 'daemonset';
  included: boolean;
  criticality: 'high' | 'medium' | 'low';
  autoExcluded: boolean;
  autoExclusionReason?: string;
}

export interface ContributionFormData {
  name: string;
  displayName: string;
  domain: string;
  shortDescription: string;
  longDescription: string;
  maintainerName: string;
  maintainerEmail: string;
  tags: string[];
  contributeMethod: ContributeMethod;
  installMethod: InstallMethod;
  chartRepoURL: string;
  chartName: string;
  chartVersion: string;
  gitURL: string;
  defaultNamespace: string;
  installTimeout: string;
  discoveredServices: DiscoveredService[];
  healthProbeURL: string;
  healthProbeStatus: string;
  initialDelaySeconds: number;
  periodSeconds: number;
  failureThreshold: number;
  loadTestMethod: LoadTestMethod;
  customJobYAML: string;
  generatedAppYAML: string;
  generatedReadmeMD: string;
}

export const EMPTY_FORM_DATA: ContributionFormData = {
  name: '',
  displayName: '',
  domain: '',
  shortDescription: '',
  longDescription: '',
  maintainerName: '',
  maintainerEmail: '',
  tags: [],
  contributeMethod: 'quick',
  installMethod: 'external-helm',
  chartRepoURL: '',
  chartName: '',
  chartVersion: '',
  gitURL: '',
  defaultNamespace: '',
  installTimeout: '30m',
  discoveredServices: [],
  healthProbeURL: 'http://{{.AppNamespace}}.svc.cluster.local:80/health',
  healthProbeStatus: '200',
  initialDelaySeconds: 30,
  periodSeconds: 10,
  failureThreshold: 6,
  loadTestMethod: 'standard',
  customJobYAML: '',
  generatedAppYAML: '',
  generatedReadmeMD: '',
};

export const DOMAINS = [
  { id: 'cloud-native', displayName: 'Cloud Native' },
  { id: 'service-mesh', displayName: 'Service Mesh' },
  { id: 'telecom', displayName: 'Telecom' },
  { id: 'health-it', displayName: 'Health IT' },
  { id: 'itops', displayName: 'IT Operations' },
  { id: 'finops', displayName: 'FinOps / Financial' },
];

export const AUTO_EXCLUDE_NAMES = ['prometheus', 'grafana', 'alertmanager', 'loki', 'jaeger', 'tempo'];
export const HIGH_CRITICALITY_PATTERNS = [/-db$/, /-database$/, /-postgres$/, /-mysql$/, /-mongo$/];
