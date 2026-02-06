import { gql, useMutation } from '@apollo/client';
import type { FaultStudio, CreateFaultStudioRequestInput } from '@api/entities';
import type { GqlAPIMutationRequest, GqlAPIMutationResponse } from '@api/types';

export interface CreateFaultStudioRequest {
  projectID: string;
  request: CreateFaultStudioRequestInput;
}

export interface CreateFaultStudioResponse {
  createFaultStudio: FaultStudio;
}

export function createFaultStudio(
  options?: GqlAPIMutationRequest<CreateFaultStudioResponse, CreateFaultStudioRequest>
): GqlAPIMutationResponse<CreateFaultStudioResponse, CreateFaultStudioRequest> {
  const [createFaultStudioMutation, result] = useMutation<CreateFaultStudioResponse, CreateFaultStudioRequest>(
    gql`
      mutation createFaultStudio($projectID: ID!, $request: CreateFaultStudioRequest!) {
        createFaultStudio(projectID: $projectID, request: $request) {
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

  return [createFaultStudioMutation, result];
}
