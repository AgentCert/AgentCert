import { gql, useMutation, MutationHookOptions } from '@apollo/client';

export const DEPLOY_AGENT_WITH_HELM = gql`
  mutation deployAgentWithHelm($projectID: ID!, $request: DeployAgentWithHelmRequest!) {
    deployAgentWithHelm(projectID: $projectID, request: $request) {
      agentID
      name
      status
      helmReleaseName
      helmChartVersion
    }
  }
`;

export interface DeployedAgent {
  agentID: string;
  name: string;
  status: string;
  helmReleaseName?: string;
  helmChartVersion?: string;
}

export interface DeployAgentWithHelmRequest {
  projectID: string;
  request: {
    name: string;
    description?: string;
    namespace: string;
    clusterName?: string;
    capabilities: string[];
    version?: string;
    helmReleaseName: string;
    helmChartVersion: string;
    valuesYaml?: string;
    kubeconfig?: string;
  };
}

export interface DeployAgentWithHelmResponse {
  deployAgentWithHelm: DeployedAgent;
}

export function useDeployAgentWithHelm(
  options?: MutationHookOptions<DeployAgentWithHelmResponse, DeployAgentWithHelmRequest>
): [
  (variables: DeployAgentWithHelmRequest) => Promise<{ data?: DeployAgentWithHelmResponse | null | undefined }>,
  { data: DeployAgentWithHelmResponse | null | undefined; loading: boolean; error: Error | undefined }
] {
  const [deployAgentMutation, { data, loading, error }] = useMutation<
    DeployAgentWithHelmResponse,
    DeployAgentWithHelmRequest
  >(DEPLOY_AGENT_WITH_HELM, options);

  const deployAgentWithHelm = async (
    variables: DeployAgentWithHelmRequest
  ): Promise<{ data?: DeployAgentWithHelmResponse | null | undefined }> => {
    return deployAgentMutation({ variables });
  };

  return [deployAgentWithHelm, { data, loading, error: error as Error | undefined }];
}
