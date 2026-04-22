import React from 'react';
import { useToaster } from '@harnessio/uicore';
import { listAgentHubCategories } from '@api/core';
import { getScope } from '@utils';
import AgentHubView from '@views/AgentHub';

export default function AgentHubController(): React.ReactElement {
  const scope = getScope();
  const { showError } = useToaster();

  const {
    data,
    loading,
    refetch
  } = listAgentHubCategories({
    ...scope,
    options: {
      onError: err => showError(err.message)
    }
  });

  return (
    <AgentHubView
      categories={data?.listAgentHubCategories}
      loading={loading}
      refetch={refetch}
    />
  );
}
