// Mirrors the GraphQL FaultCatalog types. The catalogue is composed server-side
// from the AppsHub (which applications exist) and the ChaosHub's fault
// capability catalogue (which faults are pinned to one of them), so onboarding
// an application, an agent, or a fault never requires a change here.
export interface TargetApplication {
  key: string;
  folders: string[];
  namespace: string;
  labelKey: string;
  services: string[];
}

export interface FaultCompatibility {
  faultName: string;
  classification: string;
  compatibleApps: string[];
  workloadKinds: string[];
  requiredServices: string[];
  knownFailingTargets: string[];
}

export interface FaultCatalog {
  applications: TargetApplication[];
  faults: FaultCompatibility[];
}
