import React from 'react';
import { useParams } from 'react-router-dom';
import { KubeGVRRequest, kubeObjectSubscription, kubeNamespaceSubscription } from '@api/core';
import type { ChaosEngine, FaultData, KubernetesExperimentManifest } from '@models';
import { InfrastructureType } from '@api/entities';
import experimentYamlService from '@services/experiment';
import { TargetApplicationTab } from '@views/ExperimentCreationFaultConfiguration/Tabs';
import type { AppInfoData, TargetApplicationData } from './types';
import { gvrData } from './grvData';
import { APP_NAMESPACES, APP_SERVICES, CompatibleApp, getFaultCompatibility } from './faultApplicationCompatibility';

interface TargetApplicationControllerProps {
  engineCR: ChaosEngine | undefined;
  infrastructureID: string | undefined;
  setFaultData: React.Dispatch<React.SetStateAction<FaultData | undefined>>;
  faultName: string | undefined;
}

// Chart-install steps (install-application/install-app) declare their target
// namespace via a `-namespace=<value>` container arg. A namespace created by
// one of these steps won't exist in the live cluster yet — the workflow
// hasn't run — so the live kubeNamespaceSubscription can never surface it.
// Scanning the in-progress manifest for these args lets the picker offer it
// anyway, before the experiment has ever been run.
function getPendingInstallNamespaces(manifest: KubernetesExperimentManifest | undefined): string[] {
  const spec = manifest?.spec;
  const templates = spec && 'workflowSpec' in spec ? spec.workflowSpec?.templates : spec?.templates;
  if (!templates) return [];

  const namespaces = new Set<string>();
  for (const template of templates) {
    const image = template.container?.image ?? '';
    if (!image.includes('install-app')) continue;
    const args = template.container?.args ?? [];
    for (const arg of args) {
      const match = /^-namespace=(.+)$/.exec(arg);
      if (match?.[1]) namespaces.add(match[1]);
    }
  }
  return Array.from(namespaces);
}

export default function TargetApplicationTabController({
  engineCR,
  infrastructureID,
  setFaultData,
  faultName
}: TargetApplicationControllerProps): React.ReactElement {
  const { experimentKey } = useParams<{ experimentKey: string }>();
  const experimentHandler = experimentYamlService.getInfrastructureTypeHandler(InfrastructureType.KUBERNETES);
  const [namespaceData, setNamespaceData] = React.useState<string[]>([]);
  const [pendingNamespaces, setPendingNamespaces] = React.useState<string[]>([]);
  const [appInfoData, setAppInfoData] = React.useState<AppInfoData>({ appLabels: [] });
  const [targetApp, setTargetApp] = React.useState<TargetApplicationData>({
    ...engineCR?.spec?.appinfo
  });
  const [selectedGVR, setSelectedGVR] = React.useState<KubeGVRRequest>();
  const { data: resultNamespace, loading: loadingNamespace } = kubeNamespaceSubscription({
    request: {
      infraID: infrastructureID ?? ''
    },
    shouldResubscribe: true,
    skip: targetApp?.appkind === undefined || selectedGVR === undefined
  });
  const { data: resultObject, loading: loadingObject } = kubeObjectSubscription({
    shouldResubscribe: true,
    skip: targetApp?.appns === undefined || targetApp?.appns === '',
    request: {
      infraID: infrastructureID ?? '',
      kubeObjRequest: selectedGVR,
      namespace: targetApp?.appns ?? '',
      objectType: 'kubeobject'
    }
  });

  // Call this for 1st render to pre-populate the data
  React.useEffect(() => {
    gvrData.map(data => {
      if (data.resource === targetApp?.appkind) {
        setSelectedGVR({
          group: data.group,
          resource: `${data.resource}s`,
          version: data.version
        });
      }
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [targetApp?.appkind]);

  React.useEffect(() => {
    if (resultNamespace?.getKubeNamespace) {
      setNamespaceData(resultNamespace.getKubeNamespace.kubeNamespace.map(data => data.name));
    }
  }, [resultNamespace?.getKubeNamespace, targetApp?.appkind]);

  // Surface namespaces that an install-app step earlier in this same,
  // not-yet-run workflow will create, so they're selectable ahead of time.
  React.useEffect(() => {
    if (!experimentKey) return;
    experimentHandler
      ?.getExperiment(experimentKey)
      .then(experiment => {
        setPendingNamespaces(getPendingInstallNamespaces(experiment?.manifest));
      })
      .catch(() => setPendingNamespaces([]));
  }, [experimentKey, experimentHandler]);

  React.useEffect(() => {
    if (resultObject?.getKubeObject) {
      const preferredKeys = ['app.kubernetes.io/instance', 'app.kubernetes.io/name', 'app', 'name'];
      const appLabels = resultObject.getKubeObject.kubeObj.data.map(objData => {
        const labels = objData.labels ?? [];
        let selectedLabel = labels.find(label => label.endsWith(`=${objData.name}`)) ?? '';
        if (!selectedLabel) {
          for (const key of preferredKeys) {
            const match = labels.find(label => label.startsWith(`${key}=`));
            if (match) {
              selectedLabel = match;
              break;
            }
          }
        }
        if (!selectedLabel) {
          selectedLabel = `app.kubernetes.io/instance=${objData.name}`;
        }
        return { name: objData.name, label: selectedLabel };
      });
      const appInfo: AppInfoData = { appLabels };
      setAppInfoData(appInfo);
    }
  }, [resultObject?.getKubeObject, targetApp?.appns]);

  // Fault -> app/kind/service compatibility, per
  // agents/FAULT_APPLICATION_COMPATIBILITY.md (see faultApplicationCompatibility.ts).
  // A fault with no known entry falls back to the unrestricted lists computed
  // above, so uncatalogued faults behave exactly as before.
  const compatibility = getFaultCompatibility(faultName);

  const compatibleNamespaces = compatibility?.apps.map(app => APP_NAMESPACES[app]);
  const filteredNamespaceData = compatibleNamespaces
    ? namespaceData.filter(ns => compatibleNamespaces.includes(ns))
    : namespaceData;
  const filteredPendingNamespaces = compatibleNamespaces
    ? pendingNamespaces.filter(ns => compatibleNamespaces.includes(ns))
    : pendingNamespaces;

  // Which known app the currently selected namespace corresponds to, so the
  // AppLabel picker can be narrowed to that app's compatible service list.
  const currentApp = (Object.entries(APP_NAMESPACES) as [CompatibleApp, string][]).find(
    ([, namespace]) => namespace === targetApp?.appns
  )?.[0];
  const compatibleServices = currentApp
    ? compatibility?.servicesByApp?.[currentApp] ?? (compatibility ? APP_SERVICES[currentApp] : undefined)
    : undefined;
  const filteredAppInfoData: AppInfoData = compatibleServices
    ? { appLabels: appInfoData.appLabels.filter(option => compatibleServices.includes(option.name)) }
    : appInfoData;

  return (
    <TargetApplicationTab
      appInfoData={filteredAppInfoData}
      namespaceData={filteredNamespaceData}
      pendingNamespaces={filteredPendingNamespaces}
      allowedAppKinds={compatibility?.appKinds}
      targetApp={targetApp}
      setTargetApp={setTargetApp}
      engineCR={engineCR}
      setFaultData={setFaultData}
      infrastructureID={infrastructureID}
      loadingNamespace={loadingNamespace}
      loadingObject={loadingObject}
    />
  );
}
