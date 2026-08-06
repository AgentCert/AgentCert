import React from 'react';
import { useToaster } from '@harnessio/uicore';
import { listAppHubCategories, listAgentHubCategories } from '@api/core';
import { getScope } from '@utils';
import ExperimentCreationSelectInstallStepView from '@views/ExperimentCreationSelectInstallStep';
import type { InstallStepEntry } from '@views/ExperimentCreationSelectInstallStep';

interface ExperimentCreationSelectInstallStepControllerProps {
  isOpen: boolean;
  kind: 'application' | 'agent';
  onSelect: (entry: { folder: string; namespace: string }) => void;
  onClose: () => void;
}

export default function ExperimentCreationSelectInstallStepController({
  isOpen,
  kind,
  onSelect,
  onClose
}: ExperimentCreationSelectInstallStepControllerProps): React.ReactElement {
  const scope = getScope();
  const { showError } = useToaster();

  const { data: appHubData, loading: appHubLoading } = listAppHubCategories({
    ...scope,
    options: { onError: error => showError(error.message), skip: kind !== 'application' }
  });

  const { data: agentHubData, loading: agentHubLoading } = listAgentHubCategories({
    ...scope,
    options: { onError: error => showError(error.message), skip: kind !== 'agent' }
  });

  const entries: InstallStepEntry[] =
    kind === 'application'
      ? (appHubData?.listAppHubCategories ?? []).flatMap(category =>
          category.applications.map(app => ({
            folder: app.name,
            displayName: app.displayName,
            description: app.description,
            namespace: app.namespace
          }))
        )
      : (agentHubData?.listAgentHubCategories ?? []).flatMap(category =>
          category.agents.map(agent => ({
            folder: agent.name,
            displayName: agent.displayName,
            description: agent.description,
            namespace: agent.namespace
          }))
        );

  return (
    <ExperimentCreationSelectInstallStepView
      isOpen={isOpen}
      kind={kind}
      loading={kind === 'application' ? appHubLoading : agentHubLoading}
      entries={entries}
      onSelect={onSelect}
      onClose={onClose}
    />
  );
}
