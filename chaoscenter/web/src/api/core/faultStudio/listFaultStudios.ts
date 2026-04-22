import { gql, useQuery } from '@apollo/client';
import type { FaultStudioSummary, ListFaultStudioRequestInput } from '@api/entities';
import type { GqlAPIQueryRequest, GqlAPIQueryResponse } from '@api/types';

export interface ListFaultStudiosRequest {
  projectID: string;
  request?: ListFaultStudioRequestInput;
}

export interface ListFaultStudiosResponse {
  listFaultStudios: {
    faultStudios: FaultStudioSummary[];
    totalCount: number;
  };
}

export function listFaultStudios({
  projectID,
  request,
  options = {}
}: GqlAPIQueryRequest<
  ListFaultStudiosResponse,
  ListFaultStudiosRequest,
  { request?: ListFaultStudioRequestInput }
>): GqlAPIQueryResponse<ListFaultStudiosResponse, ListFaultStudiosRequest> {
  const { data, loading, ...rest } = useQuery<ListFaultStudiosResponse, ListFaultStudiosRequest>(
    gql`
      query listFaultStudios($projectID: ID!, $request: ListFaultStudioRequest) {
        listFaultStudios(projectID: $projectID, request: $request) {
          faultStudios {
            id
            name
            description
            projectId
            sourceHubId
            sourceHubName
            totalFaults
            enabledFaults
            isActive
            createdAt
            updatedAt
          }
          totalCount
        }
      }
    `,
    {
      variables: {
        projectID,
        request
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
