import { gql, useMutation, MutationHookOptions } from '@apollo/client';

export const DELETE_AGENT = gql`
  mutation deleteAgent($agentID: ID!, $hardDelete: Boolean) {
    deleteAgent(agentID: $agentID, hardDelete: $hardDelete) {
      success
      message
    }
  }
`;

export interface DeleteAgentResponse {
  success: boolean;
  message: string;
}

export interface DeleteAgentMutationResponse {
  deleteAgent: DeleteAgentResponse;
}

export interface DeleteAgentVariables {
  agentID: string;
  hardDelete?: boolean;
}

export function useDeleteAgent(
  options?: MutationHookOptions<DeleteAgentMutationResponse, DeleteAgentVariables>
): [(variables: DeleteAgentVariables) => Promise<{ data?: DeleteAgentMutationResponse | null }>, { loading: boolean; error?: Error }] {
  const [mutate, { loading, error }] = useMutation<DeleteAgentMutationResponse, DeleteAgentVariables>(
    DELETE_AGENT,
    options
  );

  const deleteAgent = async (variables: DeleteAgentVariables): Promise<{ data?: DeleteAgentMutationResponse | null }> => {
    return mutate({ variables });
  };

  return [deleteAgent, { loading, error: error as Error | undefined }];
}
