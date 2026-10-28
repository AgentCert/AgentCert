import React from 'react';
import { useToaster } from '@harnessio/uicore';
import { listAgentModelOptions, listModelConfigs } from '@api/core';
import { getScope } from '@utils';
import ModelConfigurationListView from '@views/ModelConfiguration';
import { hasUnsettledModels, toModelRows } from '@views/ModelConfiguration/modelConfigUtils';

// While a model is registering or being deleted, the list is re-read until
// the gateway settles it.
const POLL_INTERVAL_MS = 10_000;

export default function ModelConfigurationController(): React.ReactElement {
  const scope = getScope();
  const { showError } = useToaster();

  const {
    data: configData,
    loading: configLoading,
    refetch: refetchConfigs,
    startPolling,
    stopPolling
  } = listModelConfigs({ ...scope, options: { onError: error => showError(error.message) } });
  // The platform models, exactly as Studio offers them.
  const {
    data: optionData,
    loading: optionLoading,
    refetch: refetchOptions
  } = listAgentModelOptions({ ...scope, options: { onError: error => showError(error.message) } });

  const configs = configData?.listModelConfigs;
  const unsettled = hasUnsettledModels(configs);
  React.useEffect(() => {
    if (unsettled) {
      startPolling(POLL_INTERVAL_MS);
    } else {
      stopPolling();
    }
    return () => stopPolling();
  }, [unsettled, startPolling, stopPolling]);

  const rows = React.useMemo(
    () => (configs ? toModelRows(configs, optionData?.listAgentModelOptions ?? []) : undefined),
    [configs, optionData]
  );

  const refetchModels = (): void => {
    refetchConfigs();
    refetchOptions();
  };

  return (
    <ModelConfigurationListView rows={rows} loading={configLoading || optionLoading} refetchModels={refetchModels} />
  );
}
