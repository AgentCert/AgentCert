import { gql, useMutation } from '@apollo/client';
import type { FaultStudio, UpdateFaultStudioRequestInput } from '@api/entities';
import type { GqlAPIMutationRequest, GqlAPIMutationResponse } from '@api/types';

export interface UpdateFaultStudioRequest {
  projectID: string;
  studioID: string;
  request: UpdateFaultStudioRequestInput;
}

export interface UpdateFaultStudioResponse {
  updateFaultStudio: FaultStudio;
}

export function updateFaultStudio(
  options?: GqlAPIMutationRequest<UpdateFaultStudioResponse, UpdateFaultStudioRequest>
): GqlAPIMutationResponse<UpdateFaultStudioResponse, UpdateFaultStudioRequest> {
  const [updateFaultStudioMutation, result] = useMutation<UpdateFaultStudioResponse, UpdateFaultStudioRequest>(
    gql`
      mutation updateFaultStudio($projectID: ID!, $studioID: ID!, $request: UpdateFaultStudioRequest!) {
        updateFaultStudio(projectID: $projectID, studioID: $studioID, request: $request) {
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

  return [updateFaultStudioMutation, result];
}
