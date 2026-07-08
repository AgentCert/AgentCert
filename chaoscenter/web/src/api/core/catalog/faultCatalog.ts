import { gql, useQuery } from '@apollo/client';
import type { QueryHookOptions } from '@apollo/client';

// ── Type definitions ──────────────────────────────────────────────────────────

export type FaultScope = 'GENERAL' | 'DOMAIN' | 'APP_SPECIFIC';
export type FaultParameterType = 'INTEGER' | 'STRING' | 'BOOLEAN' | 'ENUM' | 'PERCENT';

export interface FaultParameter {
  key: string;
  displayName: string;
  type: FaultParameterType;
  unit?: string;
  default: string;
  min?: number;
  max?: number;
  required: boolean;
  description: string;
  litmusEnv?: string;
  allowedValues?: string[];
}

export interface FaultDescription {
  short: string;
  long: string;
}

export interface FaultGroundTruth {
  category: string;
  impact: string;
  detectWithinSecs: number;
  mitigateWithinSecs: number;
}

export interface FaultSpec {
  name: string;
  displayName: string;
  version: string;
  tier: string;
  scope: FaultScope;
  domain?: string;
  targetApp?: string;
  tags: string[];
  description: FaultDescription;
  parameters: FaultParameter[];
  groundTruth: FaultGroundTruth;
}

// ── Queries ───────────────────────────────────────────────────────────────────

export const FAULTS_FOR_APP_QUERY = gql`
  query FaultsForApp($appName: String!) {
    faultsForApp(appName: $appName) {
      name
      displayName
      scope
      domain
      targetApp
      tags
      description {
        short
        long
      }
      parameters {
        key
        displayName
        type
        default
        required
        description
        min
        max
        unit
        allowedValues
      }
      groundTruth {
        category
        impact
        detectWithinSecs
        mitigateWithinSecs
      }
    }
  }
`;

export const LIST_FAULTS_QUERY = gql`
  query ListFaults($scope: FaultScope, $domain: String, $targetApp: String) {
    listFaults(scope: $scope, domain: $domain, targetApp: $targetApp) {
      name
      displayName
      scope
      domain
      targetApp
      tags
      description {
        short
      }
      groundTruth {
        category
        impact
      }
    }
  }
`;

export const GET_FAULT_QUERY = gql`
  query GetFault($name: String!) {
    getFault(name: $name) {
      name
      displayName
      scope
      domain
      targetApp
      description {
        short
        long
      }
      parameters {
        key
        displayName
        type
        default
        required
        description
        min
        max
        unit
        allowedValues
        litmusEnv
      }
      groundTruth {
        category
        impact
        detectWithinSecs
        mitigateWithinSecs
        detectionHints
        remediationHints
      }
    }
  }
`;

// ── Hook wrappers ─────────────────────────────────────────────────────────────

export interface FaultsForAppRequest {
  appName: string;
}

export interface FaultsForAppResponse {
  faultsForApp: FaultSpec[];
}

export function useFaultsForApp(
  options?: QueryHookOptions<FaultsForAppResponse, FaultsForAppRequest>
) {
  return useQuery<FaultsForAppResponse, FaultsForAppRequest>(FAULTS_FOR_APP_QUERY, {
    fetchPolicy: 'cache-and-network',
    ...options
  });
}

export interface GetFaultRequest {
  name: string;
}

export interface GetFaultResponse {
  getFault: FaultSpec | null;
}

export function useGetFault(options?: QueryHookOptions<GetFaultResponse, GetFaultRequest>) {
  return useQuery<GetFaultResponse, GetFaultRequest>(GET_FAULT_QUERY, {
    fetchPolicy: 'cache-first',
    ...options
  });
}
