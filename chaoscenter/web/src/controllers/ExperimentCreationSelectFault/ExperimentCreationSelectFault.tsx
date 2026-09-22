import React from 'react';
import { useToaster } from '@harnessio/uicore';
import { listChaosFaultsLazyQuery, listChaosHub } from '@api/core';
import { getScope } from '@utils';
import ExperimentCreationSelectFaultView from '@views/ExperimentCreationSelectFault';
import type { FaultData } from '@models';

interface ExperimentCreationSelectFaultControllerProps {
  isOpen: boolean;
  /** Application this experiment installs; narrows the fault list to compatible ones. */
  targetApplicationKey?: string;
  onSelect: (data: FaultData) => void;
  onClose: () => void;
}

export default function ExperimentCreationSelectFaultController({
  isOpen,
  targetApplicationKey,
  onSelect,
  onClose
}: ExperimentCreationSelectFaultControllerProps): React.ReactElement {
  const scope = getScope();
  const { showError } = useToaster();

  // List all chaoshubs
  const { data: chaoshubs, loading: listChaosHubLoading } = listChaosHub({
    ...scope,
    options: { onError: error => showError(error.message) }
  });

  const [listChaosFaultsQuery, { data: listChaosFaults, loading: listChaosFaultsLoading }] = listChaosFaultsLazyQuery({
    fetchPolicy: 'cache-first',
    onError: err => showError(err.message)
  });

  return (
    <ExperimentCreationSelectFaultView
      chaoshubs={chaoshubs?.listChaosHub}
      isOpen={isOpen}
      targetApplicationKey={targetApplicationKey}
      loading={{
        listChaosHub: listChaosHubLoading,
        listChaosFaults: listChaosFaultsLoading
      }}
      chaosCharts={listChaosFaults?.listChaosFaults}
      listChaosFaultsQuery={listChaosFaultsQuery}
      onSelect={onSelect}
      onClose={onClose}
    />
  );
}
