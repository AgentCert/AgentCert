import { gql, useQuery } from '@apollo/client';
import type { FaultStudioStatsResponse } from '@api/entities';
import type { GqlAPIQueryRequest, GqlAPIQueryResponse } from '@api/types';

export interface GetFaultStudioStatsRequest {
  projectID: string;
}

export interface GetFaultStudioStatsQueryResponse {
  getFaultStudioStats: FaultStudioStatsResponse;
}

export function getFaultStudioStats({
  projectID,
  options = {}
}: GqlAPIQueryRequest<
  GetFaultStudioStatsQueryResponse,
  GetFaultStudioStatsRequest,
  Record<string, never>
>): GqlAPIQueryResponse<GetFaultStudioStatsQueryResponse, GetFaultStudioStatsRequest> {
  const { data, loading, ...rest } = useQuery<GetFaultStudioStatsQueryResponse, GetFaultStudioStatsRequest>(
    gql`
      query getFaultStudioStats($projectID: ID!) {
        getFaultStudioStats(projectID: $projectID) {
          totalFaultStudios
          activeFaultStudios
          totalFaultsConfigured
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
