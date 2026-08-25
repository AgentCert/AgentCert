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

const APP_LABEL_KEYS: Record<CompatibleApp, string> = {
  'otel-demo': 'opentelemetry.io/name',
  'sock-shop': 'name',
  'book-info': 'app'
};

const APP_FOLDERS: Record<CompatibleApp, string[]> = {
  'otel-demo': ['otel-demo'],
  'sock-shop': ['sock-shop'],
  'book-info': ['bookinfo', 'book-info']
};

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

function resolveCompatibleApp(folder: string | undefined, namespace: string | undefined): CompatibleApp | undefined {
  // Trim + lowercase both sides -- the `-folder=`/`-namespace=` args are copied
  // verbatim from wherever the install step was authored (catalog picker, or a
  // hand-edited manifest), so a stray case/whitespace difference from the
  // hardcoded aliases here shouldn't be enough to silently fall back to "no
  // known app" and drop the synthesized label options.
  const normalizedFolder = folder?.trim().toLowerCase();
  const normalizedNamespace = namespace?.trim().toLowerCase();
  return (Object.entries(APP_NAMESPACES) as [CompatibleApp, string][]).find(([app, defaultNamespace]) => {
    const folders = APP_FOLDERS[app];
    return (
      (!!normalizedFolder && folders.some(f => f.toLowerCase() === normalizedFolder)) ||
      defaultNamespace.toLowerCase() === normalizedNamespace
    );
  })?.[0];
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

  // Fault -> app/kind/service compatibility, per
  // agents/FAULT_APPLICATION_COMPATIBILITY.md (see faultApplicationCompatibility.ts).
  // A fault with no known entry falls back to the unrestricted lists computed
  // above, so uncatalogued faults behave exactly as before.
  const compatibility = getFaultCompatibility(faultName);

  const compatibleNamespaces = compatibility?.apps.map(app => APP_NAMESPACES[app]);
  const pendingNamespaces = pendingInstallApplications.map(pendingApp => pendingApp.namespace);
  const filteredNamespaceData = compatibleNamespaces
    ? namespaceData.filter(ns => compatibleNamespaces.includes(ns))
    : namespaceData;
  const filteredPendingNamespaces = compatibility
    ? pendingInstallApplications
        .filter(pendingApp => {
          const app = resolveCompatibleApp(pendingApp.folder, pendingApp.namespace);
          return app ? compatibility.apps.includes(app) : compatibleNamespaces?.includes(pendingApp.namespace);
        })
        .map(pendingApp => pendingApp.namespace)
    : pendingNamespaces;

  // Which known app the currently selected namespace corresponds to, so the
  // AppLabel picker can be narrowed to that app's compatible service list.
  const currentApp = resolveCompatibleApp(currentPendingApp?.folder, targetApp?.appns);
  const compatibleServices = currentApp
    ? compatibility?.servicesByApp?.[currentApp] ?? (compatibility ? APP_SERVICES[currentApp] : undefined)
    : undefined;
  const pendingAppInfoData: AppInfoData =
    currentApp && selectedNamespaceIsPending
      ? {
          appLabels: (compatibleServices ?? APP_SERVICES[currentApp]).map(service => ({
            name: service,
            label: `${APP_LABEL_KEYS[currentApp]}=${service}`
          }))
        }
      : { appLabels: [] };
  const sourceAppInfoData = pendingAppInfoData.appLabels.length > 0 ? pendingAppInfoData : appInfoData;
  const filteredAppInfoData: AppInfoData = compatibleServices
    ? { appLabels: sourceAppInfoData.appLabels.filter(option => compatibleServices.includes(option.name)) }
    : sourceAppInfoData;

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
      loadingObject={selectedNamespaceIsPending ? false : loadingObject}
    />
  );
}
