import { gql, useMutation } from '@apollo/client';
import type { ModelTestResult } from '@api/entities';
import type { GqlAPIMutationRequest, GqlAPIMutationResponse } from '@api/types';

export interface TestModelConfigRequest {
  projectID: string;
  modelID: string;
}

export interface TestModelConfigResponse {
  testModelConfig: ModelTestResult;
}

export function testModelConfig(
  options?: GqlAPIMutationRequest<TestModelConfigResponse, TestModelConfigRequest>
): GqlAPIMutationResponse<TestModelConfigResponse, TestModelConfigRequest> {
  const [testModelConfigMutation, result] = useMutation<TestModelConfigResponse, TestModelConfigRequest>(
    gql`
      mutation testModelConfig($projectID: ID!, $modelID: ID!) {
        testModelConfig(projectID: $projectID, modelID: $modelID) {
          ok
          latencyMs
          error
        }
      }
    `,
    options
  );

  return [testModelConfigMutation, result];
}
