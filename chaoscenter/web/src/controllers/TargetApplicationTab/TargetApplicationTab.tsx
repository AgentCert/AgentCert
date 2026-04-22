import React from 'react';
import { KubeGVRRequest, kubeObjectSubscription, kubeNamespaceSubscription } from '@api/core';
import type { ChaosEngine, FaultData } from '@models';
import { TargetApplicationTab } from '@views/ExperimentCreationFaultConfiguration/Tabs';
import type { AppInfoData, TargetApplicationData } from './types';
import { gvrData } from './grvData';

interface TargetApplicationControllerProps {
  engineCR: ChaosEngine | undefined;
  infrastructureID: string | undefined;
  setFaultData: React.Dispatch<React.SetStateAction<FaultData | undefined>>;
}

export default function TargetApplicationTabController({
  engineCR,
  infrastructureID,
  setFaultData
}: TargetApplicationControllerProps): React.ReactElement {
  const [namespaceData, setNamespaceData] = React.useState<string[]>([]);
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

  return (
    <TargetApplicationTab
      appInfoData={appInfoData}
      namespaceData={namespaceData}
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
