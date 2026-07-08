import { gql, useQuery } from '@apollo/client';
import type { QueryHookOptions } from '@apollo/client';

// Extended Agent type with ACE agent registry fields needed for the wizard
export interface AgentLLMConfig {
  configRef?: string;
  provider?: string;
  model?: string;
  allowUserChoice: boolean;
  allowedModels: string[];
  defaultModel?: string;
  llmDependent: boolean;
}

export interface AgentCompatibilityInfo {
  supportedApps: string[];
  unsupportedApps: string[];
  minimumFaultCount: number;
  maximumFaultCount: number;
}

export interface AgentForStudio {
  agentID: string;
  name: string;
  displayName?: string;
  description?: string;
  version: string;
  vendor: string;
  capabilities: string[];
  llmConfig?: AgentLLMConfig;
  compatibility?: AgentCompatibilityInfo;
}

export interface AgentListForStudioResponse {
  totalCount: number;
  totalPages: number;
  currentPage: number;
  hasNextPage: boolean;
  agents: AgentForStudio[];
}

export const LIST_AGENTS_FOR_STUDIO = gql`
  query listAgentsForStudio($pagination: PaginationInput!, $filter: ListAgentsFilter) {
    listAgents(pagination: $pagination, filter: $filter) {
      totalCount
      agents {
        agentID
        name
        displayName
        description
        version
        vendor
        capabilities
        llmConfig {
          configRef
          provider
          model
          allowUserChoice
          allowedModels
          defaultModel
          llmDependent
        }
        compatibility {
          supportedApps
          unsupportedApps
          minimumFaultCount
          maximumFaultCount
        }
      }
    }
  }
`;

export interface ListAgentsForStudioVariables {
  pagination: { page: number; limit: number };
  filter?: { projectID?: string; status?: string; searchTerm?: string };
}

export interface ListAgentsForStudioQueryResponse {
  listAgents: AgentListForStudioResponse;
}

export function useListAgentsForStudio(
  options?: QueryHookOptions<ListAgentsForStudioQueryResponse, ListAgentsForStudioVariables>
) {
  return useQuery<ListAgentsForStudioQueryResponse, ListAgentsForStudioVariables>(
    LIST_AGENTS_FOR_STUDIO,
    {
      fetchPolicy: 'cache-and-network',
      ...options
    }
  );
}
