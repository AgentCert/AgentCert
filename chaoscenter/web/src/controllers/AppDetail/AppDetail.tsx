import React from 'react';
import { useToaster } from '@harnessio/uicore';
import { useParams } from 'react-router-dom';
import { getApplication } from '@api/core';
import { getScope } from '@utils';
import AppDetailView from '@views/AppDetail';

export default function AppDetailController(): React.ReactElement {
  const scope = getScope();
  const { showError } = useToaster();
  const { appName } = useParams<{ appName: string }>();

  const { data, loading } = getApplication({
    variables: {
      projectID: scope.projectID,
      appName
    },
    fetchPolicy: 'cache-and-network',
    onError: err => showError(err.message)
  });

  return (
    <AppDetailView
      app={data?.getApplication}
      loading={loading}
    />
  );
}
