import React from 'react';
import { useToaster } from '@harnessio/uicore';
import { listAppHubCategories } from '@api/core';
import { getScope } from '@utils';
import AppDetailView from '@views/AppDetail';

export default function AppDetailController(): React.ReactElement {
  const scope = getScope();
  const { showError } = useToaster();

  const { data, loading } = listAppHubCategories({
    ...scope,
    options: {
      onError: err => showError(err.message)
    }
  });

  return (
    <AppDetailView
      categories={data?.listAppHubCategories}
      loading={loading}
    />
  );
}
