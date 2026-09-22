import React from 'react';
import { useParams } from 'react-router-dom';
import { KubeGVRRequest, kubeObjectSubscription, kubeNamespaceSubscription } from '@api/core';
import type { ChaosEngine, FaultData, KubernetesExperimentManifest } from '@models';
import { InfrastructureType } from '@api/entities';
import { useFaultCatalog } from '@hooks';
import experimentYamlService from '@services/experiment';
import { TargetApplicationTab } from '@views/ExperimentCreationFaultConfiguration/Tabs';
import type { AppInfoData, TargetApplicationData } from './types';
import { gvrData } from './grvData';

interface PendingInstallApplication {
  folder: string;
  namespace: string;
}

interface TargetApplicationControllerProps {
  engineCR: ChaosEngine | undefined;
  infrastructureID: string | undefined;
  setFaultData: React.Dispatch<React.SetStateAction<FaultData | undefined>>;
  faultName: string | undefined;
}

// Chart-install steps (install-application/install-app) declare their target
// app via `-folder=<value>` and target namespace via `-namespace=<value>`.
// A namespace created by one of these steps won't exist in the live cluster
// yet, so the live kube subscriptions cannot surface its workloads. Scanning
// the in-progress manifest lets the picker offer both namespace and labels
// before the experiment has ever run.
function getPendingInstallApplications(
  manifest: KubernetesExperimentManifest | undefined
): PendingInstallApplication[] {
  const spec = manifest?.spec;
  const templates = spec && 'workflowSpec' in spec ? spec.workflowSpec?.templates : spec?.templates;
  if (!templates) return [];

  const pendingApps = new Map<string, PendingInstallApplication>();
  for (const template of templates) {
    const image = template.container?.image ?? '';
    if (!image.includes('install-app')) continue;

    const args = template.container?.args ?? [];
    const folder = args.find(arg => arg.startsWith('-folder='))?.slice('-folder='.length) ?? '';
    const namespace = args.find(arg => arg.startsWith('-namespace='))?.slice('-namespace='.length) ?? '';
    if (namespace) {
      pendingApps.set(namespace, { folder, namespace });
    }
  }
  return Array.from(pendingApps.values());
}

export default function TargetApplicationTabController({
  engineCR,
  infrastructureID,
  setFaultData,
  faultName
}: TargetApplicationControllerProps): React.ReactElement {
  const { experimentKey } = useParams<{ experimentKey: string }>();
  const experimentHandler = experimentYamlService.getInfrastructureTypeHandler(InfrastructureType.KUBERNETES);
  const faultCatalog = useFaultCatalog();
  const [namespaceData, setNamespaceData] = React.useState<string[]>([]);
  const [pendingInstallApplications, setPendingInstallApplications] = React.useState<PendingInstallApplication[]>([]);
  const [appInfoData, setAppInfoData] = React.useState<AppInfoData>({ appLabels: [] });
  const [targetApp, setTargetApp] = React.useState<TargetApplicationData>({
    ...engineCR?.spec?.appinfo
  });
  const [selectedGVR, setSelectedGVR] = React.useState<KubeGVRRequest>();
  const currentPendingApp = pendingInstallApplications.find(pendingApp => pendingApp.namespace === targetApp?.appns);
  const selectedNamespaceIsPending = !!targetApp?.appns && currentPendingApp !== undefined;

  const { data: resultNamespace, loading: loadingNamespace } = kubeNamespaceSubscription({
    request: {
      infraID: infrastructureID ?? ''
    },
    shouldResubscribe: true,
    skip: targetApp?.appkind === undefined || selectedGVR === undefined
  });
  const { data: resultObject, loading: loadingObject } = kubeObjectSubscription({
    shouldResubscribe: true,
    skip: targetApp?.appns === undefined || targetApp?.appns === '' || selectedNamespaceIsPending,
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

  // Surface apps that an install-app step earlier in this same,
  // not-yet-run workflow will create, so they're selectable ahead of time.
  React.useEffect(() => {
    if (!experimentKey) return;
    experimentHandler
      ?.getExperiment(experimentKey)
      .then(experiment => {
        setPendingInstallApplications(getPendingInstallApplications(experiment?.manifest));
      })
      .catch(() => setPendingInstallApplications([]));
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

  // Fault -> app/kind/service compatibility, resolved from the server's fault
  // catalogue (AppsHub registry + chaos-charts capability catalogue). A fault or
  // application the catalogue has not heard of falls back to the unrestricted
  // lists computed above, so onboarding either one needs no change here.
  const compatibleApps = faultCatalog.applications.filter(app => faultCatalog.isFaultCompatible(faultName, app.key));
  const restrictsApps = compatibleApps.length !== faultCatalog.applications.length;
  const compatibleNamespaces = restrictsApps ? compatibleApps.map(app => app.namespace) : undefined;

  const pendingNamespaces = pendingInstallApplications.map(pendingApp => pendingApp.namespace);

  // The fault can only target the application this experiment installs. Scoping
  // by fault compatibility alone is not enough: a *generic* fault is compatible
  // with every registered app, which left `compatibleNamespaces` undefined and
  // fell through to every namespace the cluster happens to have -- offering
  // unrelated ones like `litmus` and letting a run be built that injects outside
  // its own application. install-application is a prerequisite for adding any
  // fault (see ExperimentVisualBuilder's canAddFaults), so this list is
  // populated whenever a fault is being configured; the unrestricted fallback
  // only applies to a manifest that declares no application at all.
  const experimentNamespaces = pendingNamespaces.filter(
    ns => !compatibleNamespaces || compatibleNamespaces.includes(ns)
  );
  const allowedNamespaces = pendingNamespaces.length > 0 ? experimentNamespaces : compatibleNamespaces;

  const filteredNamespaceData = allowedNamespaces
    ? namespaceData.filter(ns => allowedNamespaces.includes(ns))
    : namespaceData;
  const filteredPendingNamespaces = pendingInstallApplications
    .filter(pendingApp => {
      const app = faultCatalog.resolveApplication(pendingApp.folder, pendingApp.namespace);
      return app
        ? faultCatalog.isFaultCompatible(faultName, app.key)
        : !compatibleNamespaces || compatibleNamespaces.includes(pendingApp.namespace);
    })
    .map(pendingApp => pendingApp.namespace);

  // Which registered app the currently selected namespace corresponds to, so the
  // AppLabel picker can be narrowed to that app's service list.
  const currentApp = faultCatalog.resolveApplication(currentPendingApp?.folder, targetApp?.appns);
  const requiredServices = faultCatalog.faultRequiredServices(faultName);
  const compatibleServices = currentApp ? requiredServices ?? currentApp.services : undefined;
  const pendingAppInfoData: AppInfoData =
    currentApp && selectedNamespaceIsPending
      ? {
          appLabels: (compatibleServices ?? currentApp.services).map(service => ({
            name: service,
            label: `${currentApp.labelKey}=${service}`
          }))
        }
      : { appLabels: [] };
  const sourceAppInfoData = pendingAppInfoData.appLabels.length > 0 ? pendingAppInfoData : appInfoData;
  const filteredAppInfoData: AppInfoData = requiredServices
    ? { appLabels: sourceAppInfoData.appLabels.filter(option => requiredServices.includes(option.name)) }
    : sourceAppInfoData;

  return (
    <TargetApplicationTab
      appInfoData={filteredAppInfoData}
      namespaceData={filteredNamespaceData}
      pendingNamespaces={filteredPendingNamespaces}
      allowedAppKinds={faultCatalog.faultWorkloadKinds(faultName)}
      targetApp={targetApp}
      setTargetApp={setTargetApp}
      engineCR={engineCR}
      setFaultData={setFaultData}
      infrastructureID={infrastructureID}
      loadingNamespace={loadingNamespace}
      loadingObject={selectedNamespaceIsPending ? false : loadingObject}
    />
  );
}
