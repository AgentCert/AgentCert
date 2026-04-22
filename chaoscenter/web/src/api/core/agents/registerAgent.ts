import { gql, useMutation, MutationHookOptions } from '@apollo/client';

export const REGISTER_AGENT = gql`
  mutation registerAgent($projectID: ID!, $request: RegisterAgentRequest!) {
    registerAgent(projectID: $projectID, request: $request) {
      agentID
      name
      token
      status
    }
  }
`;

export interface Agent {
  agentID: string;
  name: string;
  token?: string;
  status: string;
}

export interface RegisterAgentRequest {
  projectID: string;
  request: {
    name: string;
    description?: string;
    namespace: string;
    clusterName?: string;
    capabilities: string[];
    version?: string;
    helmReleaseName?: string;
    helmChartVersion?: string;
  };
}

export interface RegisterAgentResponse {
  registerAgent: Agent;
}

export function useRegisterAgent(
  options?: MutationHookOptions<RegisterAgentResponse, RegisterAgentRequest>
): [
  (variables: RegisterAgentRequest) => Promise<{ data?: RegisterAgentResponse | null | undefined }>,
  { data: RegisterAgentResponse | null | undefined; loading: boolean; error: Error | undefined }
] {
  const [registerAgentMutation, { data, loading, error }] = useMutation<
    RegisterAgentResponse,
    RegisterAgentRequest
  >(REGISTER_AGENT, options);

  const registerAgent = async (variables: RegisterAgentRequest): Promise<{ data?: RegisterAgentResponse | null | undefined }> => {
    return registerAgentMutation({ variables });
  };

  return [registerAgent, { data, loading, error: error as Error | undefined }];
}
