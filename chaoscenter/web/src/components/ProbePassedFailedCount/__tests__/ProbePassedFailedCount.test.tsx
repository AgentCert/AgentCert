import '@testing-library/jest-dom/extend-expect';
import React from 'react';
import { render, screen } from '@testing-library/react';
import { ExperimentRunFaultStatus } from '@api/entities';
import { TestWrapper } from 'utils/testUtils';
import ProbePassedFailedCount from '../ProbePassedFailedCount';

describe('ProbePassedFailedCount', () => {
  test('shows numeric counts for a terminal phase', () => {
    render(
      <TestWrapper>
        <ProbePassedFailedCount passedCount={3} failedCount={1} phase={ExperimentRunFaultStatus.COMPLETED} />
      </TestWrapper>
    );
    expect(screen.getByText('3')).toBeInTheDocument();
    expect(screen.getByText('1')).toBeInTheDocument();
    // getString echoes the key
    expect(screen.getByText('passed')).toBeInTheDocument();
    expect(screen.getByText('failed')).toBeInTheDocument();
  });

  test('shows -- for both counts when RUNNING', () => {
    render(
      <TestWrapper>
        <ProbePassedFailedCount passedCount={5} failedCount={2} phase={ExperimentRunFaultStatus.RUNNING} />
      </TestWrapper>
    );
    const dashes = screen.getAllByText('--');
    expect(dashes).toHaveLength(2);
  });

  test('renders with flexStart layout option', () => {
    const { container } = render(
      <TestWrapper>
        <ProbePassedFailedCount
          passedCount={0}
          failedCount={0}
          phase={ExperimentRunFaultStatus.COMPLETED}
          flexStart
          textColorInverse
        />
      </TestWrapper>
    );
    expect(container).toBeTruthy();
    expect(screen.getAllByText('0')).toHaveLength(2);
  });
});
