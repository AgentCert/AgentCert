import React from 'react';
import { Layout, Text, Container } from '@harnessio/uicore';
import { Color, FontVariation } from '@harnessio/design-system';
import type { WizardState } from './types';

interface Props {
  state: WizardState;
  onChange: (updates: Partial<WizardState>) => void;
}

const Row: React.FC<{ label: string; value: React.ReactNode }> = ({ label, value }) => (
  <Layout.Horizontal spacing="medium" padding={{ bottom: 'small' }} style={{ borderBottom: '1px solid #f0f0f0' }}>
    <Text font={{ variation: FontVariation.SMALL_BOLD }} style={{ minWidth: 160 }} color={Color.GREY_600}>{label}</Text>
    <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_900}>{value}</Text>
  </Layout.Horizontal>
);

export const Step7Review: React.FC<Props> = ({ state, onChange }) => (
  <Layout.Vertical spacing="medium">
    <Text font={{ variation: FontVariation.H5 }}>Review &amp; Register</Text>
    <Container style={{ border: '1px solid #E0E0E0', borderRadius: 8, padding: 16 }}>
      <Layout.Vertical spacing="small">
        <Row label="Agent Name" value={state.agentName} />
        <Row label="Display Name" value={state.displayName} />
        <Row label="Image" value={state.dockerImage} />
        <Row label="LLM Required" value={state.llmDependent ? 'Yes' : 'No'} />
        <Row label="Capabilities" value={state.capabilities.length + ' selected'} />
        <Row label="MCP Tools" value={state.requiredTools.length + ' tools'} />
        <Row label="Eval Metrics" value={state.evaluationMetrics.length + ' metrics'} />
        <Row label="Config Inputs" value={state.inputs.length + ' parameters'} />
        <Row label="Compatibility" value={state.compatibilityMode === 'all' ? 'All apps' : 'Specified apps'} />
        <Row label="Owner" value={state.ownerName + ' <' + state.ownerEmail + '>'} />
      </Layout.Vertical>
    </Container>
    <Text font={{ variation: FontVariation.H6 }} margin={{ top: 'medium' }}>Visibility</Text>
    <Layout.Vertical spacing="small">
      <label style={{ display: 'flex', alignItems: 'center', gap: 8, cursor: 'pointer' }}>
        <input
          type="radio"
          value="private"
          checked={state.tier === 'private'}
          onChange={() => onChange({ tier: 'private' })}
        />
        <Text font={{ variation: FontVariation.SMALL }}>Private — visible only to your project</Text>
      </label>
      <label style={{ display: 'flex', alignItems: 'center', gap: 8, cursor: 'pointer' }}>
        <input
          type="radio"
          value="community"
          checked={state.tier === 'community'}
          onChange={() => onChange({ tier: 'community' })}
        />
        <Text font={{ variation: FontVariation.SMALL }}>Contribute to community catalog (requires review)</Text>
      </label>
    </Layout.Vertical>
  </Layout.Vertical>
);
