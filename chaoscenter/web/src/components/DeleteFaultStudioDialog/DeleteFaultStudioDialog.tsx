import React from 'react';
import { Intent } from '@blueprintjs/core';
import { ConfirmationDialog, useToaster } from '@harnessio/uicore';
import { getScope } from '@utils';
import { useStrings } from '@strings';
import { deleteFaultStudio } from '@api/core';

interface DeleteFaultStudioDialogProps {
  isOpen: boolean;
  onClose: () => void;
  studioId: string;
  studioName: string;
  onSuccess: () => void;
}

export default function DeleteFaultStudioDialog({
  isOpen,
  onClose,
  studioId,
  studioName,
  onSuccess
}: DeleteFaultStudioDialogProps): React.ReactElement {
  const { getString } = useStrings();
  const scope = getScope();
  const { showSuccess, showError } = useToaster();

  const [deleteFaultStudioMutation, { loading }] = deleteFaultStudio({
    onCompleted: () => {
      showSuccess(getString('faultStudioDeletedSuccessfully'));
      onSuccess();
      onClose();
    },
    onError: error => {
      showError(error.message);
    }
  });

  const handleDelete = (): void => {
    deleteFaultStudioMutation({
      variables: {
        projectID: scope.projectID,
        studioID: studioId
      }
    });
  };

  const confirmationDialogProps = {
    usePortal: true,
    contentText: `Are you sure you want to delete the Fault Studio "${studioName}"? This action cannot be undone.`,
    titleText: getString('deleteFaultStudio'),
    cancelButtonText: getString('cancel'),
    confirmButtonText: loading ? 'Deleting...' : getString('confirm'),
    intent: Intent.DANGER,
    onClose: (isConfirmed: boolean) => {
      if (isConfirmed) {
        handleDelete();
      } else {
        onClose();
      }
    }
  };

  return <ConfirmationDialog isOpen={isOpen} {...confirmationDialogProps} />;
}
