import React from 'react';
import { Button, ButtonVariation, Container, Layout, Text, TextInput } from '@harnessio/uicore';
import { Color, FontVariation } from '@harnessio/design-system';
import { Icon } from '@harnessio/icons';
import Drawer from '@components/Drawer';
import { DrawerTypes } from '@components/Drawer/Drawer';
import { useStrings } from '@strings';

export interface InstallStepEntry {
  folder: string;
  displayName: string;
  description?: string;
  namespace?: string;
}

interface ExperimentCreationSelectInstallStepViewProps {
  isOpen: boolean;
  kind: 'application' | 'agent';
  loading: boolean;
  entries: InstallStepEntry[];
  onSelect: (entry: { folder: string; namespace: string }) => void;
  onClose: () => void;
}

export default function ExperimentCreationSelectInstallStepView({
  isOpen,
  kind,
  loading,
  entries,
  onSelect,
  onClose
}: ExperimentCreationSelectInstallStepViewProps): React.ReactElement {
  const { getString } = useStrings();
  const [selectedFolder, setSelectedFolder] = React.useState<string>('');
  const [namespace, setNamespace] = React.useState<string>('');

  const selectedEntry = entries.find(entry => entry.folder === selectedFolder);

  const handleSelectEntry = (entry: InstallStepEntry): void => {
    setSelectedFolder(entry.folder);
    setNamespace(entry.namespace ?? '');
  };

  const title = (
    <Text font={{ variation: FontVariation.H5 }}>
      {kind === 'application' ? getString('installApplication') : getString('installAgent')}
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
            {kind === 'application' ? getString('installApplicationDescription') : getString('installAgentDescription')}
          </Text>
          <Container style={{ flexGrow: 1, overflowY: 'auto' }}>
            {loading && <Text font={{ variation: FontVariation.BODY }}>{getString('loading')}</Text>}
            {!loading && entries.length === 0 && (
              <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_500}>
                {getString('noEntriesFound')}
              </Text>
            )}
            {entries.map(entry => {
              const isSelected = entry.folder === selectedFolder;
              return (
                <Container
                  key={entry.folder}
                  onClick={() => handleSelectEntry(entry)}
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
                        {entry.displayName}
                      </Text>
                      <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>
                        {entry.description}
                      </Text>
                      <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_400}>
                        {getString('folder')}: {entry.folder}
                      </Text>
                    </Layout.Vertical>
                    {isSelected && <Icon name="tick-circle" color={Color.PRIMARY_7} size={20} />}
                  </Layout.Horizontal>
                </Container>
              );
            })}
          </Container>
          {selectedEntry && (
            <Layout.Vertical spacing="small" margin={{ top: 'medium', bottom: 'medium' }}>
              <Text font={{ variation: FontVariation.FORM_LABEL }}>{getString('appNameSpace')}</Text>
              <TextInput
                value={namespace}
                onChange={(e: React.ChangeEvent<HTMLInputElement>) => setNamespace(e.target.value)}
                placeholder={getString('selectAppNamespace')}
              />
            </Layout.Vertical>
          )}
          <Layout.Horizontal spacing="small">
            <Button
              variation={ButtonVariation.PRIMARY}
              text={getString('add')}
              disabled={!selectedEntry || namespace.trim() === ''}
              onClick={() => selectedEntry && onSelect({ folder: selectedEntry.folder, namespace: namespace.trim() })}
            />
            <Button variation={ButtonVariation.TERTIARY} text={getString('cancel')} onClick={onClose} />
          </Layout.Horizontal>
        </Layout.Vertical>
      }
    />
  );
}
