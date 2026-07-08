import React from 'react';
import { Layout, Text, Container } from '@harnessio/uicore';
import { Color, FontVariation } from '@harnessio/design-system';
import type { WizardState } from './types';

interface Props {
  state: WizardState;
  onChange: (updates: Partial<WizardState>) => void;
}

export const Step2DockerImage: React.FC<Props> = ({ state, onChange }) => {
  const isLatest = state.dockerImage.endsWith(':latest') || (!state.dockerImage.includes(':') && state.dockerImage.length > 0);

  return (
    <Layout.Vertical spacing="medium">
      <Text font={{ variation: FontVariation.H5 }}>Docker Image</Text>
      <Container>
        <Text font={{ variation: FontVariation.SMALL_BOLD }} margin={{ bottom: 'xsmall' }}>Container Image *</Text>
        <input
          className="bp3-input bp3-fill"
          placeholder="docker.io/myorg/my-agent:v1.0.0"
          value={state.dockerImage}
          onChange={e => onChange({ dockerImage: e.target.value })}
        />
        {isLatest && (
          <Container style={{ background: '#FFF3CD', padding: '8px 12px', borderRadius: 4, marginTop: 8 }}>
            <Text color={Color.YELLOW_800} font={{ variation: FontVariation.SMALL }}>
              Warning: Using :latest means different agent versions may run during certification. Pin to a specific version for reproducible results.
            </Text>
          </Container>
        )}
      </Container>
      <Text font={{ variation: FontVariation.H6 }} margin={{ top: 'medium' }}>Resource Requirements</Text>
      <Layout.Horizontal spacing="medium">
        <Container style={{ flex: 1 }}>
          <Text font={{ variation: FontVariation.SMALL_BOLD }} margin={{ bottom: 'xsmall' }}>CPU Request</Text>
          <input className="bp3-input bp3-fill" value={state.cpuRequest} onChange={e => onChange({ cpuRequest: e.target.value })} placeholder="100m" />
        </Container>
        <Container style={{ flex: 1 }}>
          <Text font={{ variation: FontVariation.SMALL_BOLD }} margin={{ bottom: 'xsmall' }}>Memory Request</Text>
          <input className="bp3-input bp3-fill" value={state.memoryRequest} onChange={e => onChange({ memoryRequest: e.target.value })} placeholder="128Mi" />
        </Container>
        <Container style={{ flex: 1 }}>
          <Text font={{ variation: FontVariation.SMALL_BOLD }} margin={{ bottom: 'xsmall' }}>CPU Limit</Text>
          <input className="bp3-input bp3-fill" value={state.cpuLimit} onChange={e => onChange({ cpuLimit: e.target.value })} placeholder="500m" />
        </Container>
        <Container style={{ flex: 1 }}>
          <Text font={{ variation: FontVariation.SMALL_BOLD }} margin={{ bottom: 'xsmall' }}>Memory Limit</Text>
          <input className="bp3-input bp3-fill" value={state.memoryLimit} onChange={e => onChange({ memoryLimit: e.target.value })} placeholder="512Mi" />
        </Container>
      </Layout.Horizontal>
    </Layout.Vertical>
  );
};
