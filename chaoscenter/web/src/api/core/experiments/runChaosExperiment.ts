import { gql, useMutation, useQuery } from '@apollo/client';
import type { GqlAPIMutationRequest, GqlAPIMutationResponse, GqlAPIQueryResponse } from '@api/types';

export interface RunChaosExperimentRequest {
  projectID: string;
  experimentID: string;
  modelAlias?: string;
}

export interface AgentModelOption {
  alias: string;
  label: string;
  provider: string;
  isDefault: boolean;
}

export interface ListAgentModelOptionsResponse {
  listAgentModelOptions: AgentModelOption[];
}

export interface RunChaosExperimentResponse {
  runChaosExperiment: {
    notifyID: string;
  };
}

export function runChaosExperiment(
  options?: GqlAPIMutationRequest<RunChaosExperimentResponse, RunChaosExperimentRequest>
): GqlAPIMutationResponse<RunChaosExperimentResponse, RunChaosExperimentRequest> {
  const [runChaosExperimentMutation, result] = useMutation<RunChaosExperimentResponse, RunChaosExperimentRequest>(
    gql`
      mutation runChaosExperiment($projectID: ID!, $experimentID: String!, $modelAlias: String) {
        runChaosExperiment(experimentID: $experimentID, projectID: $projectID, modelAlias: $modelAlias) {
          notifyID
        }
      }
    `,
    options
  );

  return [runChaosExperimentMutation, result];
}

export function listAgentModelOptions(): GqlAPIQueryResponse<ListAgentModelOptionsResponse, Record<string, never>> {
  const { data, loading, ...rest } = useQuery<ListAgentModelOptionsResponse>(
    gql`
      query listAgentModelOptions {
        listAgentModelOptions {
          alias
          label
          provider
          isDefault
        }
      }
    `,
    {
      fetchPolicy: 'cache-and-network'
    }
  );

  return {
    data,
    loading,
    ...rest
  };
}
