import '@testing-library/jest-dom/extend-expect';
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { ConfigField, ConfigFieldType, ConfigValueKind } from '@api/entities';
import ExperimentCreationSelectInstallStepView, { InstallStepEntry } from '../ExperimentCreationSelectInstallStep';

// The strings module does not type-check under the TypeScript this package
// resolves to; the drawer only needs getString, which echoes its key here.
jest.mock('@strings', () => ({
  useStrings: () => ({
    getString: (key: string, vars?: Record<string, unknown>) => (vars ? `${key} ${JSON.stringify(vars)}` : key)
  })
}));

const field = (overrides: Partial<ConfigField> & Pick<ConfigField, 'key' | 'label' | 'type'>): ConfigField => ({
  valueKind: ConfigValueKind.STRING,
  defaultValue: '',
  required: false,
  advanced: false,
  ...overrides
});

const flashAgent: InstallStepEntry = {
  folder: 'flash-agent',
  displayName: 'Flash Agent',
  description: 'Log analysis agent',
  configurations: [
    field({
      key: 'agent.config.SCAN_INTERVAL',
      label: 'Scan interval',
      type: ConfigFieldType.INTEGER,
      defaultValue: '60',
      min: 1,
      group: 'Behaviour'
    }),
    field({
      key: 'agent.config.LOG_LEVEL',
      label: 'Log level',
      type: ConfigFieldType.SELECT,
      defaultValue: 'INFO',
      options: ['DEBUG', 'INFO'],
      advanced: true
    })
  ]
};
const crewAgent: InstallStepEntry = {
  folder: 'sre-agent-crewai',
  displayName: 'SRE Agent (CrewAI)',
  namespace: 'sock-shop',
  configurations: []
};

function renderDrawer(
  props: Partial<React.ComponentProps<typeof ExperimentCreationSelectInstallStepView>> = {}
): jest.Mock {
  const onSelect = jest.fn();
  render(
    <ExperimentCreationSelectInstallStepView
      isOpen
      kind="agent"
      loading={false}
      entries={[flashAgent, crewAgent]}
      modelOptions={[]}
      onSelect={onSelect}
      onClose={jest.fn()}
      {...props}
    />
  );
  return onSelect;
}

describe('install step drawer', () => {
  test('editing an installed step opens on its settings and applies the changes', async () => {
    const onSelect = renderDrawer({
      initialSelection: {
        folder: 'flash-agent',
        namespace: 'sock-shop',
        source: { args: ['-values-json={"agent":{"config":{"SCAN_INTERVAL":"90"}}}'], parameters: [] }
      }
    });

    // Only the installed agent is shown until the user asks to change it.
    expect(screen.queryByText('SRE Agent (CrewAI)')).not.toBeInTheDocument();
    const interval = screen.getByDisplayValue('90');
    fireEvent.change(interval, { target: { value: '120' } });
    fireEvent.click(screen.getByText('apply'));

    await waitFor(() => expect(onSelect).toHaveBeenCalledTimes(1));
    expect(onSelect).toHaveBeenCalledWith({
      folder: 'flash-agent',
      namespace: 'sock-shop',
      settings: {
        values: { 'agent.config.SCAN_INTERVAL': '120', 'agent.config.LOG_LEVEL': 'INFO' },
        declaredKeys: ['agent.config.SCAN_INTERVAL', 'agent.config.LOG_LEVEL']
      }
    });
  });

  test('a value the chart does not allow is reported and nothing is applied', async () => {
    const onSelect = renderDrawer({
      initialSelection: { folder: 'flash-agent', namespace: 'sock-shop', source: { args: [], parameters: [] } }
    });

    fireEvent.change(screen.getByDisplayValue('60'), { target: { value: '0' } });
    fireEvent.click(screen.getByText('apply'));

    expect(await screen.findByText('settingMustBeAtLeast {"value":1}')).toBeInTheDocument();
    expect(onSelect).not.toHaveBeenCalled();
  });

  test('adding a step starts from the catalogue and shows the chosen chart', async () => {
    const onSelect = renderDrawer();

    expect(screen.getByText('add').closest('button')).toBeDisabled();
    fireEvent.click(screen.getByText('SRE Agent (CrewAI)'));

    expect(screen.queryByText('Flash Agent')).not.toBeInTheDocument();
    expect(screen.getByDisplayValue('sock-shop')).toBeInTheDocument();
    expect(screen.getByText('noChartSettings')).toBeInTheDocument();
    fireEvent.click(screen.getByText('add'));

    await waitFor(() =>
      expect(onSelect).toHaveBeenCalledWith({
        folder: 'sre-agent-crewai',
        namespace: 'sock-shop',
        settings: { values: {}, declaredKeys: [] }
      })
    );
  });

  test('a changed setting can be reset to the chart default', async () => {
    renderDrawer({
      initialSelection: {
        folder: 'flash-agent',
        namespace: 'sock-shop',
        source: { args: ['-values-json={"agent":{"config":{"SCAN_INTERVAL":"90"}}}'], parameters: [] }
      }
    });

    expect(screen.getByText('settingsChangedFromDefaults {"count":1,"total":2}')).toBeInTheDocument();
    fireEvent.click(screen.getByText('resetSetting'));
    expect(await screen.findByDisplayValue('60')).toBeInTheDocument();
    expect(screen.getByText('settingsAllDefaults {"total":2}')).toBeInTheDocument();
  });
});
