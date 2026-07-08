import React from 'react';
import { useToaster } from '@harnessio/uicore';
import { listApplications } from '@api/core';
import { getScope } from '@utils';
import AppsHubView from '@views/AppsHub';

export default function AppsHubController(): React.ReactElement {
  const scope = getScope();
  const { showError } = useToaster();

  const { data, loading } = listApplications({
    variables: {
      projectID: scope.projectID
    },
    fetchPolicy: 'cache-and-network',
    onError: err => showError(err.message)
  });

  return (
    <AppsHubView
      apps={data?.listApplications ?? []}
      loading={loading}
    />
  );
}
