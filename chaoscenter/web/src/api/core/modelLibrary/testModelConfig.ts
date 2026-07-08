import { gql, useMutation, MutationHookOptions } from '@apollo/client';
import type { ModelConfigInput } from './createModelConfig';

export const TEST_MODEL_CONFIG = gql`
  mutation testModelConfig($input: ModelConfigInput!) {
    testModelConfig(input: $input) {
      success
      latencyMs
      errorMessage
    }
  }
`;

export interface ModelConfigTestResult {
  success: boolean;
  latencyMs?: number;
  errorMessage?: string;
}

export interface TestModelConfigResponse {
  testModelConfig: ModelConfigTestResult;
}

export interface TestModelConfigRequest {
  input: ModelConfigInput;
}

export function useTestModelConfig(
  options?: MutationHookOptions<TestModelConfigResponse, TestModelConfigRequest>
) {
  return useMutation<TestModelConfigResponse, TestModelConfigRequest>(TEST_MODEL_CONFIG, options);
}
