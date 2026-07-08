import { gql, useQuery } from '@apollo/client';
import type { QueryHookOptions } from '@apollo/client';
import type { ApplicationSpec } from '@api/entities';

export const LIST_APPLICATIONS = gql`
  query listApplications($projectID: ID!) {
    listApplications(projectID: $projectID) {
      name
      displayName
      version
      tier
      domain
      capabilityDomains
      tags
      description { short long suitableFor notSuitableFor }
      install {
        method folder timeout wait
        chartRef { repo chart version }
        namespace { default configurable }
      }
      healthProbe { urlTemplate expectedStatus initialDelaySeconds periodSeconds failureThreshold }
      loadTest { enabled method image args }
      microservices { name displayName description k8sLabel k8sKind k8sNamespace criticality relevantFaults dependsOn }
      faultCompatibility { faultName compatible notes recommendedTargets }
      inputs { key displayName description type required default helmPath values min max unit advanced }
      schemaVersion
    }
  }
`;

export interface ListApplicationsRequest {
  projectID: string;
}

export interface ListApplicationsResponse {
  listApplications: ApplicationSpec[];
}

export const listApplications = (
  options?: QueryHookOptions<ListApplicationsResponse, ListApplicationsRequest>
) =>
  useQuery<ListApplicationsResponse, ListApplicationsRequest>(LIST_APPLICATIONS, {
    ...options
  });
