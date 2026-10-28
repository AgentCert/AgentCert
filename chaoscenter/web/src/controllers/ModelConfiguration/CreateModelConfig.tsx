import React from 'react';
import { useToaster } from '@harnessio/uicore';
import { createModelConfig, testModelConnection } from '@api/core';
import type { ModelTestResult } from '@api/entities';
import { useStrings } from '@strings';
import { getScope } from '@utils';
import ModelConfigForm from '@views/ModelConfiguration/ModelConfigForm';
import {
  ConnectionTest,
  connectionKey,
  ModelFormValues,
  toConnection
} from '@views/ModelConfiguration/modelConfigUtils';

interface CreateModelConfigControllerProps {
  onClose: () => void;
  refetchModels: () => void;
}

export default function CreateModelConfigController({
  onClose,
  refetchModels
}: CreateModelConfigControllerProps): React.ReactElement {
  const scope = getScope();
  const { getString } = useStrings();
  const { showSuccess, showError } = useToaster();
  const [lastTest, setLastTest] = React.useState<ConnectionTest>();
  const [createModelConfigMutation, { loading: saving }] = createModelConfig();
  const [testModelConnectionMutation, { loading: testing }] = testModelConnection();

  const runTest = (values: ModelFormValues): Promise<ModelTestResult> => {
    setLastTest(undefined);
    return testModelConnectionMutation({ variables: { projectID: scope.projectID, request: toConnection(values) } })
      .then(({ data }): ModelTestResult => data?.testModelConnection ?? { ok: false })
      .catch((error): ModelTestResult => ({ ok: false, error: error.message }))
      .then(result => {
        setLastTest({ connection: connectionKey(values), result });
        return result;
      });
  };

  const save = (values: ModelFormValues): void => {
    createModelConfigMutation({
      variables: {
        projectID: scope.projectID,
        request: {
          alias: values.alias.trim(),
          label: values.label.trim() || undefined,
          connection: toConnection(values)
        }
      }
    })
      .then(({ data }) => {
        showSuccess(getString('modelAdded', { alias: data?.createModelConfig.alias ?? values.alias }));
        refetchModels();
        onClose();
      })
      .catch(error => {
        showError(error.message);
        // A create the gateway did not confirm is listed as Registering.
        refetchModels();
      });
  };

  // A model that cannot answer (a wrong deployment name, a bad key) is never
  // saved unnoticed: Save tests it first and stops if the test fails. Saving
  // those same values again (Save anyway) is the user's decision.
  const onSubmit = (values: ModelFormValues): void => {
    if (lastTest?.connection === connectionKey(values)) {
      save(values);
      return;
    }
    runTest(values).then(result => {
      if (result.ok) save(values);
    });
  };

  return (
    <ModelConfigForm
      saving={saving}
      testing={testing}
      lastTest={lastTest}
      onTest={runTest}
      onSubmit={onSubmit}
      onClose={onClose}
    />
  );
}
