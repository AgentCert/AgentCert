import React from 'react';
import { useToaster } from '@harnessio/uicore';
import { getScope } from '@utils';
import { listFaultStudios } from '@api/core';
import FaultStudiosView from '@views/FaultStudios';
import type { FaultStudioSummary } from '@api/entities';

export default function FaultStudiosController(): React.ReactElement {
  const scope = getScope();
  const { showError } = useToaster();
  const [searchTerm, setSearchTerm] = React.useState<string>('');

  const {
    data,
    loading: listFaultStudiosLoading,
    refetch
  } = listFaultStudios({
    projectID: scope.projectID,
    request: {
      filter: {
        name: searchTerm || undefined
      },
      limit: 15,
      offset: 0
    },
    options: {
      onError: (err: Error) => showError(err.message)
    }
  });

  const faultStudios: FaultStudioSummary[] = data?.listFaultStudios?.faultStudios ?? [];
  const totalCount = data?.listFaultStudios?.totalCount ?? 0;

  return (
    <FaultStudiosView
      faultStudios={faultStudios}
      totalCount={totalCount}
      loading={listFaultStudiosLoading}
      searchTerm={searchTerm}
      setSearchTerm={setSearchTerm}
      refetch={refetch}
    />
  );
}
