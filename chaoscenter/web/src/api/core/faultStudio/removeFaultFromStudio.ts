import { gql, useMutation } from '@apollo/client';
import type { FaultStudio } from '@api/entities';
import type { GqlAPIMutationRequest, GqlAPIMutationResponse } from '@api/types';

export interface RemoveFaultFromStudioRequest {
  projectID: string;
  studioID: string;
  faultName: string;
}

export interface RemoveFaultFromStudioResponse {
  removeFaultFromStudio: FaultStudio;
}

export function removeFaultFromStudio(
  options?: GqlAPIMutationRequest<RemoveFaultFromStudioResponse, RemoveFaultFromStudioRequest>
): GqlAPIMutationResponse<RemoveFaultFromStudioResponse, RemoveFaultFromStudioRequest> {
  const [removeFaultFromStudioMutation, result] = useMutation<
    RemoveFaultFromStudioResponse,
    RemoveFaultFromStudioRequest
  >(
    gql`
      mutation removeFaultFromStudio($projectID: ID!, $studioID: ID!, $faultName: String!) {
        removeFaultFromStudio(projectID: $projectID, studioID: $studioID, faultName: $faultName) {
          id
          name
          description
          tags
          projectId
          sourceHubId
          sourceHubName
          selectedFaults {
            faultCategory
            faultName
            displayName
            description
            enabled
            injectionConfig {
              injectionType
              schedule
              duration
              targetSelector
              interval
            }
            customParameters
            weight
          }
          isActive
          totalFaults
          enabledFaults
          isRemoved
          createdAt
          updatedAt
          createdBy {
            username
          }
          updatedBy {
            username
          }
        }
      }
    `,
    options
  );

  return [removeFaultFromStudioMutation, result];
}
