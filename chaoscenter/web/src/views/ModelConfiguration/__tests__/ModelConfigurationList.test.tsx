import React from 'react';
import '@testing-library/jest-dom';
import { render, screen } from '@testing-library/react';
import { TestWrapper } from 'utils/testUtils';
import { ModelProvider, ModelSyncStatus } from '@api/entities';
import ModelConfigurationListView from '../ModelConfigurationList';
import { PLATFORM_STATUS, ModelRow } from '../modelConfigUtils';

const rows: ModelRow[] = [
  {
    alias: 'gpt-4o-bench',
    label: 'GPT-4o bench',
    provider: 'Azure OpenAI',
    model: 'azure/gpt-4o',
    status: ModelSyncStatus.NEEDS_KEY,
    keyHint: '****1234',
    lastError: 'LiteLLM no longer has this model',
    isDefault: false,
    config: {
      projectID: 'p1',
      modelID: 'm1',
      alias: 'gpt-4o-bench',
      label: 'GPT-4o bench',
      provider: ModelProvider.AZURE_OPENAI,
      model: 'azure/gpt-4o',
      syncStatus: ModelSyncStatus.NEEDS_KEY
    }
  },
  { alias: 'gemini-2.5-flash', provider: 'Gemini', status: PLATFORM_STATUS, isDefault: true }
];

describe('ModelConfigurationListView', () => {
  test('lists project and platform models with their status', () => {
    render(
      <TestWrapper>
        <ModelConfigurationListView rows={rows} loading={false} refetchModels={jest.fn()} />
      </TestWrapper>
    );
    expect(screen.getByText('gpt-4o-bench')).toBeInTheDocument();
    expect(screen.getByText('GPT-4o bench')).toBeInTheDocument();
    expect(screen.getByText('****1234')).toBeInTheDocument();
    expect(screen.getByText('modelStatusNeedsKey')).toBeInTheDocument();
    expect(screen.getByText('platform')).toBeInTheDocument();
    expect(screen.getByText('platformDefaultModel')).toBeInTheDocument();
    // Only project models have actions.
    expect(screen.getByTestId('model-menu-gpt-4o-bench')).toBeInTheDocument();
    expect(screen.queryByTestId('model-menu-gemini-2.5-flash')).not.toBeInTheDocument();
    expect(screen.getByTestId('add-model')).not.toBeDisabled();
  });

  test('says when there are no models', () => {
    render(
      <TestWrapper>
        <ModelConfigurationListView rows={[]} loading={false} refetchModels={jest.fn()} />
      </TestWrapper>
    );
    expect(screen.getByText('noModelsTitle')).toBeInTheDocument();
  });
});
