import { Color } from '@harnessio/design-system';
import { ChaosInfrastructureStatus, PermissionGroup } from '@models';
import { ExperimentRunFaultStatus, ExperimentRunStatus, FaultProbeStatus } from '@api/entities';
import {
  getPropsBasedOnChaosInfrastructureStatus,
  getPropsBasedOnExperimentRunFaultStatus,
  getPropsBasedOnExperimentRunStatus,
  getPropsBasedOnProbeStatus,
  getPropsBasedOnPermissionGroup
} from '../getPropsBasedOnStatus';

describe('getPropsBasedOnChaosInfrastructureStatus', () => {
  test('ACTIVE -> green', () => {
    expect(getPropsBasedOnChaosInfrastructureStatus(ChaosInfrastructureStatus.ACTIVE)).toEqual({
      color: Color.GREEN_800,
      bgColor: `var(--green-50)`
    });
  });

  test('PENDING -> loading icon', () => {
    const props = getPropsBasedOnChaosInfrastructureStatus(ChaosInfrastructureStatus.PENDING);
    expect(props.iconName).toBe('loading');
    expect(props.color).toBe(Color.PRIMARY_7);
  });

  test('INACTIVE -> red', () => {
    expect(getPropsBasedOnChaosInfrastructureStatus(ChaosInfrastructureStatus.INACTIVE).color).toBe(Color.RED_600);
  });

  test('UPGRADE_NEEDED -> flash icon with size', () => {
    const props = getPropsBasedOnChaosInfrastructureStatus(ChaosInfrastructureStatus.UPGRADE_NEEDED);
    expect(props.iconName).toBe('flash');
    expect(props.iconSize).toBe(15);
    expect(props.color).toBe(Color.ORANGE_700);
  });

  test('unknown -> grey default', () => {
    expect(getPropsBasedOnChaosInfrastructureStatus('SOMETHING_ELSE' as any)).toEqual({
      color: Color.GREY_700,
      bgColor: `var(--grey-200)`
    });
  });
});

describe('getPropsBasedOnExperimentRunFaultStatus', () => {
  test.each([
    ExperimentRunFaultStatus.COMPLETED,
    ExperimentRunFaultStatus.SUCCEEDED,
    ExperimentRunFaultStatus.PASSED
  ])('%s -> tick-circle/green', status => {
    const props = getPropsBasedOnExperimentRunFaultStatus(status);
    expect(props.iconName).toBe('tick-circle');
    expect(props.color).toBe(Color.GREEN_800);
  });

  test.each([
    ExperimentRunFaultStatus.COMPLETED_WITH_PROBE_FAILURE,
    ExperimentRunFaultStatus.COMPLETED_WITH_ERROR
  ])('%s -> error/orange', status => {
    const props = getPropsBasedOnExperimentRunFaultStatus(status);
    expect(props.iconName).toBe('error');
    expect(props.color).toBe(Color.ORANGE_500);
  });

  test.each([ExperimentRunFaultStatus.ERROR, ExperimentRunFaultStatus.FAILED])('%s -> circle-cross/red', status => {
    const props = getPropsBasedOnExperimentRunFaultStatus(status);
    expect(props.iconName).toBe('circle-cross');
    expect(props.color).toBe(Color.RED_600);
  });

  test('RUNNING -> running-filled', () => {
    const props = getPropsBasedOnExperimentRunFaultStatus(ExperimentRunFaultStatus.RUNNING);
    expect(props.iconName).toBe('running-filled');
    expect(props.iconSize).toBe(11);
  });

  test('STOPPED -> circle-stop', () => {
    expect(getPropsBasedOnExperimentRunFaultStatus(ExperimentRunFaultStatus.STOPPED).iconName).toBe('circle-stop');
  });

  test('SKIPPED -> conditional-filled', () => {
    expect(getPropsBasedOnExperimentRunFaultStatus(ExperimentRunFaultStatus.SKIPPED).iconName).toBe(
      'conditional-filled'
    );
  });

  test('default -> disable', () => {
    const props = getPropsBasedOnExperimentRunFaultStatus(ExperimentRunFaultStatus.NA);
    expect(props.iconName).toBe('disable');
    expect(props.iconSize).toBe(10);
  });
});

describe('getPropsBasedOnExperimentRunStatus', () => {
  test('COMPLETED', () => {
    const props = getPropsBasedOnExperimentRunStatus(ExperimentRunStatus.COMPLETED);
    expect(props.iconName).toBe('tick-circle');
    expect(props.bgColor).toBe(`var(--green-50)`);
  });

  test.each([
    ExperimentRunStatus.COMPLETED_WITH_PROBE_FAILURE,
    ExperimentRunStatus.COMPLETED_WITH_ERROR
  ])('%s -> error', status => {
    expect(getPropsBasedOnExperimentRunStatus(status).iconName).toBe('error');
  });

  test('ERROR', () => {
    expect(getPropsBasedOnExperimentRunStatus(ExperimentRunStatus.ERROR).iconName).toBe('circle-cross');
  });

  test('TIMEOUT', () => {
    expect(getPropsBasedOnExperimentRunStatus(ExperimentRunStatus.TIMEOUT).iconName).toBe('time');
  });

  test('RUNNING', () => {
    expect(getPropsBasedOnExperimentRunStatus(ExperimentRunStatus.RUNNING).iconName).toBe('loading');
  });

  test('QUEUED', () => {
    expect(getPropsBasedOnExperimentRunStatus(ExperimentRunStatus.QUEUED).iconName).toBe('pause');
  });

  test('STOPPED', () => {
    expect(getPropsBasedOnExperimentRunStatus(ExperimentRunStatus.STOPPED).iconName).toBe('circle-stop');
  });

  test('default -> disable', () => {
    expect(getPropsBasedOnExperimentRunStatus(ExperimentRunStatus.NA).iconName).toBe('disable');
  });
});

describe('getPropsBasedOnProbeStatus', () => {
  test('PASSED', () => {
    const props = getPropsBasedOnProbeStatus(FaultProbeStatus.PASSED);
    expect(props.iconName).toBe('execution-success');
    expect(props.color).toBe(Color.GREEN_700);
  });

  test('FAILED', () => {
    const props = getPropsBasedOnProbeStatus(FaultProbeStatus.FAILED);
    expect(props.iconName).toBe('execution-warning');
    expect(props.color).toBe(Color.RED_700);
  });

  test('AWAITED', () => {
    const props = getPropsBasedOnProbeStatus(FaultProbeStatus.AWAITED);
    expect(props.iconColor).toBe('execution-waiting');
    expect(props.color).toBe(Color.YELLOW_700);
  });

  test('default -> grey', () => {
    expect(getPropsBasedOnProbeStatus(FaultProbeStatus.NA)).toEqual({
      color: Color.GREY_700,
      bgColor: `var(--grey-200)`
    });
  });
});

describe('getPropsBasedOnPermissionGroup', () => {
  test('OWNER -> green', () => {
    expect(getPropsBasedOnPermissionGroup(PermissionGroup.OWNER).color).toBe(Color.GREEN_800);
  });

  test('Executor -> orange', () => {
    expect(getPropsBasedOnPermissionGroup(PermissionGroup.Executor).color).toBe(Color.ORANGE_700);
  });

  test('VIEWER -> grey', () => {
    expect(getPropsBasedOnPermissionGroup(PermissionGroup.VIEWER).color).toBe(Color.GREY_700);
  });

  test('default -> grey', () => {
    expect(getPropsBasedOnPermissionGroup('UNKNOWN' as any).color).toBe(Color.GREY_700);
  });
});
