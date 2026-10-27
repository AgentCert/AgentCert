import '@testing-library/jest-dom/extend-expect';
import React from 'react';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import type { FormikProps } from 'formik';
import { ConfigFieldType, ConfigValueKind } from '@api/entities';
import type { TuneExperimentForm } from '../types';
import FaultTunablesTab from '../FaultTunablesTab';

jest.mock('@strings', () => ({
  useStrings: () => ({
    getString: (key: string, vars?: Record<string, unknown>) => (vars ? `${key} ${JSON.stringify(vars)}` : key)
  })
}));
jest.mock('@hooks', () => ({
  useSearchParams: () => new URLSearchParams(),
  useUpdateSearchParams: () => jest.fn()
}));
jest.mock('@utils', () => ({}));
jest.mock('@components/InputSlider', () => () => null);
jest.mock('@services/experiment', () => ({ __esModule: true, default: { getInfrastructureTypeHandler: () => ({}) } }));

test('renders typed fault fields, validates bounds, and resets to the fault default', async () => {
  const formRef = { current: undefined } as React.MutableRefObject<FormikProps<TuneExperimentForm> | undefined>;
  render(
    <FaultTunablesTab
      formRef={formRef}
      faultData={undefined}
      setFaultData={jest.fn()}
      initialFaultTunables={undefined}
      setFaultWeight={jest.fn()}
      configurations={[
        {
          key: 'BROKEN_TARGET_PORT',
          label: 'Broken target port',
          type: ConfigFieldType.INTEGER,
          valueKind: ConfigValueKind.STRING,
          defaultValue: '9999',
          required: true,
          min: 1,
          max: 65535,
          advanced: false
        }
      ]}
    />
  );
  expect(screen.getByText('Broken target port')).toBeInTheDocument();
  expect(screen.getByDisplayValue('9999')).toHaveAttribute('type', 'number');
  fireEvent.change(screen.getByDisplayValue('9999'), { target: { value: '70000' } });
  await act(async () => {
    await formRef.current?.validateForm();
  });
  await waitFor(() => expect(formRef.current?.errors.setting0).toContain('settingMustBeAtMost'));
  fireEvent.click(screen.getByText('resetSetting'));
  expect(await screen.findByDisplayValue('9999')).toBeInTheDocument();
  fireEvent.change(screen.getByDisplayValue('9999'), { target: { value: '4321' } });
  await act(async () => {
    await formRef.current?.validateForm();
  });
  expect(formRef.current?.errors).toEqual({});
  expect(formRef.current?.values.setting0).toBe(4321);
});
