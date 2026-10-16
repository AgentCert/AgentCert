import React from 'react';
import cx from 'classnames';
import { Button, ButtonSize, ButtonVariation, FormInput, SelectOption, Text } from '@harnessio/uicore';
import { Color, FontVariation } from '@harnessio/design-system';
import { useFormikContext } from 'formik';
import { ConfigField, ConfigFieldType } from '@api/entities';
import { useStrings } from '@strings';
import {
  ChartSettingsFormValues,
  isSettingModified,
  PLATFORM_DEFAULT_MODEL,
  rangeHint,
  settingFieldName,
  toFormValues
} from './chartSettingsFormUtils';
import css from './ChartSettingsForm.module.scss';

interface ChartSettingsFormProps {
  fields: ConfigField[];
  // The platform's LLM models, offered by MODEL settings.
  modelOptions: SelectOption[];
  // What "Platform default" resolves to, shown with it so the choice is explicit.
  platformDefaultModelLabel?: string;
}

interface IndexedField {
  field: ConfigField;
  index: number;
}

interface SettingProps extends IndexedField {
  modelOptions: SelectOption[];
  platformDefaultModelLabel?: string;
  defaultValue: string | number | boolean;
}

function SettingInput({ field, index, modelOptions, platformDefaultModelLabel }: SettingProps): React.ReactElement {
  const { getString } = useStrings();
  const name = settingFieldName(index);

  switch (field.type) {
    case ConfigFieldType.INTEGER:
    case ConfigFieldType.NUMBER:
      return (
        <FormInput.Text
          name={name}
          placeholder={field.defaultValue}
          inputGroup={{
            type: 'number',
            min: field.min ?? undefined,
            max: field.max ?? undefined,
            step: field.type === ConfigFieldType.INTEGER ? 1 : 'any'
          }}
        />
      );
    case ConfigFieldType.TEXT:
      return <FormInput.TextArea name={name} placeholder={field.defaultValue} className={css.textArea} />;
    case ConfigFieldType.BOOLEAN:
      // The setting's label is already shown above the toggle.
      return <FormInput.Toggle name={name} label="" />;
    case ConfigFieldType.SELECT:
      return (
        <FormInput.Select name={name} items={(field.options ?? []).map(option => ({ label: option, value: option }))} />
      );
    case ConfigFieldType.MODEL: {
      const platformDefault = platformDefaultModelLabel
        ? `${getString('platformDefaultModel')} (${platformDefaultModelLabel})`
        : getString('platformDefaultModel');
      return (
        <FormInput.Select
          name={name}
          placeholder={platformDefault}
          items={[{ label: platformDefault, value: PLATFORM_DEFAULT_MODEL }, ...modelOptions]}
        />
      );
    }
    default:
      return <FormInput.Text name={name} placeholder={field.defaultValue} />;
  }
}

// One setting: its label, whether it differs from the chart default (with a
// one-click reset), the input, and its help text.
function Setting(props: SettingProps): React.ReactElement {
  const { field, index, defaultValue } = props;
  const { getString } = useStrings();
  const { values, setFieldValue } = useFormikContext<ChartSettingsFormValues>();
  const name = settingFieldName(index);
  const modified = isSettingModified(values[name], defaultValue);
  const hint = rangeHint(field);

  return (
    <div className={cx(css.setting, { [css.wide]: field.type === ConfigFieldType.TEXT })}>
      <div className={css.settingHeader}>
        <Text font={{ variation: FontVariation.FORM_LABEL }} color={Color.GREY_800} lineClamp={1}>
          {field.label}
          {field.required && <span className={css.required}>*</span>}
        </Text>
        {modified && (
          <Button
            className={css.reset}
            variation={ButtonVariation.LINK}
            size={ButtonSize.SMALL}
            icon="reset"
            text={getString('resetSetting')}
            tooltip={getString('resetToDefaultValue', { value: String(defaultValue) || '—' })}
            onClick={() => setFieldValue(name, defaultValue)}
          />
        )}
      </div>
      <SettingInput {...props} />
      {(field.description || hint) && (
        <Text font={{ variation: FontVariation.TINY }} color={Color.GREY_500} className={css.help}>
          {[field.description, hint && getString('allowedRange', { range: hint })].filter(Boolean).join(' · ')}
        </Text>
      )}
    </div>
  );
}

// Settings grouped under their headings, in the order the chart declares them.
function SettingGroups({
  items,
  defaults,
  ...shared
}: {
  items: IndexedField[];
  defaults: ChartSettingsFormValues;
  modelOptions: SelectOption[];
  platformDefaultModelLabel?: string;
}): React.ReactElement {
  const { getString } = useStrings();
  const groups = new Map<string, IndexedField[]>();
  items.forEach(item => {
    const group = item.field.group || getString('settingsGeneral');
    groups.set(group, [...(groups.get(group) ?? []), item]);
  });

  return (
    <>
      {Array.from(groups.entries()).map(([group, members]) => (
        <section key={group} className={css.group}>
          <Text font={{ variation: FontVariation.SMALL_SEMI }} color={Color.GREY_700} className={css.groupTitle}>
            {group}
          </Text>
          <div className={css.grid}>
            {members.map(({ field, index }) => (
              <Setting
                key={field.key}
                field={field}
                index={index}
                defaultValue={defaults[settingFieldName(index)]}
                {...shared}
              />
            ))}
          </div>
        </section>
      ))}
    </>
  );
}

// Renders a chart's declared settings as inputs of the enclosing Formik form.
// The settings come from the chart (its values.yaml `configurations` block),
// so this component knows nothing about any particular agent or application.
export default function ChartSettingsForm({
  fields,
  modelOptions,
  platformDefaultModelLabel
}: ChartSettingsFormProps): React.ReactElement {
  const { getString } = useStrings();
  const { values, errors, submitCount, setValues } = useFormikContext<ChartSettingsFormValues>();
  const [showAdvanced, setShowAdvanced] = React.useState(false);
  const defaults = React.useMemo(() => toFormValues(fields, {}), [fields]);

  if (fields.length === 0) {
    return (
      <div className={css.empty}>
        <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>
          {getString('noChartSettings')}
        </Text>
      </div>
    );
  }

  const indexed = fields.map((field, index) => ({ field, index }));
  const basic = indexed.filter(({ field }) => !field.advanced);
  const advanced = indexed.filter(({ field }) => field.advanced);
  const modifiedCount = indexed.filter(({ index }) =>
    isSettingModified(values[settingFieldName(index)], defaults[settingFieldName(index)])
  ).length;
  // A rejected submit must never hide the setting that caused it.
  const advancedVisible =
    showAdvanced || (submitCount > 0 && advanced.some(({ index }) => errors[settingFieldName(index)] !== undefined));

  return (
    <div className={css.form}>
      <div className={css.summary}>
        <Text font={{ variation: FontVariation.SMALL }} color={modifiedCount > 0 ? Color.PRIMARY_7 : Color.GREY_500}>
          {modifiedCount > 0
            ? getString('settingsChangedFromDefaults', { count: modifiedCount, total: fields.length })
            : getString('settingsAllDefaults', { total: fields.length })}
        </Text>
        {modifiedCount > 0 && (
          <Button
            variation={ButtonVariation.LINK}
            size={ButtonSize.SMALL}
            icon="reset"
            text={getString('resetAllToDefaults')}
            onClick={() => setValues(defaults)}
          />
        )}
      </div>
      <SettingGroups
        items={basic}
        defaults={defaults}
        modelOptions={modelOptions}
        platformDefaultModelLabel={platformDefaultModelLabel}
      />
      {advanced.length > 0 && (
        <section className={css.advanced}>
          <button
            type="button"
            className={css.advancedToggle}
            aria-expanded={advancedVisible}
            onClick={() => setShowAdvanced(!advancedVisible)}
          >
            <span className={cx(css.chevron, { [css.expanded]: advancedVisible })} aria-hidden />
            <Text font={{ variation: FontVariation.SMALL_SEMI }} color={Color.GREY_700}>
              {getString('advancedSettingsCount', { count: advanced.length })}
            </Text>
          </button>
          {advancedVisible && (
            <SettingGroups
              items={advanced}
              defaults={defaults}
              modelOptions={modelOptions}
              platformDefaultModelLabel={platformDefaultModelLabel}
            />
          )}
        </section>
      )}
    </div>
  );
}
