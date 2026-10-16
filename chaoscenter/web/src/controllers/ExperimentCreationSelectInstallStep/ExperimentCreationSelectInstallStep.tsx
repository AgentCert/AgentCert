import React from 'react';
import { SelectOption, useToaster } from '@harnessio/uicore';
import { listAppHubCategories, listAgentHubCategories, listAgentModelOptions } from '@api/core';
import { isAgentCompatible } from '@hooks';
import { getScope } from '@utils';
import ExperimentCreationSelectInstallStepView from '@views/ExperimentCreationSelectInstallStep';
import type { InstallStepEntry } from '@views/ExperimentCreationSelectInstallStep';
import type { InstallStepChoice, InstallStepSelection } from '@services/experiment/installStepSettings';

interface ExperimentCreationSelectInstallStepControllerProps {
  isOpen: boolean;
  kind: 'application' | 'agent';
  /** Application already installed by this experiment; narrows the agent list. */
  targetApplicationKey?: string;
  initialSelection?: InstallStepSelection;
  onSelect: (choice: InstallStepChoice) => void;
  onClose: () => void;
}

export default function ExperimentCreationSelectInstallStepController({
  isOpen,
  kind,
  targetApplicationKey,
  initialSelection,
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

  const { data: agentModelData } = listAgentModelOptions();
  const modelOptions = React.useMemo<SelectOption[]>(
    () => (agentModelData?.listAgentModelOptions ?? []).map(model => ({ label: model.label, value: model.alias })),
    [agentModelData]
  );
  const platformDefaultModelLabel = agentModelData?.listAgentModelOptions.find(model => model.isDefault)?.label;

  const entries = React.useMemo<InstallStepEntry[]>(
    () =>
      kind === 'application'
        ? (appHubData?.listAppHubCategories ?? []).flatMap(category =>
            category.applications.map(app => ({
              folder: app.name,
              displayName: app.displayName,
              description: app.description,
              namespace: app.namespace,
              configurations: app.configurations ?? []
            }))
          )
        : (agentHubData?.listAgentHubCategories ?? []).flatMap(category =>
            category.agents
              // An agent that declares no restriction stays listed, so onboarding
              // an application never requires editing an agent chart.
              .filter(agent => isAgentCompatible(agent.compatibleApplications, targetApplicationKey))
              .map(agent => ({
                folder: agent.name,
                displayName: agent.displayName,
                description: agent.description,
                namespace: agent.namespace,
                configurations: agent.configurations ?? []
              }))
          ),
    [kind, appHubData, agentHubData, targetApplicationKey]
  );

  return (
    <ExperimentCreationSelectInstallStepView
      isOpen={isOpen}
      kind={kind}
      loading={kind === 'application' ? appHubLoading : agentHubLoading}
      entries={entries}
      initialSelection={initialSelection}
      modelOptions={modelOptions}
      platformDefaultModelLabel={platformDefaultModelLabel}
      onSelect={onSelect}
      onClose={onClose}
    />
  );
}
