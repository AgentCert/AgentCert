import { gql, useMutation } from '@apollo/client';
import type { FaultStudio } from '@api/entities';
import type { GqlAPIMutationRequest, GqlAPIMutationResponse } from '@api/types';

export interface SetFaultStudioActiveRequest {
  projectID: string;
  studioID: string;
  isActive: boolean;
}

export interface SetFaultStudioActiveResponse {
  setFaultStudioActive: FaultStudio;
}

export function setFaultStudioActive(
  options?: GqlAPIMutationRequest<SetFaultStudioActiveResponse, SetFaultStudioActiveRequest>
): GqlAPIMutationResponse<SetFaultStudioActiveResponse, SetFaultStudioActiveRequest> {
  const [setFaultStudioActiveMutation, result] = useMutation<SetFaultStudioActiveResponse, SetFaultStudioActiveRequest>(
    gql`
      mutation setFaultStudioActive($projectID: ID!, $studioID: ID!, $isActive: Boolean!) {
        setFaultStudioActive(projectID: $projectID, studioID: $studioID, isActive: $isActive) {
          id
          name
          isActive
          totalFaults
          enabledFaults
          updatedAt
        }
      }
    `,
    options
  );

  return [setFaultStudioActiveMutation, result];
}
