import { gql, useQuery, useMutation } from '@apollo/client';
import type { QueryHookOptions, MutationHookOptions } from '@apollo/client';

// ── Types ─────────────────────────────────────────────────────────────────────

export type AceRunStatus = 'QUEUED' | 'RUNNING' | 'COMPLETED' | 'FAILED' | 'ABORTED';

export interface RunStatusEvent {
  status: AceRunStatus;
  timestamp: string;
  reason?: string;
}

export interface AceExperimentRun {
  runID: string;
  projectID: string;
  definitionName: string;
  definitionVersion: string;
  agentName: string;
  agentVersion: string;
  modelUsed: string;
  modelProvider: string;
  argoWorkflowName: string;
  langfuseTraceId?: string;
  certifierReportId?: string;
  status: AceRunStatus;
  statusHistory: RunStatusEvent[];
  startedAt?: string;
  completedAt?: string;
  createdAt: string;
  createdBy: string;
}

export interface AceExperimentDefinition {
  name: string;
  version: string;
  status: string;
}

// ── Input types ───────────────────────────────────────────────────────────────

export interface AceTargetAppInput {
  name: string;
  version: string;
}

export interface AgentConstraintsInput {
  requiredCapabilities?: string[];
  supportedAgents?: string[];
  blockedAgents?: string[];
}

export interface AceModelSelectionInput {
  mode: string;
  fixedModel?: string | null;
}

export interface AceExperimentStepInput {
  name: string;
  type: string;
  duration?: string;
  faultRef?: string;
  target?: { microservice: string };
  params?: Array<{ key: string; value: string }>;
  probe?: { url: string; expectedStatus: number };
}

export interface AceSuccessCriteriaInput {
  perStep?: Array<{
    stepName: string;
    detectWithinSecs: number;
    mitigateWithinSecs: number;
  }>;
}

export interface AceExperimentInput {
  name: string;
  displayName?: string;
  hypothesis?: string;
  tags?: string[];
  targetApp: AceTargetAppInput;
  agentConstraints?: AgentConstraintsInput;
  modelSelection: AceModelSelectionInput;
  steps: AceExperimentStepInput[];
  successCriteria?: AceSuccessCriteriaInput;
  evaluationMetrics?: string[];
}

export interface AceSecretInput {
  key: string;
  value: string;
}

export interface AceParamInput {
  stepName: string;
  key: string;
  value: string;
}

// ── Mutations ─────────────────────────────────────────────────────────────────

export const CREATE_EXPERIMENT_MUTATION = gql`
  mutation CreateExperiment($projectID: ID!, $input: AceExperimentInput!) {
    createExperiment(projectID: $projectID, input: $input) {
      name
      version
      status
    }
  }
`;

export const UPDATE_EXPERIMENT_MUTATION = gql`
  mutation UpdateExperiment($projectID: ID!, $name: String!, $input: AceExperimentInput!) {
    updateExperiment(projectID: $projectID, name: $name, input: $input) {
      name
      version
      status
    }
  }
`;

export const DELETE_EXPERIMENT_MUTATION = gql`
  mutation DeleteExperiment($projectID: ID!, $name: String!) {
    deleteExperiment(projectID: $projectID, name: $name)
  }
`;

export const SUBMIT_RUN_MUTATION = gql`
  mutation SubmitRun(
    $projectID: ID!
    $experimentName: String!
    $agentName: String!
    $modelOverride: String
    $secretOverrides: [AceSecretInput!]
    $paramOverrides: [AceParamInput!]
  ) {
    submitRun(
      projectID: $projectID
      experimentName: $experimentName
      agentName: $agentName
      modelOverride: $modelOverride
      secretOverrides: $secretOverrides
      paramOverrides: $paramOverrides
    ) {
      runID
      status
      argoWorkflowName
    }
  }
`;

export const ABORT_RUN_MUTATION = gql`
  mutation AbortRun($projectID: ID!, $runID: String!) {
    abortRun(projectID: $projectID, runID: $runID) {
      runID
      status
    }
  }
`;

// ── Queries ───────────────────────────────────────────────────────────────────

export const GET_ACE_EXPERIMENT_QUERY = gql`
  query GetAceExperiment($projectID: ID!, $name: String!) {
    getAceExperiment(projectID: $projectID, name: $name) {
      name
      version
      status
    }
  }
`;

export const LIST_ACE_EXPERIMENTS_QUERY = gql`
  query ListAceExperiments($projectID: ID!) {
    listAceExperiments(projectID: $projectID) {
      name
      version
      status
    }
  }
`;

export const GET_RUN_QUERY = gql`
  query GetRun($projectID: ID!, $runID: String!) {
    getRun(projectID: $projectID, runID: $runID) {
      runID
      projectID
      definitionName
      definitionVersion
      agentName
      agentVersion
      modelUsed
      modelProvider
      argoWorkflowName
      langfuseTraceId
      certifierReportId
      status
      statusHistory {
        status
        timestamp
        reason
      }
      startedAt
      completedAt
      createdAt
      createdBy
    }
  }
`;

export const LIST_RUNS_QUERY = gql`
  query ListRuns($projectID: ID!, $experimentName: String, $agentName: String, $status: AceRunStatus) {
    listRuns(
      projectID: $projectID
      experimentName: $experimentName
      agentName: $agentName
      status: $status
    ) {
      runID
      definitionName
      agentName
      modelUsed
      status
      startedAt
      completedAt
      createdAt
    }
  }
`;

// ── Hook wrappers ─────────────────────────────────────────────────────────────

export interface GetRunRequest {
  projectID: string;
  runID: string;
}

export interface GetRunResponse {
  getRun: AceExperimentRun | null;
}

export function useGetRun(options?: QueryHookOptions<GetRunResponse, GetRunRequest>) {
  return useQuery<GetRunResponse, GetRunRequest>(GET_RUN_QUERY, {
    fetchPolicy: 'network-only',
    ...options
  });
}

export interface CreateExperimentRequest {
  projectID: string;
  input: AceExperimentInput;
}

export interface CreateExperimentResponse {
  createExperiment: AceExperimentDefinition;
}

export function useCreateExperiment(
  options?: MutationHookOptions<CreateExperimentResponse, CreateExperimentRequest>
) {
  return useMutation<CreateExperimentResponse, CreateExperimentRequest>(
    CREATE_EXPERIMENT_MUTATION,
    options
  );
}

export interface SubmitRunRequest {
  projectID: string;
  experimentName: string;
  agentName: string;
  modelOverride?: string;
  secretOverrides?: AceSecretInput[];
  paramOverrides?: AceParamInput[];
}

export interface SubmitRunResponse {
  submitRun: Pick<AceExperimentRun, 'runID' | 'status' | 'argoWorkflowName'>;
}

export function useSubmitRun(
  options?: MutationHookOptions<SubmitRunResponse, SubmitRunRequest>
) {
  return useMutation<SubmitRunResponse, SubmitRunRequest>(SUBMIT_RUN_MUTATION, options);
}
