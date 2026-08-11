import { gql, useMutation } from '@apollo/client';
import type { GqlAPIMutationRequest, GqlAPIMutationResponse } from '@api/types';
import type { Toleration } from '@models';
import type { InfrastructureType } from '@api/entities';

export interface connectChaosInfraRequest {
  projectID: string;
  request: {
    name: string;
    environmentID: string;
    description?: string;
    platformName: string;
    infraNamespace?: string;
    serviceAccount?: string;
    infraScope: string;
    infraNsExists?: boolean;
    infraSaExists?: boolean;
    skipSsl?: boolean;
    nodeSelector?: string;
    tolerations?: Array<Toleration>;
    infrastructureType?: InfrastructureType;
    tags?: Array<string>;
  };
}

export interface connectChaosInfraManifestModeResponse {
  registerInfra: {
    manifest: string;
    // JWT used to fetch the manifest directly from `/file/<token>.yaml` — lets
    // `kubectl apply -f <url>` run straight on the target host, no browser
    // download + manual copy required.
    token: string;
  };
}

export function connectChaosInfraManifestMode(
  options?: GqlAPIMutationRequest<connectChaosInfraManifestModeResponse, connectChaosInfraRequest>
): GqlAPIMutationResponse<connectChaosInfraManifestModeResponse, connectChaosInfraRequest> {
  const [connectChaosInfraMutation, result] = useMutation<
    connectChaosInfraManifestModeResponse,
    connectChaosInfraRequest
  >(
    gql`
      mutation registerInfra($projectID: ID!, $request: RegisterInfraRequest!) {
        registerInfra(projectID: $projectID, request: $request) {
          manifest
          token
        }
      }
    `,
    options
  );

  return [connectChaosInfraMutation, result];
}
