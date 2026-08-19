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
    // Server-computed `kubectl apply -f <url>` target. Use this instead of
    // building the URL from window.location.origin/token — the browser's own
    // origin is only correct when the browser and the shell running kubectl
    // are reachable via the same address, which breaks under SSH tunnels,
    // VS Code port-forwarding, etc. See CLAUDE.md §6 "Known Operational
    // Gotchas" for the incident that motivated this.
    manifestDownloadURL: string;
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
          manifestDownloadURL
        }
      }
    `,
    options
  );

  return [connectChaosInfraMutation, result];
}
