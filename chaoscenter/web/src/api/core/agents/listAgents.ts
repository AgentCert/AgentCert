import { gql, useQuery, QueryHookOptions } from '@apollo/client';

export const LIST_AGENTS = gql`
  query listAgents($projectID: ID!, $request: ListAgentsRequest) {
    listAgents(projectID: $projectID, request: $request) {
      totalAgents
      agents {
        agentID
        name
        description
        namespace
        clusterName
        capabilities
        status
        version
        helmReleaseName
        helmChartVersion
        isRemoved
        auditInfo {
          createdAt
          createdBy {
            userID
            username
          }
          updatedAt
          updatedBy {
            userID
            username
          }
        }
      }
    }
  }
`;

export interface ListedAgent {
  agentID: string;
  name: string;
  description?: string;
  namespace: string;
  clusterName?: string;
  capabilities: string[];
  status: string;
  version?: string;
  helmReleaseName?: string;
  helmChartVersion?: string;
  isRemoved?: boolean;
  auditInfo?: {
    createdAt?: string;
    createdBy?: {
      userID: string;
      username: string;
    };
    updatedAt?: string;
    updatedBy?: {
      userID: string;
      username: string;
    };
  };
}

export interface AgentListResponse {
  totalAgents: number;
  agents: ListedAgent[];
}

export interface ListAgentsPagination {
  page?: number;
  limit?: number;
}

export interface ListAgentsFilter {
  agentName?: string;
  status?: string;
  namespace?: string;
}

export interface ListAgentsRequest {
  projectID: string;
  request?: {
    pagination?: ListAgentsPagination;
    filter?: ListAgentsFilter;
  };
}

export interface ListAgentsQueryResponse {
  listAgents: AgentListResponse;
}

export function useListAgents(
  options?: QueryHookOptions<ListAgentsQueryResponse, ListAgentsRequest>
): { data: ListAgentsQueryResponse | undefined; loading: boolean; error: Error | undefined; refetch: () => void } {
  const { data, loading, error, refetch } = useQuery<ListAgentsQueryResponse, ListAgentsRequest>(
    LIST_AGENTS,
    {
      ...options,
      fetchPolicy: options?.fetchPolicy || 'cache-and-network',
    }
  );

  return { data, loading, error: error as Error | undefined, refetch };
}
