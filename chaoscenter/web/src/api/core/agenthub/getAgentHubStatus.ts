import { gql, useQuery } from '@apollo/client';
import type { AgentHubStatus } from '@api/entities';
import type { GqlAPIQueryRequest, GqlAPIQueryResponse } from '@api/types';

export interface GetAgentHubStatusRequest {
  projectID: string;
}

export interface GetAgentHubStatusResponse {
  getAgentHubStatus: AgentHubStatus;
}

export function getAgentHubStatus({
  projectID,
  options = {}
}: GqlAPIQueryRequest<
  GetAgentHubStatusResponse,
  GetAgentHubStatusRequest
>): GqlAPIQueryResponse<GetAgentHubStatusResponse, GetAgentHubStatusRequest> {
  const { data, loading, ...rest } = useQuery<GetAgentHubStatusResponse, GetAgentHubStatusRequest>(
    gql`
      query getAgentHubStatus($projectID: ID!) {
        getAgentHubStatus(projectID: $projectID) {
          totalAgents
          deployedAgents
          categories {
            categoryName
            agents {
              name
              displayName
              description
              version
              capabilities
              isDeployed
              deploymentStatus
              agentID
              namespace
              helmReleaseName
            }
          }
        }
      }
    `,
    {
      variables: {
        projectID
      },
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
