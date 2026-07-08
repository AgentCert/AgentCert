import { gql, useMutation, MutationHookOptions } from '@apollo/client';

export const DELETE_MODEL_CONFIG = gql`
  mutation deleteModelConfig($projectID: ID!, $alias: String!) {
    deleteModelConfig(projectID: $projectID, alias: $alias)
  }
`;

export interface DeleteModelConfigRequest {
  projectID: string;
  alias: string;
}

export interface DeleteModelConfigResponse {
  deleteModelConfig: boolean;
}

export function useDeleteModelConfig(
  options?: MutationHookOptions<DeleteModelConfigResponse, DeleteModelConfigRequest>
) {
  return useMutation<DeleteModelConfigResponse, DeleteModelConfigRequest>(DELETE_MODEL_CONFIG, options);
}
