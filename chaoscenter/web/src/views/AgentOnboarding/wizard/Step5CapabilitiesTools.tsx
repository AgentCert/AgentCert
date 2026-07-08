import React from 'react';
import { Layout, Text, Button, ButtonVariation, Container, Checkbox } from '@harnessio/uicore';
import { FontVariation } from '@harnessio/design-system';
import type { WizardState, RequiredToolInput } from './types';
import { EVAL_METRICS } from './types';

const CAPABILITIES = [
  { key: 'prometheus-query', domain: 'cloud-native', category: 'observe', label: 'Prometheus Query' },
  { key: 'kubernetes-get-pods', domain: 'cloud-native', category: 'observe', label: 'K8s Get Pods' },
  { key: 'kubernetes-restart', domain: 'cloud-native', category: 'act', label: 'K8s Restart' },
  { key: 'kubernetes-scale', domain: 'cloud-native', category: 'act', label: 'K8s Scale' },
  { key: 'log-query', domain: 'cloud-native', category: 'observe', label: 'Log Query' },
  { key: 'http-probe', domain: 'common', category: 'observe', label: 'HTTP Probe' },
  { key: 'webhook-notify', domain: 'common', category: 'act', label: 'Webhook Notify' },
  { key: 'ticket-create', domain: 'itops', category: 'act', label: 'Ticket Create' },
  { key: 'monitoring-alert-query', domain: 'itops', category: 'observe', label: 'Alert Query' },
];

interface Props {
  state: WizardState;
  onChange: (updates: Partial<WizardState>) => void;
}

export const Step5CapabilitiesTools: React.FC<Props> = ({ state, onChange }) => {
  const toggleCap = (key: string) => {
    const caps = state.capabilities.includes(key)
      ? state.capabilities.filter(c => c !== key)
      : [...state.capabilities, key];
    onChange({ capabilities: caps });
  };

  const toggleMetric = (key: string) => {
    const metrics = state.evaluationMetrics.includes(key)
      ? state.evaluationMetrics.filter(m => m !== key)
      : [...state.evaluationMetrics, key];
    onChange({ evaluationMetrics: metrics });
  };

  const addTool = () => {
    onChange({
      requiredTools: [
        ...state.requiredTools,
        { name: '', critical: false, minCallCount: 1 },
      ],
    });
  };

  const updateTool = (idx: number, updates: Partial<RequiredToolInput>) => {
    const tools = [...state.requiredTools];
    tools[idx] = { ...tools[idx], ...updates };
    onChange({ requiredTools: tools });
  };

  const removeTool = (idx: number) => {
    onChange({ requiredTools: state.requiredTools.filter((_, i) => i !== idx) });
  };

  return (
    <Layout.Vertical spacing="large">
      <Text font={{ variation: FontVariation.H5 }}>Capabilities &amp; Tools</Text>

      <Container>
        <Text font={{ variation: FontVariation.H6 }} margin={{ bottom: 'small' }}>Capabilities</Text>
        <Layout.Vertical spacing="xsmall">
          {CAPABILITIES.map(cap => (
            <Layout.Horizontal key={cap.key} spacing="small" style={{ alignItems: 'center' }}>
              <Checkbox
                checked={state.capabilities.includes(cap.key)}
                onChange={() => toggleCap(cap.key)}
                label=""
              />
              <Text font={{ variation: FontVariation.SMALL_BOLD }}>{cap.label}</Text>
              <Text font={{ variation: FontVariation.SMALL }} style={{ color: '#999' }}>
                [{cap.domain} / {cap.category}]
              </Text>
            </Layout.Horizontal>
          ))}
        </Layout.Vertical>
      </Container>

      <Container>
        <Text font={{ variation: FontVariation.H6 }} margin={{ bottom: 'small' }}>Required MCP Tools</Text>
        {state.requiredTools.map((tool, idx) => (
          <Layout.Horizontal key={idx} spacing="small" margin={{ bottom: 'xsmall' }} style={{ alignItems: 'center' }}>
            <input
              className="bp3-input"
              style={{ flex: 2 }}
              placeholder="Tool name"
              value={tool.name}
              onChange={e => updateTool(idx, { name: e.target.value })}
            />
            <Checkbox
              checked={tool.critical}
              onChange={(e: React.FormEvent<HTMLInputElement>) => updateTool(idx, { critical: (e.currentTarget as HTMLInputElement).checked })}
              label="Critical"
            />
            <Button icon="trash" variation={ButtonVariation.ICON} onClick={() => removeTool(idx)} />
          </Layout.Horizontal>
        ))}
        <Button text="+ Add Tool" variation={ButtonVariation.SECONDARY} onClick={addTool} />
      </Container>

      <Container>
        <Text font={{ variation: FontVariation.H6 }} margin={{ bottom: 'small' }}>Evaluation Metrics</Text>
        <Layout.Vertical spacing="xsmall">
          {EVAL_METRICS.map(m => (
            <Layout.Horizontal key={m.key} spacing="small" style={{ alignItems: 'center' }}>
              <Checkbox
                checked={state.evaluationMetrics.includes(m.key)}
                onChange={() => toggleMetric(m.key)}
                label=""
              />
              <Text font={{ variation: FontVariation.SMALL_BOLD }}>{m.label}</Text>
              <Text font={{ variation: FontVariation.SMALL }} style={{ color: '#999' }}>{m.description}</Text>
            </Layout.Horizontal>
          ))}
        </Layout.Vertical>
      </Container>
    </Layout.Vertical>
  );
};
