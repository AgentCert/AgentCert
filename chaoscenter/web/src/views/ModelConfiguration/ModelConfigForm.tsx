import React from 'react';
import { Form, Formik, FormikProps } from 'formik';
import * as Yup from 'yup';
import { Button, ButtonVariation, FormInput, Layout, Text } from '@harnessio/uicore';
import { Color, FontVariation } from '@harnessio/design-system';
import type { ModelProvider } from '@api/entities';
import PasswordInput from '@components/PasswordInput';
import { useStrings } from '@strings';
import ModelTestResultView from './ModelTestResult';
import {
  ALIAS_PATTERN,
  ConnectionTest,
  connectionKey,
  INITIAL_MODEL_FORM_VALUES,
  ModelFormValues,
  PROVIDER_OPTIONS,
  PROVIDER_RULES,
  suggestAlias
} from './modelConfigUtils';
import css from './ModelConfiguration.module.scss';

interface ModelConfigFormProps {
  saving: boolean;
  testing: boolean;
  lastTest?: ConnectionTest;
  onTest: (values: ModelFormValues) => void;
  onSubmit: (values: ModelFormValues) => void;
  onClose: () => void;
}

function useModelSchema(): Yup.AnyObjectSchema {
  const { getString } = useStrings();
  const requiredFor =
    (field: 'apiBaseRequired' | 'needsVersion' | 'needsKey') => (provider: ModelProvider, schema: Yup.StringSchema) =>
      PROVIDER_RULES[provider]?.[field] ? schema.required(getString('required')) : schema;
  return Yup.object().shape({
    alias: Yup.string().trim().required(getString('required')).matches(ALIAS_PATTERN, getString('modelAliasInvalid')),
    model: Yup.string().trim().required(getString('required')),
    apiBase: Yup.string()
      .trim()
      .matches(/^https?:\/\/\S+$/i, { message: getString('modelApiBaseInvalid'), excludeEmptyString: true })
      .when('provider', requiredFor('apiBaseRequired')),
    apiVersion: Yup.string().trim().when('provider', requiredFor('needsVersion')),
    apiKey: Yup.string().trim().when('provider', requiredFor('needsKey'))
  });
}

// The Add Model form. Its provider decides which fields it asks for. Save
// tests the model first (see CreateModelConfigController); a model whose test
// failed is saved only through Save anyway.
export default function ModelConfigForm({
  saving,
  testing,
  lastTest,
  onTest,
  onSubmit,
  onClose
}: ModelConfigFormProps): React.ReactElement {
  const { getString } = useStrings();
  const schema = useModelSchema();
  // The alias follows the model name until the user types one.
  const aliasEdited = React.useRef(false);

  return (
    <Formik<ModelFormValues> initialValues={INITIAL_MODEL_FORM_VALUES} validationSchema={schema} onSubmit={onSubmit}>
      {(formik: FormikProps<ModelFormValues>) => {
        const rule = PROVIDER_RULES[formik.values.provider];
        const testResult = lastTest?.connection === connectionKey(formik.values) ? lastTest.result : undefined;
        return (
          // Browsers take a form with a password field for a login and fill
          // the saved username into one of its text fields.
          <Form autoComplete="off">
            <Layout.Vertical spacing="small">
              <Text font={{ variation: FontVariation.H4 }} color={Color.GREY_800}>
                {getString('addModel')}
              </Text>
              <Text color={Color.GREY_600} margin={{ bottom: 'medium' }}>
                {getString('llmModelsDescription')}
              </Text>
              <FormInput.Select name="provider" label={getString('modelProvider')} items={PROVIDER_OPTIONS} />
              <FormInput.Text
                name="model"
                label={getString(rule.modelLabel)}
                placeholder={rule.modelPlaceholder}
                className={css.formInput}
                onChange={event => {
                  if (!aliasEdited.current) {
                    formik.setFieldValue('alias', suggestAlias((event.target as HTMLInputElement).value));
                  }
                }}
              />
              <FormInput.Text
                name="alias"
                label={getString('modelAlias')}
                placeholder="gpt-4o-bench"
                helperText={getString('modelAliasHelp')}
                className={css.formInput}
                onChange={() => {
                  aliasEdited.current = true;
                }}
              />
              <FormInput.Text
                name="label"
                label={getString('modelDisplayName')}
                placeholder={formik.values.alias}
                className={css.formInput}
              />
              <FormInput.Text
                name="apiBase"
                label={getString('modelApiBase')}
                placeholder={rule.apiBasePlaceholder}
                className={css.formInput}
              />
              {rule.needsVersion && (
                <FormInput.Text
                  name="apiVersion"
                  label={getString('modelApiVersion')}
                  placeholder="2024-08-01-preview"
                  className={css.formInput}
                />
              )}
              <PasswordInput
                name="apiKey"
                label={rule.needsKey ? getString('modelApiKey') : getString('modelApiKeyOptional')}
                placeholder="sk-..."
                autoComplete="new-password"
              />
              <Text color={Color.GREY_500} font={{ size: 'small' }} margin={{ bottom: 'medium' }}>
                {getString('modelApiKeyHelp')}
              </Text>
              <ModelTestResultView testing={testing} result={testResult} />
              <Layout.Horizontal spacing="medium" margin={{ top: 'medium' }}>
                <Button
                  variation={ButtonVariation.PRIMARY}
                  type="submit"
                  text={testResult?.ok === false ? getString('modelSaveAnyway') : getString('save')}
                  disabled={saving || testing}
                  loading={saving}
                  data-testid="save-model"
                />
                <Button
                  variation={ButtonVariation.SECONDARY}
                  text={getString('modelTest')}
                  disabled={saving || testing || !formik.values.model.trim()}
                  onClick={() => onTest(formik.values)}
                  data-testid="test-model"
                />
                <Button variation={ButtonVariation.TERTIARY} text={getString('cancel')} onClick={onClose} />
              </Layout.Horizontal>
            </Layout.Vertical>
          </Form>
        );
      }}
    </Formik>
  );
}
