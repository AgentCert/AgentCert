import { gql, useMutation } from '@apollo/client';
import type { GqlAPIMutationRequest, GqlAPIMutationResponse } from '@api/types';

export interface DeleteModelConfigRequest {
  projectID: string;
  modelID: string;
}

export interface DeleteModelConfigResponse {
  deleteModelConfig: boolean;
}

export function deleteModelConfig(
  options?: GqlAPIMutationRequest<DeleteModelConfigResponse, DeleteModelConfigRequest>
): GqlAPIMutationResponse<DeleteModelConfigResponse, DeleteModelConfigRequest> {
  const [deleteModelConfigMutation, result] = useMutation<DeleteModelConfigResponse, DeleteModelConfigRequest>(
    gql`
      mutation deleteModelConfig($projectID: ID!, $modelID: ID!) {
        deleteModelConfig(projectID: $projectID, modelID: $modelID)
      }
    `,
    options
  );

  return [deleteModelConfigMutation, result];
}
