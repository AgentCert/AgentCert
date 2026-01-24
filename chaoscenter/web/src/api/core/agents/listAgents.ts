import { gql, useQuery, QueryHookOptions } from '@apollo/client';

export const LIST_AGENTS = gql`
  query listAgents($pagination: PaginationInput!, $filter: ListAgentsFilter) {
    listAgents(pagination: $pagination, filter: $filter) {
      totalCount
      totalPages
      currentPage
      hasNextPage
      agents {
        agentID
        projectID
        name
        description
        tags
        version
        vendor
        capabilities
        namespace
        status
        containerImage {
          registry
          repository
          tag
        }
        endpoint {
          url
          discoveryType
          endpointType
          healthPath
          readyPath
        }
        auditInfo {
          createdAt
          createdBy
          updatedAt
          updatedBy
        }
      }
    }
  }
`;

export interface ContainerImage {
  registry: string;
  repository: string;
  tag: string;
}

export interface AgentEndpoint {
  url: string;
  discoveryType: string;
  endpointType: string;
  healthPath: string;
  readyPath: string;
}

export interface ListedAgent {
  agentID: string;
  projectID: string;
  name: string;
  description?: string;
  tags?: string[];
  version: string;
  vendor: string;
  capabilities: string[];
  namespace: string;
  status: string;
  containerImage: ContainerImage;
  endpoint: AgentEndpoint;
  auditInfo?: {
    createdAt: string;
    createdBy: string;
    updatedAt: string;
    updatedBy: string;
  };
}

export interface AgentListResponse {
  totalCount: number;
  totalPages: number;
  currentPage: number;
  hasNextPage: boolean;
  agents: ListedAgent[];
}

export interface ListAgentsFilter {
  projectID?: string;
  status?: string;
  capabilities?: string[];
  searchTerm?: string;
  tags?: string[];
}

export interface ListAgentsPagination {
  page: number;
  limit: number;
}

export interface ListAgentsVariables {
  pagination: ListAgentsPagination;
  filter?: ListAgentsFilter;
}

export interface ListAgentsQueryResponse {
  listAgents: AgentListResponse;
}

export function useListAgents(
  options?: QueryHookOptions<ListAgentsQueryResponse, ListAgentsVariables>
): { data: ListAgentsQueryResponse | undefined; loading: boolean; error: Error | undefined; refetch: () => void } {
  const { data, loading, error, refetch } = useQuery<ListAgentsQueryResponse, ListAgentsVariables>(
    LIST_AGENTS,
    {
      ...options,
      fetchPolicy: options?.fetchPolicy || 'cache-and-network',
    }
  );

  return { data, loading, error: error as Error | undefined, refetch };
}
