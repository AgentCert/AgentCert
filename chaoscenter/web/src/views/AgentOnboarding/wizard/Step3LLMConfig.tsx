import React from 'react';
import { Layout, Text, Container, Switch } from '@harnessio/uicore';
import { Color, FontVariation } from '@harnessio/design-system';
import type { WizardState } from './types';

interface Props {
  state: WizardState;
  onChange: (updates: Partial<WizardState>) => void;
  projectID: string;
}

const PROVIDER_OPTIONS = ['openai', 'anthropic', 'google', 'azure', 'ollama', 'custom'];

export const Step3LLMConfig: React.FC<Props> = ({ state, onChange }) => {
  return (
    <Layout.Vertical spacing="medium">
      <Text font={{ variation: FontVariation.H5 }}>LLM Configuration</Text>
      <Layout.Horizontal spacing="medium" style={{ alignItems: 'center' }}>
        <Switch
          checked={state.llmDependent}
          onChange={(e: React.FormEvent<HTMLInputElement>) => onChange({ llmDependent: (e.currentTarget as HTMLInputElement).checked })}
          label="This agent requires an LLM"
        />
      </Layout.Horizontal>
      {state.llmDependent && (
        <Layout.Vertical spacing="medium">
          <Container>
            <Text font={{ variation: FontVariation.SMALL_BOLD }} margin={{ bottom: 'xsmall' }}>Provider</Text>
            <select
              className="bp3-select bp3-fill"
              value={state.llmProvider}
              onChange={e => onChange({ llmProvider: e.target.value })}
            >
              {PROVIDER_OPTIONS.map(p => <option key={p} value={p}>{p}</option>)}
            </select>
          </Container>
          <Container>
            <Text font={{ variation: FontVariation.SMALL_BOLD }} margin={{ bottom: 'xsmall' }}>Model</Text>
            <input
              className="bp3-input bp3-fill"
              placeholder="e.g. gpt-4o"
              value={state.llmModel}
              onChange={e => onChange({ llmModel: e.target.value })}
            />
          </Container>
          <Container>
            <Text font={{ variation: FontVariation.SMALL_BOLD }} margin={{ bottom: 'xsmall' }}>API Key</Text>
            <input
              className="bp3-input bp3-fill"
              type="password"
              placeholder="sk-..."
              value={state.llmApiKey}
              onChange={e => onChange({ llmApiKey: e.target.value })}
            />
          </Container>
          {['azure', 'ollama', 'custom'].includes(state.llmProvider) && (
            <Container>
              <Text font={{ variation: FontVariation.SMALL_BOLD }} margin={{ bottom: 'xsmall' }}>Base URL</Text>
              <input
                className="bp3-input bp3-fill"
                placeholder="https://..."
                value={state.llmBaseURL}
                onChange={e => onChange({ llmBaseURL: e.target.value })}
              />
            </Container>
          )}
          <Container style={{ background: '#E8F4FD', padding: '8px 12px', borderRadius: 4 }}>
            <Text color={Color.BLUE_600} font={{ variation: FontVariation.SMALL }}>
              LLM credentials will be stored as a K8s Secret. Configure the Model Library in Settings to save reusable configs.
            </Text>
          </Container>
        </Layout.Vertical>
      )}
    </Layout.Vertical>
  );
};
