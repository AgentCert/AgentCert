import { gql, useMutation } from '@apollo/client';
import { MODEL_CONFIG_SELECTION, ModelConfig, ModelConnectionInput } from '@api/entities';
import type { GqlAPIMutationRequest, GqlAPIMutationResponse } from '@api/types';

export interface CreateModelConfigRequest {
  projectID: string;
  request: {
    alias: string;
    label?: string;
    connection: ModelConnectionInput;
  };
}

export interface CreateModelConfigResponse {
  createModelConfig: ModelConfig;
}

export function createModelConfig(
  options?: GqlAPIMutationRequest<CreateModelConfigResponse, CreateModelConfigRequest>
): GqlAPIMutationResponse<CreateModelConfigResponse, CreateModelConfigRequest> {
  const [createModelConfigMutation, result] = useMutation<CreateModelConfigResponse, CreateModelConfigRequest>(
    gql`
      mutation createModelConfig($projectID: ID!, $request: CreateModelConfigInput!) {
        createModelConfig(projectID: $projectID, request: $request) {
          ${MODEL_CONFIG_SELECTION}
        }
      }
    `,
    options
  );

  return [createModelConfigMutation, result];
}
