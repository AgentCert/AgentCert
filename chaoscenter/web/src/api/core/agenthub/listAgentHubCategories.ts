import { gql, useQuery } from '@apollo/client';
import type { AgentHubCategory } from '@api/entities';
import type { GqlAPIQueryRequest, GqlAPIQueryResponse } from '@api/types';

export interface ListAgentHubCategoriesRequest {
  projectID: string;
}

export interface ListAgentHubCategoriesResponse {
  listAgentHubCategories: AgentHubCategory[];
}

export function listAgentHubCategories({
  projectID,
  options = {}
}: GqlAPIQueryRequest<
  ListAgentHubCategoriesResponse,
  ListAgentHubCategoriesRequest
>): GqlAPIQueryResponse<ListAgentHubCategoriesResponse, ListAgentHubCategoriesRequest> {
  const { data, loading, ...rest } = useQuery<ListAgentHubCategoriesResponse, ListAgentHubCategoriesRequest>(
    gql`
      query listAgentHubCategories($projectID: ID!) {
        listAgentHubCategories(projectID: $projectID) {
          displayName
          categoryDescription
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
