import React from 'react';
import '@testing-library/jest-dom';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { TestWrapper } from 'utils/testUtils';
import ModelConfigForm from '../ModelConfigForm';
import { connectionKey, INITIAL_MODEL_FORM_VALUES } from '../modelConfigUtils';

function renderForm(props: Partial<React.ComponentProps<typeof ModelConfigForm>> = {}): Record<string, jest.Mock> {
  const handlers = { onTest: jest.fn(), onSubmit: jest.fn(), onClose: jest.fn() };
  render(
    <TestWrapper>
      <ModelConfigForm saving={false} testing={false} {...handlers} {...props} />
    </TestWrapper>
  );
  return handlers;
}

const input = (name: string): HTMLInputElement => document.querySelector(`input[name="${name}"]`) as HTMLInputElement;

describe('ModelConfigForm', () => {
  test('asks Azure OpenAI for a version and requires its fields', async () => {
    const { onSubmit } = renderForm();
    expect(input('apiVersion')).toBeInTheDocument();
    fireEvent.click(screen.getByTestId('save-model'));
    await waitFor(() => expect(screen.getAllByText('required').length).toBeGreaterThanOrEqual(4));
    expect(onSubmit).not.toHaveBeenCalled();
  });

  test('proposes an alias from the model name until one is typed', async () => {
    renderForm();
    fireEvent.change(input('model'), { target: { value: 'qwen2.5:32b-instruct' } });
    await waitFor(() => expect(input('alias').value).toBe('qwen2.5-32b-instruct'));
    fireEvent.change(input('alias'), { target: { value: 'my-alias' } });
    fireEvent.change(input('model'), { target: { value: 'gpt-4o' } });
    await waitFor(() => expect(input('alias').value).toBe('my-alias'));
  });

  test('tests the connection as entered, and only once there is a model', async () => {
    const { onTest } = renderForm();
    expect(screen.getByTestId('test-model')).toBeDisabled();
    fireEvent.change(input('model'), { target: { value: 'gpt-4o' } });
    await waitFor(() => expect(screen.getByTestId('test-model')).not.toBeDisabled());
    fireEvent.click(screen.getByTestId('test-model'));
    expect(onTest).toHaveBeenCalledWith(expect.objectContaining({ model: 'gpt-4o', provider: 'AZURE_OPENAI' }));
  });

  test('submits a complete model', async () => {
    const { onSubmit } = renderForm();
    fireEvent.change(input('model'), { target: { value: 'gpt-4o' } });
    fireEvent.change(input('apiBase'), { target: { value: 'https://acme.openai.azure.com' } });
    fireEvent.change(input('apiVersion'), { target: { value: '2024-08-01-preview' } });
    fireEvent.change(input('apiKey'), { target: { value: 'az-key' } });
    fireEvent.click(screen.getByTestId('save-model'));
    await waitFor(() =>
      expect(onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ alias: 'gpt-4o', apiKey: 'az-key', apiVersion: '2024-08-01-preview' }),
        expect.anything()
      )
    );
  });

  test('shows a failed test with its reason, and offers to save the model anyway', () => {
    renderForm({
      lastTest: {
        connection: connectionKey(INITIAL_MODEL_FORM_VALUES),
        result: { ok: false, error: 'AuthenticationError: invalid key' }
      }
    });
    expect(screen.getByTestId('model-test-failed')).toHaveTextContent('AuthenticationError: invalid key');
    expect(screen.getByTestId('save-model')).toHaveTextContent('modelSaveAnyway');
  });

  test('drops a test result once the values it was run on change', async () => {
    renderForm({
      lastTest: {
        connection: connectionKey(INITIAL_MODEL_FORM_VALUES),
        result: { ok: false, error: 'DeploymentNotFound' }
      }
    });
    fireEvent.change(input('model'), { target: { value: 'gpt4o' } });
    await waitFor(() => expect(screen.queryByTestId('model-test-failed')).not.toBeInTheDocument());
    expect(screen.getByTestId('save-model')).toHaveTextContent('save');
  });

  test('keeps saved logins out of the form', () => {
    renderForm();
    expect(document.querySelector('form')).toHaveAttribute('autocomplete', 'off');
    expect(input('apiKey')).toHaveAttribute('autocomplete', 'new-password');
  });

  test('asks Azure OpenAI for its deployment name', () => {
    renderForm();
    expect(screen.getByText('modelDeploymentName')).toBeInTheDocument();
  });
});
