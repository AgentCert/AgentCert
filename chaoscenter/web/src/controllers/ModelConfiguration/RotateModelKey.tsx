import React from 'react';
import { useToaster } from '@harnessio/uicore';
import { updateModelConfigKey } from '@api/core';
import type { ModelConfig } from '@api/entities';
import { useStrings } from '@strings';
import { getScope } from '@utils';
import RotateModelKeyView from '@views/ModelConfiguration/RotateModelKey';
import { PROVIDER_RULES } from '@views/ModelConfiguration/modelConfigUtils';

interface RotateModelKeyControllerProps {
  config: ModelConfig;
  onClose: () => void;
  refetchModels: () => void;
}

export default function RotateModelKeyController({
  config,
  onClose,
  refetchModels
}: RotateModelKeyControllerProps): React.ReactElement {
  const scope = getScope();
  const { getString } = useStrings();
  const { showSuccess, showError } = useToaster();
  const [updateModelConfigKeyMutation, { loading }] = updateModelConfigKey();

  const onSubmit = (apiKey: string): void => {
    updateModelConfigKeyMutation({ variables: { projectID: scope.projectID, modelID: config.modelID, apiKey } })
      .then(() => {
        showSuccess(getString('modelKeyRotated', { alias: config.alias }));
        refetchModels();
        onClose();
      })
      .catch(error => showError(error.message));
  };

  return (
    <RotateModelKeyView
      alias={config.alias}
      needsKey={PROVIDER_RULES[config.provider]?.needsKey ?? true}
      saving={loading}
      onSubmit={onSubmit}
      onClose={onClose}
    />
  );
}
