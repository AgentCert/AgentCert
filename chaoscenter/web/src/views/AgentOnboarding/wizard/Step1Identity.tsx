import React from 'react';
import { Layout, Text, Container } from '@harnessio/uicore';
import { FontVariation } from '@harnessio/design-system';
import type { WizardState } from './types';
import { APPROACH_OPTIONS } from './types';

interface Props {
  state: WizardState;
  onChange: (updates: Partial<WizardState>) => void;
}

export const Step1Identity: React.FC<Props> = ({ state, onChange }) => {
  return (
    <Layout.Vertical spacing="medium">
      <Text font={{ variation: FontVariation.H5 }}>Agent Identity</Text>
      <Layout.Horizontal spacing="medium">
        <Container style={{ flex: 1 }}>
          <Text font={{ variation: FontVariation.SMALL_BOLD }} margin={{ bottom: 'xsmall' }}>Agent Name *</Text>
          <input
            className="bp3-input bp3-fill"
            placeholder="e.g. my-k8s-agent"
            value={state.agentName}
            onChange={e => onChange({ agentName: e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, '') })}
          />
          <Text font={{ variation: FontVariation.TINY }} style={{ marginTop: 4 }}>
            Lowercase alphanumeric and hyphens only. Max 63 chars.
          </Text>
        </Container>
        <Container style={{ flex: 1 }}>
          <Text font={{ variation: FontVariation.SMALL_BOLD }} margin={{ bottom: 'xsmall' }}>Display Name *</Text>
          <input
            className="bp3-input bp3-fill"
            placeholder="e.g. My K8s Remediation Agent"
            value={state.displayName}
            onChange={e => onChange({ displayName: e.target.value })}
          />
        </Container>
      </Layout.Horizontal>
      <Container>
        <Text font={{ variation: FontVariation.SMALL_BOLD }} margin={{ bottom: 'xsmall' }}>Short Description * (10–120 chars)</Text>
        <input
          className="bp3-input bp3-fill"
          placeholder="One-line summary of what this agent does"
          value={state.shortDescription}
          onChange={e => onChange({ shortDescription: e.target.value })}
        />
      </Container>
      <Container>
        <Text font={{ variation: FontVariation.SMALL_BOLD }} margin={{ bottom: 'xsmall' }}>Full Description *</Text>
        <textarea
          className="bp3-input bp3-fill"
          rows={4}
          placeholder="Detailed description of agent behavior, prerequisites, and use cases"
          value={state.fullDescription}
          onChange={e => onChange({ fullDescription: e.target.value })}
          style={{ resize: 'vertical' }}
        />
      </Container>
      <Layout.Horizontal spacing="medium">
        <Container style={{ flex: 1 }}>
          <Text font={{ variation: FontVariation.SMALL_BOLD }} margin={{ bottom: 'xsmall' }}>Reasoning Approach</Text>
          <select
            className="bp3-select bp3-fill"
            value={state.approach}
            onChange={e => onChange({ approach: e.target.value })}
          >
            {APPROACH_OPTIONS.map(opt => <option key={opt.value} value={opt.value}>{opt.label}</option>)}
          </select>
        </Container>
        <Container style={{ flex: 1 }}>
          <Text font={{ variation: FontVariation.SMALL_BOLD }} margin={{ bottom: 'xsmall' }}>Repository URL</Text>
          <input
            className="bp3-input bp3-fill"
            placeholder="https://github.com/..."
            value={state.repositoryURL}
            onChange={e => onChange({ repositoryURL: e.target.value })}
          />
        </Container>
      </Layout.Horizontal>
      <Text font={{ variation: FontVariation.H6 }} margin={{ top: 'medium' }}>Owner</Text>
      <Layout.Horizontal spacing="medium">
        <Container style={{ flex: 1 }}>
          <Text font={{ variation: FontVariation.SMALL_BOLD }} margin={{ bottom: 'xsmall' }}>Name *</Text>
          <input className="bp3-input bp3-fill" value={state.ownerName} onChange={e => onChange({ ownerName: e.target.value })} />
        </Container>
        <Container style={{ flex: 1 }}>
          <Text font={{ variation: FontVariation.SMALL_BOLD }} margin={{ bottom: 'xsmall' }}>Email *</Text>
          <input className="bp3-input bp3-fill" type="email" value={state.ownerEmail} onChange={e => onChange({ ownerEmail: e.target.value })} />
        </Container>
        <Container style={{ flex: 1 }}>
          <Text font={{ variation: FontVariation.SMALL_BOLD }} margin={{ bottom: 'xsmall' }}>Organization</Text>
          <input className="bp3-input bp3-fill" value={state.ownerOrg} onChange={e => onChange({ ownerOrg: e.target.value })} />
        </Container>
      </Layout.Horizontal>
    </Layout.Vertical>
  );
};
