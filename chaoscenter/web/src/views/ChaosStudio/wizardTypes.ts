// Shared types for the ChaosStudio 4-screen wizard

export interface ExperimentStepDraft {
  id: string;
  name: string;
  type: 'observe' | 'fault' | 'verify' | 'wait' | 'parallel-fault';
  duration?: string;
  faultRef?: string;
  targetMicroservice?: string;
  params?: Record<string, string>;
  dependsOn?: string;
  probe?: { url: string; expectedStatus: number };
  parallelFaults?: Array<{
    faultRef: string;
    targetMicroservice: string;
    params?: Record<string, string>;
  }>;
}

export interface ChaosStudioWizardState {
  selectedAppName: string;
  selectedAppDomain: string;
  selectedAppMicroservices: string[];
  selectedAgentName: string;
  selectedAgentVersion: string;
  selectedAgentAllowUserChoice: boolean;
  selectedAgentAllowedModels: string[];
  steps: ExperimentStepDraft[];
  hypothesis: string;
}

export const INITIAL_WIZARD_STATE: Partial<ChaosStudioWizardState> = {
  steps: [],
  hypothesis: ''
};
