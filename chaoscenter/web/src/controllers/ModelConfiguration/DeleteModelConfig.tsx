import React from 'react';
import { useToaster } from '@harnessio/uicore';
import { deleteModelConfig } from '@api/core';
import type { ModelConfig } from '@api/entities';
import { useStrings } from '@strings';
import { getScope } from '@utils';
import DeleteModelConfigView from '@views/ModelConfiguration/DeleteModelConfig';

interface DeleteModelConfigControllerProps {
  config: ModelConfig;
  onClose: () => void;
  refetchModels: () => void;
}

export default function DeleteModelConfigController({
  config,
  onClose,
  refetchModels
}: DeleteModelConfigControllerProps): React.ReactElement {
  const scope = getScope();
  const { getString } = useStrings();
  const { showSuccess } = useToaster();
  // Kept in the dialog: it names the experiments still using the model.
  const [error, setError] = React.useState<string>();
  const [deleteModelConfigMutation, { loading }] = deleteModelConfig();

  const onConfirm = (): void => {
    setError(undefined);
    deleteModelConfigMutation({ variables: { projectID: scope.projectID, modelID: config.modelID } })
      .then(() => {
        showSuccess(getString('modelDeleted', { alias: config.alias }));
        refetchModels();
        onClose();
      })
      .catch(err => {
        setError(err.message);
        refetchModels();
      });
  };

  return (
    <DeleteModelConfigView
      alias={config.alias}
      deleting={loading}
      error={error}
      onConfirm={onConfirm}
      onClose={onClose}
    />
  );
}
