import { gql, useQuery } from '@apollo/client';
import type { FaultCatalog } from '@api/entities';
import type { GqlAPIQueryRequest, GqlAPIQueryResponse } from '@api/types';

export interface GetFaultCatalogRequest {
  projectID: string;
}

export interface GetFaultCatalogResponse {
  getFaultCatalog: FaultCatalog;
}

export function getFaultCatalog({
  projectID,
  options = {}
}: GqlAPIQueryRequest<GetFaultCatalogResponse, GetFaultCatalogRequest>): GqlAPIQueryResponse<
  GetFaultCatalogResponse,
  GetFaultCatalogRequest
> {
  const { data, loading, ...rest } = useQuery<GetFaultCatalogResponse, GetFaultCatalogRequest>(
    gql`
      query getFaultCatalog($projectID: ID!) {
        getFaultCatalog(projectID: $projectID) {
          applications {
            key
            folders
            namespace
            labelKey
            services
          }
          faults {
            faultName
            classification
            compatibleApps
            workloadKinds
            requiredServices
            knownFailingTargets
          }
        }
      }
    `,
    {
      variables: {
        projectID
      },
      fetchPolicy: options.fetchPolicy ?? 'cache-first',
      ...options
    }
  );

  return {
    data,
    loading,
    ...rest
  };
}
