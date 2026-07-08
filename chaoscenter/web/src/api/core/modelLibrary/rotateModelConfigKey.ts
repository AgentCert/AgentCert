import { gql, useMutation, MutationHookOptions } from '@apollo/client';
import type { ModelConfig } from './listModelConfigs';

export const ROTATE_MODEL_CONFIG_KEY = gql`
  mutation rotateModelConfigKey($projectID: ID!, $alias: String!, $newApiKey: String!) {
    rotateModelConfigKey(projectID: $projectID, alias: $alias, newApiKey: $newApiKey) {
      alias
      provider
      model
      status
    }
  }
`;

export interface RotateModelConfigKeyRequest {
  projectID: string;
  alias: string;
  newApiKey: string;
}

export interface RotateModelConfigKeyResponse {
  rotateModelConfigKey: ModelConfig;
}

export function useRotateModelConfigKey(
  options?: MutationHookOptions<RotateModelConfigKeyResponse, RotateModelConfigKeyRequest>
) {
  return useMutation<RotateModelConfigKeyResponse, RotateModelConfigKeyRequest>(ROTATE_MODEL_CONFIG_KEY, options);
}
