import { gql, useQuery } from '@apollo/client';
import type { AppHubStatus } from '@api/entities';
import type { GqlAPIQueryRequest, GqlAPIQueryResponse } from '@api/types';

export interface GetAppHubStatusRequest {
  projectID: string;
}

export interface GetAppHubStatusResponse {
  getAppHubStatus: AppHubStatus;
}

export function getAppHubStatus({
  projectID,
  options = {}
}: GqlAPIQueryRequest<
  GetAppHubStatusResponse,
  GetAppHubStatusRequest
>): GqlAPIQueryResponse<GetAppHubStatusResponse, GetAppHubStatusRequest> {
  const { data, loading, ...rest } = useQuery<GetAppHubStatusResponse, GetAppHubStatusRequest>(
    gql`
      query getAppHubStatus($projectID: ID!) {
        getAppHubStatus(projectID: $projectID) {
          totalApps
          deployedApps
          categories {
            categoryName
            applications {
              name
              displayName
              description
              version
              namespace
              isDeployed
              runningServices
              helmReleaseName
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
