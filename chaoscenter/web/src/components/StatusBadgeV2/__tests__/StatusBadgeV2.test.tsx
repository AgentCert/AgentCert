import '@testing-library/jest-dom/extend-expect';
import React from 'react';
import { render, screen } from '@testing-library/react';
import { ExperimentRunStatus, ExperimentRunFaultStatus, FaultProbeStatus } from '@api/entities';
import { ChaosInfrastructureStatus, PermissionGroup } from '@models';
import { TestWrapper } from 'utils/testUtils';
import { StatusBadgeV2, StatusBadgeEntity } from '../StatusBadgeV2';

const renderBadge = (props: { status: any; entity: StatusBadgeEntity; tooltip?: string }) =>
  render(
    <TestWrapper>
      <StatusBadgeV2 {...props} />
    </TestWrapper>
  );

describe('StatusBadgeV2', () => {
  test('ExperimentRun status renders the UI phase text', () => {
    renderBadge({ status: ExperimentRunStatus.COMPLETED, entity: StatusBadgeEntity.ExperimentRun });
    expect(screen.getByTestId('status-badge-v2')).toBeInTheDocument();
    expect(screen.getByText('COMPLETED')).toBeInTheDocument();
  });

  test('ExperimentRunFault status', () => {
    renderBadge({ status: ExperimentRunFaultStatus.ERROR, entity: StatusBadgeEntity.ExperimentRunFault });
    expect(screen.getByText('ERROR')).toBeInTheDocument();
  });

  test('Probe status', () => {
    renderBadge({ status: FaultProbeStatus.PASSED, entity: StatusBadgeEntity.Probe });
    expect(screen.getByText('PASSED')).toBeInTheDocument();
  });

  test('Infrastructure status', () => {
    renderBadge({ status: ChaosInfrastructureStatus.ACTIVE, entity: StatusBadgeEntity.Infrastructure });
    // ChaosInfrastructureStatus.ACTIVE = 'CONNECTED' -> phaseToUI uppercases
    expect(screen.getByText('CONNECTED')).toBeInTheDocument();
  });

  test('PermissionGroup status', () => {
    renderBadge({ status: PermissionGroup.OWNER, entity: StatusBadgeEntity.PermissionGroup });
    expect(screen.getByText('OWNER')).toBeInTheDocument();
  });

  test('renders NA phase as N/A', () => {
    renderBadge({ status: ExperimentRunStatus.NA, entity: StatusBadgeEntity.ExperimentRun });
    expect(screen.getByText('N/A')).toBeInTheDocument();
  });

  test('passes through tooltip without crashing', () => {
    renderBadge({
      status: ExperimentRunStatus.RUNNING,
      entity: StatusBadgeEntity.ExperimentRun,
      tooltip: 'hovering'
    });
    expect(screen.getByText('RUNNING')).toBeInTheDocument();
  });
});
