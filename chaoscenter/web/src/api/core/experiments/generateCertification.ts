import { gql, useMutation } from '@apollo/client';
import type { GqlAPIMutationRequest, GqlAPIMutationResponse } from '../../types';

export interface CertificationGenerationRequest {
  agentID: string;
  agentName: string;
  experimentID: string;
  experimentRunID: string;
  expectedRuns?: number;
}

export interface CertificationGenerationResponse {
  generateCertification: {
    status: string;
    experimentRunWorkflowStatus: string;
    message?: string;
  };
}

export interface GenerateCertificationVariables {
  projectID: string;
  request: CertificationGenerationRequest;
}

export function generateCertification(
  options?: GqlAPIMutationRequest<CertificationGenerationResponse, GenerateCertificationVariables>
): GqlAPIMutationResponse<CertificationGenerationResponse, GenerateCertificationVariables> {
  return useMutation<CertificationGenerationResponse, GenerateCertificationVariables>(
    gql`
      mutation generateCertification($projectID: ID!, $request: CertificationGenerationRequest!) {
        generateCertification(projectID: $projectID, request: $request) {
          status
          experimentRunWorkflowStatus
          message
        }
      }
    `,
    options
  );
}
