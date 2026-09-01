import React from 'react';
import { Button, ButtonVariation, Container, Layout, Text, TextInput } from '@harnessio/uicore';
import { Color, FontVariation } from '@harnessio/design-system';
import { Icon } from '@harnessio/icons';
import Drawer from '@components/Drawer';
import { DrawerTypes } from '@components/Drawer/Drawer';
import { useStrings } from '@strings';

export interface UninstallTargetEntry {
  folder: string;
  namespace: string;
}

interface ExperimentCreationSelectUninstallStepViewProps {
  isOpen: boolean;
  kind: 'application' | 'agent';
  loading: boolean;
  submitting: boolean;
  installedTargets: UninstallTargetEntry[];
  onSelect: (entry: { folder: string; namespace: string }) => void;
  onClose: () => void;
}

export default function ExperimentCreationSelectUninstallStepView({
  isOpen,
  kind,
  loading,
  submitting,
  installedTargets,
  onSelect,
  onClose
}: ExperimentCreationSelectUninstallStepViewProps): React.ReactElement {
  const { getString } = useStrings();
  const [folder, setFolder] = React.useState<string>('');
  const [namespace, setNamespace] = React.useState<string>('');

  // Pre-select the only installed target so the common (single install) case
  // is one click.
  React.useEffect(() => {
    if (folder === '' && namespace === '' && installedTargets.length === 1) {
      setFolder(installedTargets[0].folder);
      setNamespace(installedTargets[0].namespace);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [installedTargets]);

  const selectTarget = (target: UninstallTargetEntry): void => {
    setFolder(target.folder);
    setNamespace(target.namespace);
  };

  const title = (
    <Text font={{ variation: FontVariation.H5 }}>
      {kind === 'application' ? getString('uninstallApplication') : getString('uninstallAgent')}
    </Text>
  );

  return (
    <Drawer
      isOpen={isOpen}
      handleClose={onClose}
      title={title}
      type={DrawerTypes.InstallStep}
      leftPanel={
        <Layout.Vertical height={'100%'} padding={{ left: 'xlarge', right: 'xlarge', top: 'medium', bottom: 'large' }}>
          <Text font={{ variation: FontVariation.BODY }} margin={{ bottom: 'medium' }}>
            {kind === 'application'
              ? getString('uninstallApplicationDescription')
              : getString('uninstallAgentDescription')}
          </Text>
          <Container style={{ flexGrow: 1, overflowY: 'auto' }}>
            {loading && <Text font={{ variation: FontVariation.BODY }}>{getString('loading')}</Text>}
            {!loading && installedTargets.length === 0 && (
              <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_500}>
                {getString('uninstallNoInstalledTargets')}
              </Text>
            )}
            {installedTargets.map(target => {
              const isSelected = target.folder === folder && target.namespace === namespace;
              return (
                <Container
                  key={`${target.folder}/${target.namespace}`}
                  onClick={() => selectTarget(target)}
                  padding="medium"
                  margin={{ bottom: 'small' }}
                  style={{
                    cursor: 'pointer',
                    borderRadius: 4,
                    border: `1px solid ${isSelected ? 'var(--primary-7, #0278d5)' : 'var(--grey-200, #d9dae6)'}`,
                    background: isSelected ? 'var(--primary-1, #ecf6fe)' : 'transparent'
                  }}
                >
                  <Layout.Horizontal flex={{ justifyContent: 'space-between', alignItems: 'center' }}>
                    <Layout.Vertical spacing="xsmall">
                      <Text font={{ variation: FontVariation.BODY1 }} color={Color.GREY_900}>
                        {target.folder}
                      </Text>
                      <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>
                        {getString('appNameSpace')}: {target.namespace || '—'}
                      </Text>
                    </Layout.Vertical>
                    {isSelected && <Icon name="tick-circle" color={Color.PRIMARY_7} size={20} />}
                  </Layout.Horizontal>
                </Container>
              );
            })}
          </Container>
          <Layout.Vertical spacing="small" margin={{ top: 'medium', bottom: 'medium' }}>
            <Text font={{ variation: FontVariation.FORM_LABEL }}>{getString('uninstallTargetFolder')}</Text>
            <TextInput
              value={folder}
              onChange={(e: React.ChangeEvent<HTMLInputElement>) => setFolder(e.target.value)}
              placeholder={getString('uninstallTargetFolder')}
            />
            <Text font={{ variation: FontVariation.FORM_LABEL }}>{getString('appNameSpace')}</Text>
            <TextInput
              value={namespace}
              onChange={(e: React.ChangeEvent<HTMLInputElement>) => setNamespace(e.target.value)}
              placeholder={getString('selectAppNamespace')}
            />
          </Layout.Vertical>
          <Layout.Horizontal spacing="small">
            <Button
              variation={ButtonVariation.PRIMARY}
              text={getString('add')}
              disabled={submitting || folder.trim() === '' || namespace.trim() === ''}
              onClick={() => onSelect({ folder: folder.trim(), namespace: namespace.trim() })}
            />
            <Button variation={ButtonVariation.TERTIARY} text={getString('cancel')} onClick={onClose} />
          </Layout.Horizontal>
        </Layout.Vertical>
      }
    />
  );
}
