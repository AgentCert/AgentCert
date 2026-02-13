import { gql, useMutation } from '@apollo/client';
import type { GqlAPIMutationRequest, GqlAPIMutationResponse } from '@api/types';

export interface DeleteFaultStudioRequest {
  projectID: string;
  studioID: string;
}

export interface DeleteFaultStudioResponse {
  deleteFaultStudio: boolean;
}

export function deleteFaultStudio(
  options?: GqlAPIMutationRequest<DeleteFaultStudioResponse, DeleteFaultStudioRequest>
): GqlAPIMutationResponse<DeleteFaultStudioResponse, DeleteFaultStudioRequest> {
  const [deleteFaultStudioMutation, result] = useMutation<DeleteFaultStudioResponse, DeleteFaultStudioRequest>(
    gql`
      mutation deleteFaultStudio($projectID: ID!, $studioID: ID!) {
        deleteFaultStudio(projectID: $projectID, studioID: $studioID)
      }
    `,
    options
  );

  return [deleteFaultStudioMutation, result];
}
