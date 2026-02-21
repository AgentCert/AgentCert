import React from 'react';
import { useParams } from 'react-router-dom';
import { useToaster } from '@harnessio/uicore';
import { getScope } from '@utils';
import { getFaultStudio } from '@api/core';
import FaultStudioView from '@views/FaultStudio';

interface FaultStudioParams {
  studioID: string;
}

export default function FaultStudioController(): React.ReactElement {
  const { studioID } = useParams<FaultStudioParams>();
  const scope = getScope();
  const { showError } = useToaster();

  const {
    data,
    loading,
    refetch
  } = getFaultStudio({
    projectID: scope.projectID,
    studioID,
    options: {
      onError: (err: Error) => showError(err.message)
    }
  });

  const faultStudio = data?.getFaultStudio;

  return (
    <FaultStudioView
      faultStudio={faultStudio}
      loading={loading}
      refetch={refetch}
    />
  );
}
