import { gql, useMutation, MutationHookOptions } from '@apollo/client';

export const UPDATE_AGENT = gql`
  mutation updateAgent($agentID: ID!, $input: UpdateAgentInput!) {
    updateAgent(agentID: $agentID, input: $input) {
      agentID
      name
      description
      namespace
      version
      vendor
      capabilities
      status
      containerImage {
        registry
        repository
        tag
      }
      endpoint {
        url
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
`;

export interface ContainerImageInput {
  registry: string;
  repository: string;
  tag: string;
}

export interface AgentEndpointInput {
  url: string;
  endpointType: 'REST' | 'GRPC';
  healthPath?: string;
  readyPath?: string;
}

export interface UpdateAgentInput {
  name?: string;
  description?: string;
  tags?: string[];
  version?: string;
  vendor?: string;
  capabilities?: string[];
  containerImage?: ContainerImageInput;
  namespace?: string;
  endpoint?: AgentEndpointInput;
  status?: 'REGISTERED' | 'VALIDATING' | 'ACTIVE' | 'INACTIVE' | 'DELETED';
}

export interface UpdatedAgent {
  agentID: string;
  name: string;
  description?: string;
  namespace: string;
  version: string;
  vendor: string;
  capabilities: string[];
  status: string;
  containerImage: {
    registry: string;
    repository: string;
    tag: string;
  };
  endpoint: {
    url: string;
    endpointType: string;
    healthPath: string;
    readyPath: string;
  };
  auditInfo: {
    createdAt: string;
    createdBy: string;
    updatedAt: string;
    updatedBy: string;
  };
}

export interface UpdateAgentMutationResponse {
  updateAgent: UpdatedAgent;
}

export interface UpdateAgentVariables {
  agentID: string;
  input: UpdateAgentInput;
}

export function useUpdateAgent(
  options?: MutationHookOptions<UpdateAgentMutationResponse, UpdateAgentVariables>
): [(variables: UpdateAgentVariables) => Promise<{ data?: UpdateAgentMutationResponse | null }>, { loading: boolean; error?: Error }] {
  const [mutate, { loading, error }] = useMutation<UpdateAgentMutationResponse, UpdateAgentVariables>(
    UPDATE_AGENT,
    options
  );

  const updateAgent = async (variables: UpdateAgentVariables): Promise<{ data?: UpdateAgentMutationResponse | null }> => {
    return mutate({ variables });
  };

  return [updateAgent, { loading, error: error as Error | undefined }];
}
