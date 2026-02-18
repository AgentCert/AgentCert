import { gql, useQuery, QueryHookOptions } from '@apollo/client';

export const GET_ENVIRONMENT_VARIABLES = gql`
  query getEnvironmentVariables {
    getEnvironmentVariables {
      name
      value
      isSensitive
    }
  }
`;

export interface EnvironmentVariable {
  name: string;
  value: string;
  isSensitive?: boolean | null;
}

export interface GetEnvironmentVariablesResponse {
  getEnvironmentVariables: EnvironmentVariable[];
}

export function useGetEnvironmentVariables(
  options?: QueryHookOptions<GetEnvironmentVariablesResponse>
) {
  const { data, loading, error } = useQuery<GetEnvironmentVariablesResponse>(
    GET_ENVIRONMENT_VARIABLES,
    options
  );

  return { data, loading, error };
}
