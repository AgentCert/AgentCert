import { gql, useMutation, useQuery } from '@apollo/client';
import type { GqlAPIMutationRequest, GqlAPIMutationResponse, GqlAPIQueryRequest, GqlAPIQueryResponse } from '@api/types';

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

export interface ListAgentModelOptionsRequest {
  projectID: string;
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

// The platform models plus the project's own, from the LLM Models registry.
export function listAgentModelOptions({
  projectID,
  options = {}
}: GqlAPIQueryRequest<ListAgentModelOptionsResponse, ListAgentModelOptionsRequest>): GqlAPIQueryResponse<
  ListAgentModelOptionsResponse,
  ListAgentModelOptionsRequest
> {
  const { data, loading, ...rest } = useQuery<ListAgentModelOptionsResponse, ListAgentModelOptionsRequest>(
    gql`
      query listAgentModelOptions($projectID: ID!) {
        listAgentModelOptions(projectID: $projectID) {
          alias
          label
          provider
          isDefault
        }
      }
    `,
    {
      variables: {
        projectID
      },
      // A model added under LLM Models shows up the next time this is read.
      fetchPolicy: options.fetchPolicy ?? 'cache-and-network',
      ...options
    }
  );

  return {
    data,
    loading,
    ...rest
  };
}
