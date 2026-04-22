import { gql, useQuery, QueryHookOptions } from '@apollo/client';

export const GET_KUBERNETES_NAMESPACES = gql`
  query getKubernetesNamespaces {
    getKubernetesNamespaces
  }
`;

export interface GetKubernetesNamespacesResponse {
  getKubernetesNamespaces: string[];
}

export function useGetKubernetesNamespaces(
  options?: QueryHookOptions<GetKubernetesNamespacesResponse>
) {
  const { data, loading, error } = useQuery<GetKubernetesNamespacesResponse>(
    GET_KUBERNETES_NAMESPACES,
    options
  );

  return { data, loading, error };
}
