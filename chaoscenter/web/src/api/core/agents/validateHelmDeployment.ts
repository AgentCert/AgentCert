import { gql, useMutation, MutationHookOptions } from '@apollo/client';

export const VALIDATE_HELM_DEPLOYMENT = gql`
  mutation validateHelmDeployment($projectID: ID!, $request: DeployAgentWithHelmRequest!) {
    validateHelmDeployment(projectID: $projectID, request: $request) {
      valid
      errors
      mergedValues
      releaseName
      namespace
    }
  }
`;

export interface ValidationResponse {
  valid: boolean;
  errors?: string[];
  mergedValues?: string;
  releaseName: string;
  namespace: string;
}

export interface ValidateHelmDeploymentResponse {
  validateHelmDeployment: ValidationResponse;
}

export const useValidateHelmDeployment = (
  options?: MutationHookOptions<ValidateHelmDeploymentResponse>
) => {
  return useMutation<ValidateHelmDeploymentResponse>(VALIDATE_HELM_DEPLOYMENT, options);
};
