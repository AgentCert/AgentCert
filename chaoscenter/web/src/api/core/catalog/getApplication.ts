import { gql, useQuery } from '@apollo/client';
import type { QueryHookOptions } from '@apollo/client';
import type { ApplicationSpec } from '@api/entities';

export const GET_APPLICATION = gql`
  query getApplication($projectID: ID!, $appName: String!) {
    getApplication(projectID: $projectID, appName: $appName) {
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

export interface GetApplicationRequest {
  projectID: string;
  appName: string;
}

export interface GetApplicationResponse {
  getApplication: ApplicationSpec | null;
}

export const getApplication = (
  options?: QueryHookOptions<GetApplicationResponse, GetApplicationRequest>
) =>
  useQuery<GetApplicationResponse, GetApplicationRequest>(GET_APPLICATION, {
    ...options
  });
