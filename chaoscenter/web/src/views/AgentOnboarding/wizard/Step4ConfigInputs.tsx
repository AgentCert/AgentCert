import React from 'react';
import { Layout, Text, Button, ButtonVariation, Container } from '@harnessio/uicore';
import { FontVariation } from '@harnessio/design-system';
import type { WizardState, AgentInputDefinition } from './types';

interface Props {
  state: WizardState;
  onChange: (updates: Partial<WizardState>) => void;
}

const INPUT_TYPES = ['string', 'secret', 'integer', 'boolean', 'enum'];

export const Step4ConfigInputs: React.FC<Props> = ({ state, onChange }) => {
  const addInput = () => {
    onChange({
      inputs: [
        ...state.inputs,
        { key: '', displayName: '', type: 'string', required: false, helmPath: '', advanced: false },
      ],
    });
  };

  const updateInput = (idx: number, updates: Partial<AgentInputDefinition>) => {
    const inputs = [...state.inputs];
    inputs[idx] = { ...inputs[idx], ...updates };
    onChange({ inputs });
  };

  const removeInput = (idx: number) => {
    onChange({ inputs: state.inputs.filter((_, i) => i !== idx) });
  };

  return (
    <Layout.Vertical spacing="medium">
      <Text font={{ variation: FontVariation.H5 }}>Configuration Inputs</Text>
      <Container style={{ background: '#E8F4FD', padding: '8px 12px', borderRadius: 4 }}>
        <Text font={{ variation: FontVariation.SMALL }}>
          LLM API keys are configured in Step 3. This step is for other secrets your agent needs — PagerDuty tokens, JIRA API keys, monitoring credentials.
        </Text>
      </Container>
      {state.inputs.map((inp, idx) => (
        <Container key={idx} style={{ border: '1px solid #E0E0E0', borderRadius: 4, padding: 12 }}>
          <Layout.Vertical spacing="small">
            <Layout.Horizontal spacing="small">
              <Container style={{ flex: 2 }}>
                <Text font={{ variation: FontVariation.TINY_SEMI }} margin={{ bottom: 'xsmall' }}>Key</Text>
                <input
                  className="bp3-input bp3-fill"
                  placeholder="MY_SECRET_KEY"
                  value={inp.key}
                  onChange={e => updateInput(idx, { key: e.target.value.toUpperCase().replace(/[^A-Z0-9_]/g, '_') })}
                />
              </Container>
              <Container style={{ flex: 2 }}>
                <Text font={{ variation: FontVariation.TINY_SEMI }} margin={{ bottom: 'xsmall' }}>Display Name</Text>
                <input className="bp3-input bp3-fill" value={inp.displayName} onChange={e => updateInput(idx, { displayName: e.target.value })} />
              </Container>
              <Container style={{ flex: 1 }}>
                <Text font={{ variation: FontVariation.TINY_SEMI }} margin={{ bottom: 'xsmall' }}>Type</Text>
                <select
                  className="bp3-select bp3-fill"
                  value={inp.type}
                  onChange={e => updateInput(idx, { type: e.target.value as AgentInputDefinition['type'] })}
                >
                  {INPUT_TYPES.map(t => <option key={t} value={t}>{t}</option>)}
                </select>
              </Container>
              <Container style={{ flex: 2 }}>
                <Text font={{ variation: FontVariation.TINY_SEMI }} margin={{ bottom: 'xsmall' }}>Helm Path</Text>
                <input className="bp3-input bp3-fill" placeholder="agent.mySecretKey" value={inp.helmPath} onChange={e => updateInput(idx, { helmPath: e.target.value })} />
              </Container>
              <Button icon="trash" variation={ButtonVariation.ICON} onClick={() => removeInput(idx)} style={{ alignSelf: 'flex-end' }} />
            </Layout.Horizontal>
          </Layout.Vertical>
        </Container>
      ))}
      <Button text="+ Add Parameter" variation={ButtonVariation.SECONDARY} onClick={addInput} />
    </Layout.Vertical>
  );
};
