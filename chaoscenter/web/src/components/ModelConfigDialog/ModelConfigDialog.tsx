import React, { useState } from 'react';
import { Formik, Form } from 'formik';
import {
  Dialog,
  Layout,
  Text,
  Button,
  ButtonVariation,
  Container,
  FormInput,
  SelectOption,
  useToaster
} from '@harnessio/uicore';
import { Color, FontVariation } from '@harnessio/design-system';
import type { ModelConfig } from '@api/core/modelLibrary';
import { useCreateModelConfig, useTestModelConfig } from '@api/core/modelLibrary';
import type { ModelConfigTestResult } from '@api/core/modelLibrary';

const PROVIDER_OPTIONS: SelectOption[] = [
  { label: 'OpenAI', value: 'openai' },
  { label: 'Anthropic', value: 'anthropic' },
  { label: 'Google', value: 'google' },
  { label: 'Azure OpenAI', value: 'azure' },
  { label: 'Ollama (local)', value: 'ollama' },
  { label: 'Custom', value: 'custom' },
];

const PROVIDER_MODELS: Record<string, SelectOption[]> = {
  openai: [
    { label: 'gpt-4o', value: 'gpt-4o' },
    { label: 'gpt-4o-mini', value: 'gpt-4o-mini' },
    { label: 'gpt-4-turbo', value: 'gpt-4-turbo' },
    { label: 'o1', value: 'o1' },
    { label: 'o1-mini', value: 'o1-mini' },
  ],
  anthropic: [
    { label: 'claude-sonnet-4-6', value: 'claude-sonnet-4-6' },
    { label: 'claude-opus-4-8', value: 'claude-opus-4-8' },
    { label: 'claude-haiku-4-5', value: 'claude-haiku-4-5' },
  ],
  google: [
    { label: 'gemini-2.0-flash', value: 'gemini-2.0-flash' },
    { label: 'gemini-1.5-pro', value: 'gemini-1.5-pro' },
    { label: 'gemini-1.5-flash', value: 'gemini-1.5-flash' },
  ],
  azure: [],
  ollama: [],
  custom: [],
};

export interface ModelConfigDialogProps {
  isOpen: boolean;
  onClose: () => void;
  onSaved: (config: ModelConfig) => void;
  projectID: string;
  mode?: 'add' | 'edit';
}

interface FormValues {
  alias: string;
  provider: string;
  model: string;
  customModel: string;
  apiKey: string;
  baseURL: string;
}

export const ModelConfigDialog: React.FC<ModelConfigDialogProps> = ({
  isOpen,
  onClose,
  onSaved,
  projectID,
  mode = 'add',
}) => {
  const { showSuccess, showError } = useToaster();
  const [testResult, setTestResult] = useState<ModelConfigTestResult | null>(null);
  const [isTested, setIsTested] = useState(false);

  const [createModelConfig, { loading: creating }] = useCreateModelConfig();
  const [testModelConfig, { loading: testing }] = useTestModelConfig();

  const requiresBaseURL = (provider: string) =>
    ['azure', 'ollama', 'custom'].includes(provider);

  const getModelOptions = (provider: string): SelectOption[] =>
    PROVIDER_MODELS[provider] ?? [];

  const handleTest = async (values: FormValues) => {
    setTestResult(null);
    const model = values.customModel || values.model;
    try {
      const result = await testModelConfig({
        variables: {
          input: {
            alias: values.alias || 'test',
            provider: values.provider,
            model,
            apiKey: values.apiKey,
            baseURL: values.baseURL || undefined,
          },
        },
      });
      const r = result.data?.testModelConfig;
      if (r) {
        setTestResult(r);
        if (r.success) setIsTested(true);
      }
    } catch (e: unknown) {
      showError((e as Error).message);
    }
  };

  const handleSave = async (values: FormValues) => {
    const model = values.customModel || values.model;
    try {
      const result = await createModelConfig({
        variables: {
          projectID,
          input: {
            alias: values.alias,
            provider: values.provider,
            model,
            apiKey: values.apiKey,
            baseURL: values.baseURL || undefined,
          },
        },
      });
      if (result.data?.createModelConfig.config) {
        showSuccess('Model config saved successfully');
        onSaved(result.data.createModelConfig.config);
        onClose();
      }
    } catch (e: unknown) {
      showError((e as Error).message);
    }
  };

  const initialValues: FormValues = {
    alias: '',
    provider: 'openai',
    model: 'gpt-4o',
    customModel: '',
    apiKey: '',
    baseURL: '',
  };

  return (
    <Dialog
      isOpen={isOpen}
      onClose={onClose}
      title={mode === 'add' ? 'Add Model Config' : 'Edit Model Config'}
      style={{ width: 520 }}
    >
      <Formik<FormValues>
        initialValues={initialValues}
        onSubmit={handleSave}
      >
        {({ values, setFieldValue }) => (
          <Form>
            <Layout.Vertical spacing="medium" padding={{ left: 'medium', right: 'medium', bottom: 'medium' }}>
              <FormInput.Text
                name="alias"
                label="Alias"
                placeholder="e.g. my-openai-gpt4o"
                disabled={mode === 'edit'}
              />
              <FormInput.Select
                name="provider"
                label="Provider"
                items={PROVIDER_OPTIONS}
                onChange={(opt) => {
                  setFieldValue('provider', opt.value);
                  setFieldValue('model', '');
                  setFieldValue('customModel', '');
                  setIsTested(false);
                  setTestResult(null);
                }}
              />
              {getModelOptions(values.provider).length > 0 && (
                <FormInput.Select
                  name="model"
                  label="Model"
                  items={[
                    ...getModelOptions(values.provider),
                    { label: 'Custom...', value: '__custom__' },
                  ]}
                  onChange={() => {
                    setIsTested(false);
                    setTestResult(null);
                  }}
                />
              )}
              {(values.model === '__custom__' || getModelOptions(values.provider).length === 0) && (
                <FormInput.Text
                  name="customModel"
                  label="Model name"
                  placeholder="Enter model name"
                />
              )}
              <FormInput.Text
                name="apiKey"
                label="API Key"
                inputGroup={{ type: 'password' }}
                placeholder="sk-..."
              />
              {requiresBaseURL(values.provider) && (
                <FormInput.Text
                  name="baseURL"
                  label="Base URL"
                  placeholder="https://..."
                />
              )}
              {testResult && (
                <Container>
                  {testResult.success ? (
                    <Text color={Color.GREEN_600} font={{ variation: FontVariation.SMALL }}>
                      Connected in {testResult.latencyMs}ms
                    </Text>
                  ) : (
                    <Text color={Color.RED_600} font={{ variation: FontVariation.SMALL }}>
                      {testResult.errorMessage}
                    </Text>
                  )}
                </Container>
              )}
              <Layout.Horizontal spacing="small" style={{ justifyContent: 'space-between' }}>
                <Button
                  text={testing ? 'Testing...' : 'Test Connection'}
                  variation={ButtonVariation.SECONDARY}
                  loading={testing}
                  onClick={() => handleTest(values)}
                />
                <Layout.Horizontal spacing="small">
                  <Button
                    text="Cancel"
                    variation={ButtonVariation.TERTIARY}
                    onClick={onClose}
                  />
                  <Button
                    text={creating ? 'Saving...' : 'Save'}
                    variation={ButtonVariation.PRIMARY}
                    loading={creating}
                    disabled={!isTested}
                    type="submit"
                  />
                </Layout.Horizontal>
              </Layout.Horizontal>
            </Layout.Vertical>
          </Form>
        )}
      </Formik>
    </Dialog>
  );
};

export default ModelConfigDialog;
