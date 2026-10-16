import { gql, useQuery } from '@apollo/client';
import { AppHubCategory, CONFIG_FIELD_SELECTION } from '@api/entities';
import type { GqlAPIQueryRequest, GqlAPIQueryResponse } from '@api/types';

export interface ListAppHubCategoriesRequest {
  projectID: string;
}

export interface ListAppHubCategoriesResponse {
  listAppHubCategories: AppHubCategory[];
}

export function listAppHubCategories({
  projectID,
  options = {}
}: GqlAPIQueryRequest<
  ListAppHubCategoriesResponse,
  ListAppHubCategoriesRequest
>): GqlAPIQueryResponse<ListAppHubCategoriesResponse, ListAppHubCategoriesRequest> {
  const { data, loading, ...rest } = useQuery<ListAppHubCategoriesResponse, ListAppHubCategoriesRequest>(
    gql`
      query listAppHubCategories($projectID: ID!) {
        listAppHubCategories(projectID: $projectID) {
          displayName
          categoryDescription
          applications {
            name
            displayName
            description
            version
            namespace
            isDeployed
            runningServices
            helmReleaseName
            configurations {
              ${CONFIG_FIELD_SELECTION}
            }
            microservices {
              name
              description
              isRunning
              readyReplicas
              desiredReplicas
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
