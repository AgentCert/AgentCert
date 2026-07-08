import { gql, useQuery, QueryHookOptions } from '@apollo/client';

export const LIST_MODEL_CONFIGS = gql`
  query listModelConfigs($projectID: ID!) {
    listModelConfigs(projectID: $projectID) {
      alias
      provider
      model
      baseURL
      secretRef
      agentsUsing
      status
      lastTested
    }
  }
`;

export interface ModelConfig {
  alias: string;
  provider: string;
  model: string;
  baseURL?: string;
  secretRef: string;
  agentsUsing: string[];
  status: string;
  lastTested?: string;
}

export interface ListModelConfigsResponse {
  listModelConfigs: ModelConfig[];
}

export interface ListModelConfigsRequest {
  projectID: string;
}

export function useListModelConfigs(
  options?: QueryHookOptions<ListModelConfigsResponse, ListModelConfigsRequest>
) {
  return useQuery<ListModelConfigsResponse, ListModelConfigsRequest>(LIST_MODEL_CONFIGS, options);
}
