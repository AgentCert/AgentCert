import { gql, useMutation, MutationHookOptions } from '@apollo/client';
import type { ModelConfig } from './listModelConfigs';

export const CREATE_MODEL_CONFIG = gql`
  mutation createModelConfig($projectID: ID!, $input: ModelConfigInput!) {
    createModelConfig(projectID: $projectID, input: $input) {
      config {
        alias
        provider
        model
        baseURL
        secretRef
        agentsUsing
        status
        lastTested
      }
      message
    }
  }
`;

export interface ModelConfigInput {
  alias: string;
  provider: string;
  model: string;
  baseURL?: string;
  apiKey: string;
}

export interface CreateModelConfigResponse {
  createModelConfig: {
    config: ModelConfig;
    message: string;
  };
}

export interface CreateModelConfigRequest {
  projectID: string;
  input: ModelConfigInput;
}

export function useCreateModelConfig(
  options?: MutationHookOptions<CreateModelConfigResponse, CreateModelConfigRequest>
) {
  return useMutation<CreateModelConfigResponse, CreateModelConfigRequest>(CREATE_MODEL_CONFIG, options);
}
