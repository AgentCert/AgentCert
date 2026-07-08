import React from 'react';
import { Layout, Text, Container } from '@harnessio/uicore';
import { FontVariation } from '@harnessio/design-system';
import type { WizardState } from './types';

interface Props {
  state: WizardState;
  onChange: (updates: Partial<WizardState>) => void;
}

export const Step6AppCompatibility: React.FC<Props> = ({ state, onChange }) => (
  <Layout.Vertical spacing="medium">
    <Text font={{ variation: FontVariation.H5 }}>App Compatibility</Text>
    <Layout.Vertical spacing="small">
      <label style={{ display: 'flex', alignItems: 'center', gap: 8, cursor: 'pointer' }}>
        <input
          type="radio"
          value="all"
          checked={state.compatibilityMode === 'all'}
          onChange={() => onChange({ compatibilityMode: 'all' })}
        />
        <Text font={{ variation: FontVariation.SMALL }}>Compatible with all catalog apps (default)</Text>
      </label>
      <label style={{ display: 'flex', alignItems: 'center', gap: 8, cursor: 'pointer' }}>
        <input
          type="radio"
          value="specify"
          checked={state.compatibilityMode === 'specify'}
          onChange={() => onChange({ compatibilityMode: 'specify' })}
        />
        <Text font={{ variation: FontVariation.SMALL }}>Specify compatibility</Text>
      </label>
    </Layout.Vertical>
    {state.compatibilityMode === 'specify' && (
      <Container>
        <Text font={{ variation: FontVariation.SMALL_BOLD }} margin={{ bottom: 'xsmall' }}>
          Note: Specify app IDs from the catalog. Leave blank to allow all.
        </Text>
        <Text font={{ variation: FontVariation.SMALL_BOLD }} margin={{ bottom: 'xsmall' }}>Supported Apps (comma-separated)</Text>
        <input
          className="bp3-input bp3-fill"
          placeholder="sock-shop, otel-demo"
          value={state.supportedApps.join(', ')}
          onChange={e => onChange({ supportedApps: e.target.value.split(',').map(s => s.trim()).filter(Boolean) })}
        />
        <Text font={{ variation: FontVariation.SMALL_BOLD }} margin={{ top: 'small', bottom: 'xsmall' }}>Incompatible Apps (comma-separated)</Text>
        <input
          className="bp3-input bp3-fill"
          placeholder="bookinfo"
          value={state.unsupportedApps.join(', ')}
          onChange={e => onChange({ unsupportedApps: e.target.value.split(',').map(s => s.trim()).filter(Boolean) })}
        />
      </Container>
    )}
  </Layout.Vertical>
);
