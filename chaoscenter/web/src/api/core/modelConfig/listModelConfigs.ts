import { gql, useQuery } from '@apollo/client';
import { MODEL_CONFIG_SELECTION, ModelConfig } from '@api/entities';
import type { GqlAPIQueryRequest, GqlAPIQueryResponse } from '@api/types';

export interface ListModelConfigsRequest {
  projectID: string;
}

export interface ListModelConfigsResponse {
  listModelConfigs: ModelConfig[];
}

export function listModelConfigs({
  projectID,
  options = {}
}: GqlAPIQueryRequest<ListModelConfigsResponse, ListModelConfigsRequest>): GqlAPIQueryResponse<
  ListModelConfigsResponse,
  ListModelConfigsRequest
> {
  const { data, loading, ...rest } = useQuery<ListModelConfigsResponse, ListModelConfigsRequest>(
    gql`
      query listModelConfigs($projectID: ID!) {
        listModelConfigs(projectID: $projectID) {
          ${MODEL_CONFIG_SELECTION}
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
