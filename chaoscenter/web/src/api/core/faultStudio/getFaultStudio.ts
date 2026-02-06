import { gql, useQuery } from '@apollo/client';
import type { FaultStudio } from '@api/entities';
import type { GqlAPIQueryRequest, GqlAPIQueryResponse } from '@api/types';

export interface GetFaultStudioRequest {
  projectID: string;
  studioID: string;
}

export interface GetFaultStudioResponse {
  getFaultStudio: FaultStudio;
}

export function getFaultStudio({
  projectID,
  studioID,
  options = {}
}: GqlAPIQueryRequest<GetFaultStudioResponse, GetFaultStudioRequest, { studioID: string }>): GqlAPIQueryResponse<
  GetFaultStudioResponse,
  GetFaultStudioRequest
> {
  const { data, loading, ...rest } = useQuery<GetFaultStudioResponse, GetFaultStudioRequest>(
    gql`
      query getFaultStudio($projectID: ID!, $studioID: ID!) {
        getFaultStudio(projectID: $projectID, studioID: $studioID) {
          id
          name
          description
          tags
          projectId
          sourceHubId
          sourceHubName
          selectedFaults {
            faultCategory
            faultName
            displayName
            description
            enabled
            injectionConfig {
              injectionType
              schedule
              duration
              targetSelector
              interval
            }
            customParameters
            weight
          }
          isActive
          totalFaults
          enabledFaults
          isRemoved
          createdAt
          updatedAt
          createdBy {
            username
          }
          updatedBy {
            username
          }
        }
      }
    `,
    {
      variables: {
        projectID,
        studioID
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
