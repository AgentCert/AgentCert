import * as Yup from 'yup';
import { ConfigField, ConfigFieldType, ConfigValueKind } from '@api/entities';
import type { ConfigValues } from '@services/experiment/installStepSettings';
import type { UseStringsReturn } from '@strings';

export type ChartSettingsFormValues = Record<string, string | number | boolean>;

// Formik reads a dot in a field name as a path into nested values, and every
// setting key is a dotted Helm path, so form fields are named by position.
export const settingFieldName = (index: number): string => `setting${index}`;

// A model setting left on this value is not written, so the platform's
// default model applies, exactly as for an experiment without settings.
export const PLATFORM_DEFAULT_MODEL = '';

// The form's starting values: what the step already sets, else the chart default.
export function toFormValues(fields: ConfigField[], current: ConfigValues): ChartSettingsFormValues {
  const values: ChartSettingsFormValues = {};
  fields.forEach((field, index) => {
    let value: string | number | boolean = field.defaultValue;
    if (Object.prototype.hasOwnProperty.call(current, field.key)) value = current[field.key];
    else if (field.type === ConfigFieldType.MODEL) value = PLATFORM_DEFAULT_MODEL;

    values[settingFieldName(index)] =
      field.type === ConfigFieldType.BOOLEAN ? value === true || value === 'true' : value;
  });
  return values;
}

// Whether a form value differs from the setting's default. Inputs hand back
// numbers or strings for the same setting, so values are compared as text.
export function isSettingModified(
  value: string | number | boolean | undefined,
  defaultValue: string | number | boolean | undefined
): boolean {
  return String(value ?? '') !== String(defaultValue ?? '');
}

// "1 – 5", "≥ 1" or "≤ 5" for a number setting with bounds, else undefined.
export function rangeHint(field: ConfigField): string | undefined {
  const hasMin = field.min !== null && field.min !== undefined;
  const hasMax = field.max !== null && field.max !== undefined;
  if (hasMin && hasMax) return `${field.min} – ${field.max}`;
  if (hasMin) return `≥ ${field.min}`;
  if (hasMax) return `≤ ${field.max}`;
  return undefined;
}

// The settings to write to the step, typed as the chart's defaults are. Every
// filled-in setting is written, not only the changed ones, so a saved
// experiment keeps running the same configuration if a chart default changes.
// Empty numeric inputs keep the chart default. Empty text is a chosen value.
export function fromFormValues(fields: ConfigField[], form: ChartSettingsFormValues): ConfigValues {
  const values: ConfigValues = {};
  fields.forEach((field, index) => {
    let raw = form[settingFieldName(index)];
    if (raw === undefined || raw === null) return;
    if (field.type === ConfigFieldType.MODEL && raw === PLATFORM_DEFAULT_MODEL) return;
    if (raw === '' && (field.type === ConfigFieldType.INTEGER || field.type === ConfigFieldType.NUMBER)) {
      raw = field.defaultValue;
    }

    switch (field.valueKind) {
      case ConfigValueKind.BOOLEAN:
        values[field.key] = raw === true || raw === 'true';
        break;
      case ConfigValueKind.INTEGER:
      case ConfigValueKind.NUMBER:
        values[field.key] = Number(raw);
        break;
      default:
        values[field.key] =
          field.type === ConfigFieldType.INTEGER || field.type === ConfigFieldType.NUMBER
            ? String(Number(raw))
            : String(raw);
    }
  });
  return values;
}

function numberSchema(field: ConfigField, getString: UseStringsReturn['getString']): Yup.NumberSchema {
  let schema = Yup.number()
    // An emptied input is "use the chart default", not "not a number".
    .transform((value, original) =>
      original === '' || original === null ? undefined : Number.isFinite(value) ? value : NaN
    )
    .typeError(getString('settingMustBeANumber'));
  if (field.type === ConfigFieldType.INTEGER || field.valueKind === ConfigValueKind.INTEGER) {
    schema = schema.integer(getString('settingMustBeAWholeNumber'));
  }
  if (field.min !== null && field.min !== undefined) {
    schema = schema.min(field.min, getString('settingMustBeAtLeast', { value: field.min }));
  }
  if (field.max !== null && field.max !== undefined) {
    schema = schema.max(field.max, getString('settingMustBeAtMost', { value: field.max }));
  }
  return field.required ? schema.required(getString('required')) : schema;
}

function textSchema(field: ConfigField, getString: UseStringsReturn['getString']): Yup.StringSchema {
  let schema = Yup.string()
    // Argo would expand it as a workflow template expression.
    .test('no-template-expression', getString('settingMustNotContainBraces'), value => !value?.includes('{{'))
    .test('required', getString('required'), value => !field.required || Boolean(value?.trim()));
  if (field.pattern) {
    schema = schema.matches(new RegExp(field.pattern), {
      message: getString('settingMustMatch', { pattern: field.pattern }),
      excludeEmptyString: false
    });
  }
  if (field.type === ConfigFieldType.SELECT && field.options) {
    schema = schema.oneOf(field.options, getString('invalidSelection'));
  }
  return schema;
}

// Client-side checks matching the server's (pkg/chartconfig), so a value the
// server would reject is caught in the form. The server remains the authority.
export function buildSettingsSchema(
  fields: ConfigField[],
  getString: UseStringsReturn['getString']
): Yup.AnyObjectSchema {
  const shape: Record<string, Yup.AnySchema> = {};
  fields.forEach((field, index) => {
    const name = settingFieldName(index);
    switch (field.type) {
      case ConfigFieldType.INTEGER:
      case ConfigFieldType.NUMBER:
        shape[name] = numberSchema(field, getString);
        break;
      case ConfigFieldType.BOOLEAN:
        shape[name] = Yup.boolean();
        break;
      case ConfigFieldType.MODEL:
        shape[name] = Yup.string();
        break;
      default:
        shape[name] = textSchema(field, getString);
    }
  });
  return Yup.object().shape(shape);
}
