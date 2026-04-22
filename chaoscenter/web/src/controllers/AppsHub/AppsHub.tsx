import React from 'react';
import { useToaster } from '@harnessio/uicore';
import { listAppHubCategories } from '@api/core';
import { getScope } from '@utils';
import AppsHubView from '@views/AppsHub';

export default function AppsHubController(): React.ReactElement {
  const scope = getScope();
  const { showError } = useToaster();

  const {
    data,
    loading,
    refetch
  } = listAppHubCategories({
    ...scope,
    options: {
      onError: err => showError(err.message)
    }
  });

  return (
    <AppsHubView
      categories={data?.listAppHubCategories}
      loading={loading}
      refetch={refetch}
    />
  );
}
