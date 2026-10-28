import React from 'react';
import { act, render, waitFor } from '@testing-library/react';
import { createModelConfig, testModelConnection } from '@api/core';
import type { ModelTestResult } from '@api/entities';
import ModelConfigForm from '@views/ModelConfiguration/ModelConfigForm';
import { INITIAL_MODEL_FORM_VALUES, ModelFormValues } from '@views/ModelConfiguration/modelConfigUtils';
import CreateModelConfigController from '../CreateModelConfig';

jest.mock('@api/core', () => ({ createModelConfig: jest.fn(), testModelConnection: jest.fn() }));
jest.mock('@views/ModelConfiguration/ModelConfigForm', () => ({ __esModule: true, default: jest.fn(() => null) }));
jest.mock('@utils', () => ({ getScope: () => ({ projectID: 'project-1' }) }));
jest.mock('@strings', () => ({ useStrings: () => ({ getString: (key: string) => key }) }));
jest.mock('@harnessio/uicore', () => ({ useToaster: () => ({ showSuccess: jest.fn(), showError: jest.fn() }) }));

const azure: ModelFormValues = {
  ...INITIAL_MODEL_FORM_VALUES,
  alias: 'gpt-4o-bench',
  model: 'gpt4o',
  apiBase: 'https://acme.openai.azure.com',
  apiVersion: '2024-12-01-preview',
  apiKey: 'az-key'
};

// Each test answers the connection tests with these results, in order.
function setup(...results: Array<ModelTestResult | Error>): Record<string, jest.Mock> {
  const testMutation = jest.fn();
  for (const result of results) {
    if (result instanceof Error) testMutation.mockRejectedValueOnce(result);
    else testMutation.mockResolvedValueOnce({ data: { testModelConnection: result } });
  }
  const createMutation = jest.fn().mockResolvedValue({ data: { createModelConfig: { alias: azure.alias } } });
  (testModelConnection as jest.Mock).mockReturnValue([testMutation, { loading: false }]);
  (createModelConfig as jest.Mock).mockReturnValue([createMutation, { loading: false }]);
  const onClose = jest.fn();
  render(<CreateModelConfigController onClose={onClose} refetchModels={jest.fn()} />);
  return { testMutation, createMutation, onClose };
}

const form = (): React.ComponentProps<typeof ModelConfigForm> => {
  const calls = (ModelConfigForm as unknown as jest.Mock).mock.calls;
  return calls[calls.length - 1][0];
};

const press = async (action: 'onSubmit' | 'onTest', values: ModelFormValues): Promise<void> => {
  await act(async () => {
    form()[action](values);
  });
};

const savedModel = (createMutation: jest.Mock, call = 0): string =>
  createMutation.mock.calls[call][0].variables.request.connection.model;

beforeEach(() => jest.clearAllMocks());

describe('CreateModelConfigController', () => {
  test('tests a model before saving it, and saves it once it answers', async () => {
    const { testMutation, createMutation, onClose } = setup({ ok: true, latencyMs: 300 });
    await press('onSubmit', azure);
    await waitFor(() => expect(createMutation).toHaveBeenCalledTimes(1));
    expect(testMutation).toHaveBeenCalledWith({
      variables: { projectID: 'project-1', request: expect.objectContaining({ model: 'gpt4o', apiKey: 'az-key' }) }
    });
    expect(testMutation.mock.invocationCallOrder[0]).toBeLessThan(createMutation.mock.invocationCallOrder[0]);
    expect(savedModel(createMutation)).toBe('gpt4o');
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });

  test('does not save a model whose test fails, and shows why', async () => {
    const { createMutation } = setup({ ok: false, error: 'The API deployment for this resource does not exist' });
    await press('onSubmit', { ...azure, model: 'admin' });
    await waitFor(() => expect(form().lastTest?.result.error).toMatch(/deployment .* does not exist/));
    expect(createMutation).not.toHaveBeenCalled();
  });

  test('Save anyway saves the values whose test failed, without testing them again', async () => {
    const { testMutation, createMutation } = setup({ ok: false, error: 'RateLimitError' });
    await press('onSubmit', azure);
    await waitFor(() => expect(form().lastTest?.result.ok).toBe(false));
    await press('onSubmit', azure);
    await waitFor(() => expect(createMutation).toHaveBeenCalledTimes(1));
    expect(testMutation).toHaveBeenCalledTimes(1);
  });

  test('tests again once the values change', async () => {
    const { testMutation, createMutation } = setup({ ok: false, error: 'DeploymentNotFound' }, { ok: true });
    await press('onSubmit', { ...azure, model: 'admin' });
    await waitFor(() => expect(form().lastTest?.result.ok).toBe(false));
    await press('onSubmit', azure);
    await waitFor(() => expect(createMutation).toHaveBeenCalledTimes(1));
    expect(testMutation).toHaveBeenCalledTimes(2);
    expect(savedModel(createMutation)).toBe('gpt4o');
  });

  test('a test the server could not run counts as failed', async () => {
    const { createMutation } = setup(new Error('LiteLLM is unreachable'));
    await press('onSubmit', azure);
    await waitFor(() => expect(form().lastTest?.result).toEqual({ ok: false, error: 'LiteLLM is unreachable' }));
    expect(createMutation).not.toHaveBeenCalled();
  });

  test('a model that passed Test is saved without a second test', async () => {
    const { testMutation, createMutation } = setup({ ok: true });
    await press('onTest', azure);
    await waitFor(() => expect(form().lastTest?.result.ok).toBe(true));
    await press('onSubmit', azure);
    await waitFor(() => expect(createMutation).toHaveBeenCalledTimes(1));
    expect(testMutation).toHaveBeenCalledTimes(1);
  });
});
