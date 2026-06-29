import { gql, useQuery } from '@apollo/client';
import type { GqlAPIQueryRequest, GqlAPIQueryResponse } from '../../types';

export interface CertificationExperimentSummary {
  projectID: string;
  agentID: string;
  agentName: string;
  experimentID: string;
  status: string;
  expectedRuns: number;
  totalRuns: number;
  completedRuns: number;
  failedRuns: number;
  ready: boolean;
  certificationID?: string;
  generatedAt?: string;
}

export interface GetCertificationStatusRequest {
  projectID: string;
  experimentID: string;
}

export interface GetCertificationStatusResponse {
  getCertificationStatus: CertificationExperimentSummary | null;
}

export function getCertificationStatus({
  projectID,
  experimentID,
  options = {}
}: GqlAPIQueryRequest<GetCertificationStatusResponse, GetCertificationStatusRequest>): GqlAPIQueryResponse<
  GetCertificationStatusResponse,
  GetCertificationStatusRequest
> {
  const { data, loading, ...rest } = useQuery<GetCertificationStatusResponse, GetCertificationStatusRequest>(
    gql`
      query getCertificationStatus($projectID: ID!, $experimentID: String!) {
        getCertificationStatus(projectID: $projectID, experimentID: $experimentID) {
          projectID
          agentID
          agentName
          experimentID
          status
          expectedRuns
          totalRuns
          completedRuns
          failedRuns
          ready
          certificationID
          generatedAt
        }
      }
    `,
    {
      variables: { projectID, experimentID },
      fetchPolicy: options.fetchPolicy ?? 'network-only',
      ...options
    }
  );

  return { data, loading, ...rest };
}
