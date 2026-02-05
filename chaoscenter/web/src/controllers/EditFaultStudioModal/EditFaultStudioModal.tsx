import React from 'react';
import { updateFaultStudio } from '@api/core';
import type { FaultStudio } from '@api/entities';
import EditFaultStudioModalView from '@views/EditFaultStudioModal';

interface EditFaultStudioModalControllerProps {
  isOpen: boolean;
  onClose: () => void;
  faultStudio: FaultStudio;
  onSuccess: () => void;
}

export default function EditFaultStudioModalController({
  isOpen,
  onClose,
  faultStudio,
  onSuccess
}: EditFaultStudioModalControllerProps): React.ReactElement {
  // Update mutation
  const [updateFaultStudioMutation, { loading: updateLoading }] = updateFaultStudio();

  return (
    <EditFaultStudioModalView
      isOpen={isOpen}
      onClose={onClose}
      faultStudio={faultStudio}
      updateFaultStudioMutation={updateFaultStudioMutation}
      updateLoading={updateLoading}
      onSuccess={onSuccess}
    />
  );
}
