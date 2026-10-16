import React from 'react';
import { useParams } from 'react-router-dom';
import { useToaster } from '@harnessio/uicore';
import { parse } from 'yaml';
import { getChaosFaultLazyQuery, listChaosFaultsLazyQuery, listChaosHub } from '@api/core';
import { InfrastructureType } from '@api/entities';
import { getHash, getScope } from '@utils';
import { useStrings } from '@strings';
import experimentYamlService from '@services/experiment';
import type { KubernetesYamlService } from '@services/experiment';
import type { ChaosEngine, ChaosExperiment, FaultData } from '@models';
import ExperimentCreationSelectUninstallStepView from '@views/ExperimentCreationSelectUninstallStep';
import type { UninstallTargetEntry } from '@views/ExperimentCreationSelectUninstallStep';

interface ExperimentCreationSelectUninstallStepControllerProps {
  isOpen: boolean;
  kind: 'application' | 'agent';
  onSelect: (data: FaultData) => void;
  onClose: () => void;
}

// The hub fault names for the two teardown steps. `-folder` == helm release,
// `-namespace` == where it lives; both are injected as the fault engine's
// FOLDER / NAMESPACE env below.
const UNINSTALL_FAULT_NAME: Record<'application' | 'agent', string> = {
  application: 'uninstall-application',
  agent: 'uninstall-agent'
};

function setEngineEnv(engine: ChaosEngine, name: string, value: string): void {
  const components = engine.spec?.experiments?.[0]?.spec?.components;
  if (!components) return;
  if (!components.env) components.env = [];
  const existing = components.env.find(entry => entry.name === name);
  if (existing) existing.value = value;
  else components.env.push({ name, value });
}

export default function ExperimentCreationSelectUninstallStepController({
  isOpen,
  kind,
  onSelect,
  onClose
}: ExperimentCreationSelectUninstallStepControllerProps): React.ReactElement {
  const scope = getScope();
  const { getString } = useStrings();
  const { showError } = useToaster();
  const { experimentKey } = useParams<{ experimentKey: string }>();
  const experimentHandler = experimentYamlService.getInfrastructureTypeHandler(InfrastructureType.KUBERNETES) as
    | KubernetesYamlService
    | undefined;

  const [installedTargets, setInstalledTargets] = React.useState<UninstallTargetEntry[]>([]);
  const [targetsLoading, setTargetsLoading] = React.useState<boolean>(true);
  const [submitting, setSubmitting] = React.useState<boolean>(false);

  React.useEffect(() => {
    const handler = experimentHandler;
    if (!handler) {
      setTargetsLoading(false);
      return;
    }
    let cancelled = false;
    setTargetsLoading(true);
    handler.getExperiment(experimentKey).then(experiment => {
      if (cancelled) return;
      setInstalledTargets(handler.getInstalledTargets(experiment?.manifest, kind));
      setTargetsLoading(false);
    });
    return () => {
      cancelled = true;
    };
  }, [experimentHandler, experimentKey, kind]);

  const { data: chaoshubs } = listChaosHub({
    ...scope,
    options: { onError: error => showError(error.message) }
  });
  const defaultHub = chaoshubs?.listChaosHub?.find(hub => hub.isDefault) ?? chaoshubs?.listChaosHub?.[0];

  const [listChaosFaultsQuery] = listChaosFaultsLazyQuery({
    fetchPolicy: 'cache-first',
    onError: error => showError(error.message)
  });
  const [getChaosFaultQuery] = getChaosFaultLazyQuery({
    fetchPolicy: 'cache-first',
    onError: error => showError(error.message)
  });

  const handleSelect = async (entry: { folder: string; namespace: string }): Promise<void> => {
    if (!experimentHandler || !defaultHub) {
      showError(getString('uninstallStepHubUnavailable'));
      return;
    }
    const faultName = UNINSTALL_FAULT_NAME[kind];

    try {
      setSubmitting(true);

      // The uninstall faults live under whichever chart category the synced
      // default hub filed them in ("kubernetes" today) -- discover it rather
      // than hardcoding.
      const faultsResponse = await listChaosFaultsQuery({
        variables: { projectID: scope.projectID, hubID: defaultHub.id }
      });
      const category = (faultsResponse.data?.listChaosFaults ?? []).find(chart =>
        chart.spec.faults.some(fault => fault.name === faultName)
      )?.metadata.name;
      if (!category) {
        showError(getString('uninstallStepFaultNotInHub', { fault: faultName }));
        return;
      }

      const faultResponse = await getChaosFaultQuery({
        variables: {
          projectID: scope.projectID,
          request: { category, experimentName: faultName, hubID: defaultHub.id }
        }
      });
      const rawFault = faultResponse.data?.getChaosFault?.fault;
      const rawEngine = faultResponse.data?.getChaosFault?.engine ?? '';
      if (!rawFault) {
        showError(getString('uninstallStepFaultNotInHub', { fault: faultName }));
        return;
      }

      const processed = await experimentHandler.preProcessChaosEngineAndExperimentManifest(
        experimentKey,
        parse(rawEngine) as ChaosEngine,
        parse(rawFault) as ChaosExperiment
      );
      if (!processed) {
        showError(getString('uninstallStepHubUnavailable'));
        return;
      }

      setEngineEnv(processed.chaosEngine, 'FOLDER', entry.folder);
      setEngineEnv(processed.chaosEngine, 'NAMESPACE', entry.namespace);

      onSelect({
        faultName: getHash(3, faultName),
        faultCR: processed.chaosExperiment,
        engineCR: processed.chaosEngine,
        weight: 10
      });
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <ExperimentCreationSelectUninstallStepView
      isOpen={isOpen}
      kind={kind}
      loading={targetsLoading}
      submitting={submitting}
      installedTargets={installedTargets}
      onSelect={handleSelect}
      onClose={onClose}
    />
  );
}
