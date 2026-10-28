import React from 'react';
import { Button, ButtonVariation, Layout } from '@harnessio/uicore';
import { testModelConfig } from '@api/core';
import { useStrings } from '@strings';
import { getScope } from '@utils';
import ModelTestResultView from '@views/ModelConfiguration/ModelTestResult';

interface TestModelConfigControllerProps {
  modelID: string;
  onClose: () => void;
}

// Sends a registered model one test prompt as soon as it opens.
export default function TestModelConfigController({
  modelID,
  onClose
}: TestModelConfigControllerProps): React.ReactElement {
  const scope = getScope();
  const { getString } = useStrings();
  const [testModelConfigMutation, { data, error, loading }] = testModelConfig();

  React.useEffect(() => {
    testModelConfigMutation({ variables: { projectID: scope.projectID, modelID } }).catch(() => undefined);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [modelID]);

  const result = error ? { ok: false, error: error.message } : data?.testModelConfig;
  return (
    <Layout.Vertical padding="small" spacing="medium">
      <ModelTestResultView testing={loading || !result} result={result} />
      <Layout.Horizontal>
        <Button variation={ButtonVariation.SECONDARY} text={getString('done')} onClick={onClose} />
      </Layout.Horizontal>
    </Layout.Vertical>
  );
}
