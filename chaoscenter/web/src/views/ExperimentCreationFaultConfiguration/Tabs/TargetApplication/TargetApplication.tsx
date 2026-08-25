import React from 'react';
import { Container, DropDown, Layout, SelectOption, Text } from '@harnessio/uicore';
import { Color, FontVariation } from '@harnessio/design-system';
import type { ChaosEngine, FaultData } from '@models';
import { gvrData } from '@controllers/TargetApplicationTab/grvData';
import { useStrings } from '@strings';
import type { AppInfoData, TargetApplicationData } from '@controllers/TargetApplicationTab/types';

interface TargetApplicationViewProps {
  appInfoData: AppInfoData;
  namespaceData: string[];
  pendingNamespaces?: string[];
  // Restricts the App Kind dropdown to these gvrData resource names, per
  // agents/FAULT_APPLICATION_COMPATIBILITY.md. Undefined = unrestricted.
  allowedAppKinds?: string[];
  targetApp: TargetApplicationData | undefined;
  setTargetApp: React.Dispatch<React.SetStateAction<TargetApplicationData>>;
  engineCR: ChaosEngine | undefined;
  setFaultData: React.Dispatch<React.SetStateAction<FaultData | undefined>>;
  // getKubeObjectLazyQueryFunction: LazyQueryFunction<KubeObjResponse, KubeObjRequest>;
  infrastructureID: string | undefined;
  loadingNamespace: boolean;
  loadingObject: boolean;
}

export default function TargetApplicationTab({
  appInfoData,
  namespaceData,
  pendingNamespaces = [],
  allowedAppKinds,
  targetApp,
  setTargetApp,
  engineCR,
  setFaultData,
  // getKubeObjectLazyQueryFunction,
  loadingNamespace,
  loadingObject
}: TargetApplicationViewProps): React.ReactElement {
  const { getString } = useStrings();

  function getAppKindItems(): SelectOption[] {
    const source = allowedAppKinds ? gvrData.filter(data => allowedAppKinds.includes(data.resource)) : gvrData;
    return source.map(data => ({
      label: data.resource,
      value: data.resource
    }));
  }

  function getAppNamespaceItems(): SelectOption[] {
    if (loadingNamespace) return [];
    const liveItems = namespaceData.map(data => ({
      label: data,
      value: data
    }));
    // Namespaces an earlier install-app step in this same, not-yet-run
    // workflow will create — offered ahead of time since the live cluster
    // can't know about them yet. Tagged so they're distinguishable from
    // namespaces that already exist.
    const pendingItems = pendingNamespaces
      .filter(ns => !namespaceData.includes(ns))
      .map(ns => ({
        label: `${ns} (pending install)`,
        value: ns
      }));
    return [...liveItems, ...pendingItems];
  }

  function getAppLabelItems(): SelectOption[] {
    if (loadingObject) return [];
    return appInfoData?.appLabels.map(option => ({
      label: option.name,
      value: option.label
    }));
  }

  return (
    <Layout.Vertical background={Color.PRIMARY_BG} height={'100%'}>
      <Container padding={{ left: 'xxlarge', right: 'xxlarge', top: 'xlarge', bottom: 'xlarge' }}>
        <Text font={{ variation: FontVariation.BODY }}>{getString('provideTargetApplicationDetails')}</Text>
        {engineCR?.spec?.appinfo?.appkind !== undefined && (
          <Layout.Vertical margin={{ top: 'medium', bottom: 'medium' }} spacing="xsmall">
            <Text font={{ variation: FontVariation.FORM_LABEL }}>{getString('appKind')}</Text>
            <DropDown
              width={'50%'}
              placeholder={getString('selectAppKind')}
              items={getAppKindItems()}
              value={targetApp?.appkind}
              onChange={selectedItem => {
                const newKind = selectedItem.label;
                // A dropdown re-fires onChange even when the user re-selects the
                // already-active value (e.g. clicking through to confirm a
                // pre-filled default like "deployment"). Only reset Namespace/Label
                // -- and only when the kind has genuinely changed, since a live
                // namespace/object query keyed to the old kind is no longer valid,
                // but re-picking the same kind shouldn't silently discard work
                // already entered in the other two fields below.
                const kindChanged = newKind !== targetApp?.appkind;
                setTargetApp(kindChanged ? { appkind: newKind, applabel: '', appns: '' } : { ...targetApp, appkind: newKind });
                if (engineCR?.spec?.appinfo) {
                  engineCR.spec.appinfo.appkind = newKind;
                  if (kindChanged) {
                    if (engineCR.spec.appinfo.appns !== undefined) engineCR.spec.appinfo.appns = '';
                    if (engineCR.spec.appinfo.applabel !== undefined) engineCR.spec.appinfo.applabel = '';
                  }
                }
                setFaultData(faultData => {
                  if (faultData?.faultName) return { ...faultData, engineCR: faultData?.engineCR };
                });
              }}
            />
          </Layout.Vertical>
        )}
        {engineCR?.spec?.appinfo?.appns !== undefined && (
          <Layout.Vertical margin={{ top: 'medium', bottom: 'medium' }} spacing="xsmall">
            <Text font={{ variation: FontVariation.FORM_LABEL }}>{getString('appNameSpace')}</Text>
            <DropDown
              width={'50%'}
              placeholder={getString('selectAppNamespace')}
              items={getAppNamespaceItems()}
              value={targetApp?.appns}
              onChange={selectedItem => {
                // "Pending install" namespace options display a decorated
                // label (`${ns} (pending install)`) but carry the real
                // namespace name in `value` — falling back to `.label` here
                // (as opposed to the App Label dropdown below, which already
                // does this correctly) wrote the decorated string itself into
                // appns, which is never a real namespace.
                const selectedNamespace = selectedItem.value ?? selectedItem.label;
                const tmp = { ...targetApp, appns: selectedNamespace, applabel: '' };
                setTargetApp(tmp);
                if (engineCR?.spec?.appinfo?.appns !== undefined) engineCR.spec.appinfo.appns = selectedNamespace;
                setFaultData(faultData => {
                  if (faultData?.faultName) return { ...faultData, engineCR: faultData?.engineCR };
                });
              }}
            />
          </Layout.Vertical>
        )}
        {engineCR?.spec?.appinfo?.applabel !== undefined && (
          <Layout.Vertical margin={{ top: 'medium', bottom: 'medium' }} spacing="xsmall">
            <Text font={{ variation: FontVariation.FORM_LABEL }}>{getString('appLabel')}</Text>
            <DropDown
              width={'50%'}
              placeholder={getString('selectAppLabel')}
              items={getAppLabelItems()}
              value={targetApp?.applabel}
              onChange={selectedItem => {
                const selectedLabel = selectedItem.value ?? selectedItem.label;
                const tmp = {
                  ...targetApp,
                  applabel: selectedLabel
                };
                setTargetApp(tmp);
                if (engineCR?.spec?.appinfo?.applabel !== undefined)
                  engineCR.spec.appinfo.applabel = selectedLabel;
                setFaultData(faultData => {
                  if (faultData?.faultName) return { ...faultData, engineCR: faultData?.engineCR };
                });
              }}
            />
          </Layout.Vertical>
        )}
      </Container>
    </Layout.Vertical>
  );
}
