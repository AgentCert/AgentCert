import { gql, useQuery } from '@apollo/client';
import type { GqlAPIQueryRequest, GqlAPIQueryResponse } from '@api/types';

export interface IsFaultStudioNameAvailableRequest {
  projectID: string;
  name: string;
  excludeStudioID?: string;
}

export interface IsFaultStudioNameAvailableResponse {
  isFaultStudioNameAvailable: boolean;
}

export function isFaultStudioNameAvailable({
  projectID,
  name,
  excludeStudioID,
  options = {}
}: GqlAPIQueryRequest<
  IsFaultStudioNameAvailableResponse,
  IsFaultStudioNameAvailableRequest,
  { name: string; excludeStudioID?: string }
>): GqlAPIQueryResponse<IsFaultStudioNameAvailableResponse, IsFaultStudioNameAvailableRequest> {
  const { data, loading, ...rest } = useQuery<IsFaultStudioNameAvailableResponse, IsFaultStudioNameAvailableRequest>(
    gql`
      query isFaultStudioNameAvailable($projectID: ID!, $name: String!, $excludeStudioID: ID) {
        isFaultStudioNameAvailable(projectID: $projectID, name: $name, excludeStudioID: $excludeStudioID)
      }
    `,
    {
      variables: {
        projectID,
        name,
        excludeStudioID
      },
      fetchPolicy: options.fetchPolicy ?? 'network-only',
      ...options
    }
  );

  return {
    data,
    loading,
    ...rest
  };
}
