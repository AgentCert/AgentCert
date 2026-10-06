import React from 'react';
import cx from 'classnames';
import { Button, ButtonVariation, SelectOption, Text, TextInput } from '@harnessio/uicore';
import { Color, FontVariation } from '@harnessio/design-system';
import { Formik, FormikProps } from 'formik';
import Drawer from '@components/Drawer';
import { DrawerTypes } from '@components/Drawer/Drawer';
import ChartSettingsForm, {
  buildSettingsSchema,
  ChartSettingsFormValues,
  fromFormValues,
  toFormValues
} from '@components/ChartSettingsForm';
import type { ConfigField } from '@api/entities';
import {
  InstallStepChoice,
  InstallStepSelection,
  readInstallStepSettings
} from '@services/experiment/installStepSettings';
import { useStrings } from '@strings';
import css from './ExperimentCreationSelectInstallStep.module.scss';

export interface InstallStepEntry {
  folder: string;
  displayName: string;
  description?: string;
  namespace?: string;
  // Settings the entry's chart offers (its values.yaml `configurations`).
  configurations: ConfigField[];
}

interface ExperimentCreationSelectInstallStepViewProps {
  isOpen: boolean;
  kind: 'application' | 'agent';
  loading: boolean;
  entries: InstallStepEntry[];
  initialSelection?: InstallStepSelection;
  // The platform's LLM models, offered by an agent's model setting.
  modelOptions: SelectOption[];
  // What the platform's default model is, shown with "Platform default".
  platformDefaultModelLabel?: string;
  onSelect: (choice: InstallStepChoice) => void;
  onClose: () => void;
}

function SectionHeader({
  step,
  title,
  subtitle,
  action
}: {
  step: number;
  title: string;
  subtitle?: string;
  action?: React.ReactNode;
}): React.ReactElement {
  return (
    <div className={css.sectionHeader}>
      <span className={css.step}>{step}</span>
      <div className={css.sectionText}>
        <Text font={{ variation: FontVariation.H6 }} color={Color.GREY_900}>
          {title}
        </Text>
        {subtitle && (
          <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500} margin={{ top: 'xsmall' }}>
            {subtitle}
          </Text>
        )}
      </div>
      {action}
    </div>
  );
}

function EntryCard({
  entry,
  selected,
  onClick
}: {
  entry: InstallStepEntry;
  selected: boolean;
  onClick: () => void;
}): React.ReactElement {
  const { getString } = useStrings();
  return (
    <button
      type="button"
      className={cx(css.entry, { [css.selected]: selected })}
      onClick={onClick}
      aria-pressed={selected}
    >
      <div className={css.entryHeader}>
        <Text font={{ variation: FontVariation.BODY1 }} color={Color.GREY_900} lineClamp={1}>
          {entry.displayName}
        </Text>
        {selected && <span className={css.check} aria-hidden />}
      </div>
      {entry.description && (
        <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500} lineClamp={2}>
          {entry.description}
        </Text>
      )}
      <div className={css.entryMeta}>
        <span className={css.chip}>{entry.folder}</span>
        <Text font={{ variation: FontVariation.TINY }} color={Color.GREY_500}>
          {getString('settingsCount', { count: entry.configurations.length })}
        </Text>
      </div>
    </button>
  );
}

export default function ExperimentCreationSelectInstallStepView({
  isOpen,
  kind,
  loading,
  entries,
  initialSelection,
  modelOptions,
  platformDefaultModelLabel,
  onSelect,
  onClose
}: ExperimentCreationSelectInstallStepViewProps): React.ReactElement {
  const { getString } = useStrings();
  const [selectedFolder, setSelectedFolder] = React.useState<string>(initialSelection?.folder ?? '');
  const [namespace, setNamespace] = React.useState<string>(initialSelection?.namespace ?? '');
  // Editing an installed step opens on its settings; adding one starts with the catalogue.
  const [isChoosing, setIsChoosing] = React.useState<boolean>(!initialSelection);
  const settingsFormRef = React.useRef<FormikProps<ChartSettingsFormValues>>();

  const selectedEntry = entries.find(entry => entry.folder === selectedFolder);
  const fields = React.useMemo(() => selectedEntry?.configurations ?? [], [selectedEntry]);

  // The step's current settings apply only to the chart it already installs;
  // choosing another chart starts from that chart's defaults.
  const initialSettings = React.useMemo(
    () =>
      toFormValues(
        fields,
        selectedFolder === initialSelection?.folder ? readInstallStepSettings(fields, initialSelection.source) : {}
      ),
    [fields, selectedFolder, initialSelection]
  );
  const settingsSchema = React.useMemo(() => buildSettingsSchema(fields, getString), [fields, getString]);

  const handleSelectEntry = (entry: InstallStepEntry): void => {
    if (entry.folder !== selectedFolder) {
      setSelectedFolder(entry.folder);
      setNamespace(entry.namespace ?? '');
    }
    setIsChoosing(false);
  };

  const handleSubmit = (values: ChartSettingsFormValues): void => {
    if (!selectedEntry) return;
    onSelect({
      folder: selectedEntry.folder,
      namespace: namespace.trim(),
      settings: { values: fromFormValues(fields, values), declaredKeys: fields.map(field => field.key) }
    });
  };

  const isApplication = kind === 'application';

  return (
    <Drawer
      isOpen={isOpen}
      handleClose={onClose}
      title={
        <Text font={{ variation: FontVariation.H5 }}>
          {isApplication ? getString('installApplication') : getString('installAgent')}
        </Text>
      }
      type={DrawerTypes.InstallStep}
      leftPanel={
        <div className={css.panel}>
          <div className={css.body}>
            <section className={css.section}>
              <SectionHeader
                step={1}
                title={isApplication ? getString('installStepChooseApplication') : getString('installStepChooseAgent')}
                subtitle={
                  isApplication ? getString('installApplicationDescription') : getString('installAgentDescription')
                }
                action={
                  !isChoosing &&
                  entries.length > 1 && (
                    <Button
                      variation={ButtonVariation.SECONDARY}
                      text={getString('changeSelection')}
                      onClick={() => setIsChoosing(true)}
                    />
                  )
                }
              />
              {loading && entries.length === 0 && (
                <Text font={{ variation: FontVariation.BODY }}>{getString('loading')}</Text>
              )}
              {!loading && entries.length === 0 && (
                <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_500}>
                  {getString('noEntriesFound')}
                </Text>
              )}
              <div className={css.entries}>
                {(isChoosing ? entries : entries.filter(entry => entry.folder === selectedFolder)).map(entry => (
                  <EntryCard
                    key={entry.folder}
                    entry={entry}
                    selected={entry.folder === selectedFolder}
                    onClick={() => (isChoosing ? handleSelectEntry(entry) : setIsChoosing(true))}
                  />
                ))}
              </div>
            </section>

            {/* Hidden rather than unmounted while choosing, so edits survive re-picking the same chart. */}
            {selectedEntry && (
              <div className={css.section} style={isChoosing ? { display: 'none' } : undefined}>
                <section className={css.section}>
                  <SectionHeader
                    step={2}
                    title={getString('namespace')}
                    subtitle={getString('installStepNamespaceHelp')}
                  />
                  <TextInput
                    value={namespace}
                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => setNamespace(e.target.value)}
                    placeholder={getString('selectAppNamespace')}
                  />
                </section>
                <section className={css.section}>
                  <SectionHeader
                    step={3}
                    title={getString('settings')}
                    subtitle={getString('chartSettingsDescription')}
                  />
                  {/* Keyed by chart, so switching charts resets the form to the new chart's settings. */}
                  <Formik<ChartSettingsFormValues>
                    key={selectedEntry.folder}
                    innerRef={settingsFormRef as React.Ref<FormikProps<ChartSettingsFormValues>>}
                    initialValues={initialSettings}
                    validationSchema={settingsSchema}
                    onSubmit={handleSubmit}
                  >
                    <ChartSettingsForm
                      fields={fields}
                      modelOptions={modelOptions}
                      platformDefaultModelLabel={platformDefaultModelLabel}
                    />
                  </Formik>
                </section>
              </div>
            )}
          </div>
          <div className={css.footer}>
            <Button
              variation={ButtonVariation.PRIMARY}
              text={initialSelection ? getString('apply') : getString('add')}
              disabled={!selectedEntry || isChoosing || namespace.trim() === ''}
              onClick={() => settingsFormRef.current?.submitForm()}
            />
            <Button variation={ButtonVariation.TERTIARY} text={getString('cancel')} onClick={onClose} />
          </div>
        </div>
      }
    />
  );
}
