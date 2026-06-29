import '@testing-library/jest-dom/extend-expect';
import React from 'react';
import { render, screen } from '@testing-library/react';
import type { ProbeAttributes } from '@models';
import { TestWrapper } from 'utils/testUtils';
import ProbeInformationCard, { ProbeInformationType } from '../ProbeInformationCard';

const probe = {
  name: 'http-check',
  type: 'httpProbe',
  'httpProbe/inputs': {
    url: 'http://service:8080',
    insecureSkipVerify: false
  },
  runProperties: {
    probeTimeout: '10s',
    interval: '2s'
  }
} as unknown as ProbeAttributes;

describe('ProbeInformationCard', () => {
  test('renders DETAILS view with probe inputs', () => {
    render(
      <TestWrapper>
        <ProbeInformationCard probe={probe} display={ProbeInformationType.DETAILS} />
      </TestWrapper>
    );
    // header uses getString('probeDetails') which echoes the key
    expect(screen.getByText('probeDetails')).toBeInTheDocument();
    expect(screen.getByText('url:')).toBeInTheDocument();
    expect(screen.getByText('http://service:8080')).toBeInTheDocument();
  });

  test('renders PROPERTIES view with run properties', () => {
    render(
      <TestWrapper>
        <ProbeInformationCard probe={probe} display={ProbeInformationType.PROPERTIES} />
      </TestWrapper>
    );
    expect(screen.getByText('probeProperties')).toBeInTheDocument();
    expect(screen.getByText('probeTimeout:')).toBeInTheDocument();
    expect(screen.getByText('10s')).toBeInTheDocument();
    expect(screen.getByText('interval:')).toBeInTheDocument();
  });

  test('hides title when isVerbose is false', () => {
    render(
      <TestWrapper>
        <ProbeInformationCard probe={probe} display={ProbeInformationType.DETAILS} isVerbose={false} />
      </TestWrapper>
    );
    expect(screen.queryByText('probeDetails')).not.toBeInTheDocument();
    // data still renders
    expect(screen.getByText('url:')).toBeInTheDocument();
  });
});
