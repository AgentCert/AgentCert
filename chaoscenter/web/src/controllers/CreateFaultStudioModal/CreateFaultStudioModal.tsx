import React from 'react';
import { getScope } from '@utils';
import { listChaosHub, createFaultStudio } from '@api/core';
import CreateFaultStudioModalView from '@views/CreateFaultStudioModal';

interface CreateFaultStudioModalControllerProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

export default function CreateFaultStudioModalController({
  isOpen,
  onClose,
  onSuccess
}: CreateFaultStudioModalControllerProps): React.ReactElement {
  const scope = getScope();

  // Fetch available ChaosHubs for selection
  const {
    data: chaosHubsData,
    loading: chaosHubsLoading
  } = listChaosHub({
    projectID: scope.projectID,
    options: {
      skip: !isOpen // Only fetch when modal is open
    }
  });

  // Create mutation
  const [createFaultStudioMutation, { loading: createLoading }] = createFaultStudio();

  const chaosHubs = chaosHubsData?.listChaosHub ?? [];

  return (
    <CreateFaultStudioModalView
      isOpen={isOpen}
      onClose={onClose}
      chaosHubs={chaosHubs}
      chaosHubsLoading={chaosHubsLoading}
      createFaultStudioMutation={createFaultStudioMutation}
      createLoading={createLoading}
      onSuccess={onSuccess}
    />
  );
}
