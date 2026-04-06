import React from 'react';
import { useToaster } from '@harnessio/uicore';
import { listAgentHubCategories } from '@api/core';
import { getScope } from '@utils';
import AgentDetailView from '@views/AgentDetail';

export default function AgentDetailController(): React.ReactElement {
  const scope = getScope();
  const { showError } = useToaster();

  const { data, loading } = listAgentHubCategories({
    ...scope,
    options: {
      onError: err => showError(err.message)
    }
  });

  return (
    <AgentDetailView
      categories={data?.listAgentHubCategories}
      loading={loading}
    />
  );
}
