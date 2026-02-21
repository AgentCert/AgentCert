import { gql, useMutation } from '@apollo/client';
import type { ToggleFaultResponse } from '@api/entities';
import type { GqlAPIMutationRequest, GqlAPIMutationResponse } from '@api/types';

export interface ToggleFaultInStudioRequest {
  projectID: string;
  studioID: string;
  faultName: string;
  enabled: boolean;
}

export interface ToggleFaultInStudioResponse {
  toggleFaultInStudio: ToggleFaultResponse;
}

export function toggleFaultInStudio(
  options?: GqlAPIMutationRequest<ToggleFaultInStudioResponse, ToggleFaultInStudioRequest>
): GqlAPIMutationResponse<ToggleFaultInStudioResponse, ToggleFaultInStudioRequest> {
  const [toggleFaultInStudioMutation, result] = useMutation<ToggleFaultInStudioResponse, ToggleFaultInStudioRequest>(
    gql`
      mutation toggleFaultInStudio($projectID: ID!, $studioID: ID!, $faultName: String!, $enabled: Boolean!) {
        toggleFaultInStudio(projectID: $projectID, studioID: $studioID, faultName: $faultName, enabled: $enabled) {
          success
          message
          faultStudio {
            id
            name
            totalFaults
            enabledFaults
            isActive
          }
        }
      }
    `,
    options
  );

  return [toggleFaultInStudioMutation, result];
}
