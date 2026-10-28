import { gql, useMutation } from '@apollo/client';
import { MODEL_CONFIG_SELECTION, ModelConfig } from '@api/entities';
import type { GqlAPIMutationRequest, GqlAPIMutationResponse } from '@api/types';

export interface UpdateModelConfigKeyRequest {
  projectID: string;
  modelID: string;
  apiKey: string;
}

export interface UpdateModelConfigKeyResponse {
  updateModelConfigKey: ModelConfig;
}

export function updateModelConfigKey(
  options?: GqlAPIMutationRequest<UpdateModelConfigKeyResponse, UpdateModelConfigKeyRequest>
): GqlAPIMutationResponse<UpdateModelConfigKeyResponse, UpdateModelConfigKeyRequest> {
  const [updateModelConfigKeyMutation, result] = useMutation<UpdateModelConfigKeyResponse, UpdateModelConfigKeyRequest>(
    gql`
      mutation updateModelConfigKey($projectID: ID!, $modelID: ID!, $apiKey: String!) {
        updateModelConfigKey(projectID: $projectID, modelID: $modelID, apiKey: $apiKey) {
          ${MODEL_CONFIG_SELECTION}
        }
      }
    `,
    options
  );

  return [updateModelConfigKeyMutation, result];
}
