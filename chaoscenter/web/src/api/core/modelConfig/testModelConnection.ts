import { gql, useMutation } from '@apollo/client';
import type { ModelConnectionInput, ModelTestResult } from '@api/entities';
import type { GqlAPIMutationRequest, GqlAPIMutationResponse } from '@api/types';

export interface TestModelConnectionRequest {
  projectID: string;
  request: ModelConnectionInput;
}

export interface TestModelConnectionResponse {
  testModelConnection: ModelTestResult;
}

// A mutation, not a query: it spends a call on the user's key.
export function testModelConnection(
  options?: GqlAPIMutationRequest<TestModelConnectionResponse, TestModelConnectionRequest>
): GqlAPIMutationResponse<TestModelConnectionResponse, TestModelConnectionRequest> {
  const [testModelConnectionMutation, result] = useMutation<TestModelConnectionResponse, TestModelConnectionRequest>(
    gql`
      mutation testModelConnection($projectID: ID!, $request: ModelConnectionInput!) {
        testModelConnection(projectID: $projectID, request: $request) {
          ok
          latencyMs
          error
        }
      }
    `,
    options
  );

  return [testModelConnectionMutation, result];
}
