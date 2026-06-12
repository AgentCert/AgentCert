import React from 'react';
import { Layout, Text, useToaster, Utils } from '@harnessio/uicore';
import { useParams } from 'react-router-dom';
import { Color } from '@harnessio/design-system';
import { isEqual } from 'lodash-es';
import { listExperimentRunForHistory, getCertificationStatus, generateCertification } from '@api/core';
import { getScope, getColorBasedOnResilienceScore, cronEnabled } from '@utils';
import ExperimentRunHistoryView from '@views/ExperimentRunHistory';
import { useStrings } from '@strings';
import { ExperimentRun, ExperimentRunStatus, ExperimentType } from '@api/entities';
import type { ColumnData } from '@components/ColumnChart/ColumnChart.types';
import {
  initialExperimentRunFilterState,
  useExperimentRunsFilter,
  useSearchParams,
  useUpdateSearchParams,
  useRouteWithBaseUrl
} from '@hooks';
import RightSideBarV2 from '@components/RightSideBarV2';
import type { UseRouteDefinitionsProps } from '@routes/RouteDefinitions';
import {
  DateRangePicker,
  ExperimentRunSearchBar,
  FilterProps,
  StatusDropDown,
  ResetFilterButton
} from './ExperimentRunFilter';
import type { ExperimentRunHistoryTableProps } from './types';
import { generateExperimentRunTableContent } from './helpers';

const Tooltip = ({ experimentRun }: { experimentRun: ExperimentRun }): React.ReactElement => {
  const { getString } = useStrings();
  const TooltipRow = ({
    property,
    value
  }: {
    property: string;
    value: string | number | undefined;
  }): React.ReactElement => (
    <Layout.Horizontal flex={{ alignItems: 'baseline', justifyContent: 'flex-start' }}>
      <Text color={Color.BLACK} font={{ size: 'normal', weight: 'semi-bold' }} width={135}>
        {property} :
      </Text>
      <Text font={{ size: 'normal', weight: 'semi-bold' }}>{value}</Text>
    </Layout.Horizontal>
  );
  return (
    <Layout.Vertical spacing={'small'}>
      <TooltipRow property={getString('experimentRunID')} value={experimentRun.experimentRunID.slice(0, 8)} />
      <TooltipRow property={getString('resiliencyScore')} value={`${experimentRun.resiliencyScore}% `} />
    </Layout.Vertical>
  );
};

function generateColumnGraphData(
  experimentRunsWithExecutionData: Array<ExperimentRun>,
  paths: UseRouteDefinitionsProps
): ColumnData[] {
  const content: ColumnData[] = experimentRunsWithExecutionData.map(individualRun => {
    return {
      color: Utils.getRealCSSColor(getColorBasedOnResilienceScore(individualRun.resiliencyScore).primary),
      height: individualRun.resiliencyScore ?? 0,
      path: paths.toExperimentRunDetails({
        experimentID: individualRun.experimentID,
        runID: individualRun.experimentRunID
      }),
      popoverContent: <Tooltip experimentRun={individualRun} />
    };
  });
  return content;
}

export default function ExperimentRunHistoryController(): React.ReactElement {
  const scope = getScope();
  const { state, dispatch } = useExperimentRunsFilter();
  const { showError, showSuccess } = useToaster();
  const searchParams = useSearchParams();
  const updateSearchParams = useUpdateSearchParams();
  const { experimentID } = useParams<{ experimentID: string }>();
  const paths = useRouteWithBaseUrl();

  // State for pagination
  const page = parseInt(searchParams.get('page') ?? '0');
  const limit = Math.min(parseInt(searchParams.get('limit') ?? '15'), 30);

  const setPage = (newPage: number): void => updateSearchParams({ page: newPage.toString() });
  const setLimit = (newLimit: number): void => updateSearchParams({ limit: newLimit.toString() });
  const resetPage = (): void => {
    page !== 0 && updateSearchParams({ page: '0' });
  };

  //state to use experiment name from cache if after search api returned no data
  const [experimentNamePersistent, setExperimentNamePersistent] = React.useState<string>();

  const {
    data: experimentRunData,
    loading: listExperimentRunsLoading,
    exists: experimentRunsExists,
    refetch: refetchExperimentRuns
  } = listExperimentRunForHistory({
    ...scope,
    pagination: { page: page, limit: limit },
    experimentIDs: [experimentID],
    options: {
      onError: error => showError(error.message),
      nextFetchPolicy: 'cache-first',
      pollInterval: 10000
    },
    filter: {
      // TODO: update state names now that filters are updated
      experimentRunID: state.experimentRunID,
      experimentRunStatus: state.experimentRunStatus,
      dateRange: state.dateRange
    }
  });

  const totalExperimentRuns = experimentRunData?.listExperimentRun?.totalNoOfExperimentRuns;
  const experimentRunsWithExecutionData = experimentRunData?.listExperimentRun?.experimentRuns;
  const experimentName = experimentRunsWithExecutionData?.[0]?.experimentName;
  const experimentPhase = experimentRunsWithExecutionData?.[0]?.phase;
  const experimentType = experimentRunsWithExecutionData?.[0]?.experimentType;
  const experimentManifest = experimentRunsWithExecutionData?.[0]?.experimentManifest;

  const agentID = experimentRunsWithExecutionData?.[0]?.infra?.infraID ?? '';
  const agentName = experimentRunsWithExecutionData?.[0]?.infra?.name ?? agentID;

  const hasAnyRuns = (experimentRunsWithExecutionData?.length ?? 0) > 0;

  const { data: certStatusData } = getCertificationStatus({
    projectID: scope.projectID,
    experimentID,
    options: {
      // Poll as soon as there are any runs so the panel shows live progress.
      // Always keep polling — cert status can revert to RUNS_IN_PROGRESS when
      // the experiment is extended with more runs after a certificate was issued.
      skip: !hasAnyRuns || !experimentID,
      pollInterval: 10000
    }
  });

  const certReady = certStatusData?.getCertificationStatus?.ready === true;
  const certificateDownload = { enabled: certReady && !!agentID, agentID };

  // Manual re-trigger — shows success toast so user knows the action fired.
  const [generateCertificationMutation, { loading: retriggerLoading }] = generateCertification({
    onCompleted: () => showSuccess('Certification pipeline re-triggered'),
    onError: err => showError(err.message)
  });

  // Silent auto-trigger — no success toast; errors still surface.
  const [autoTriggerMutation] = generateCertification({
    onError: err => showError(err.message)
  });

  // Memoize so parsedManifest identity is stable between renders.
  const parsedManifest = React.useMemo(
    () => (experimentManifest ? JSON.parse(experimentManifest) : null),
    [experimentManifest]
  );

  const isCronEnabled =
    experimentRunsWithExecutionData && experimentType === ExperimentType.CRON && cronEnabled(parsedManifest);

  // Multi-run config extracted from manifest annotations.
  const multiRunConfig = React.useMemo(() => {
    if (!parsedManifest?.metadata?.annotations) return null;
    const annotations = parsedManifest.metadata.annotations;
    if (annotations['litmuschaos.io/multiRunEnabled'] !== 'true') return null;

    const maxRuns = Number.parseInt(annotations['litmuschaos.io/maxRuns'] || '1', 10);
    const currentRun = Number.parseInt(annotations['litmuschaos.io/currentRun'] || '0', 10);
    const completedRuns = experimentRunsWithExecutionData?.filter(
      run => run.phase !== ExperimentRunStatus.RUNNING && run.phase !== ExperimentRunStatus.QUEUED
    ).length ?? 0;

    return { maxRuns, currentRun, completedRuns, totalRuns: maxRuns };
  }, [parsedManifest, experimentRunsWithExecutionData]);

  // Shared terminal-phase set used by both auto-trigger and manual re-trigger.
  // Includes the deprecated COMPLETED_WITH_ERROR so old runs stored in the DB
  // are still bucketed. TIMEOUT is terminal — a timed-out run produces data.
  const TERMINAL_PHASES = React.useMemo(() => new Set<string>([
    ExperimentRunStatus.COMPLETED,
    ExperimentRunStatus.COMPLETED_WITH_PROBE_FAILURE,
    ExperimentRunStatus.COMPLETED_WITH_ERROR,
    ExperimentRunStatus.ERROR,
    ExperimentRunStatus.STOPPED,
    ExperimentRunStatus.TIMEOUT
  ]), []);

  // Tracks which run IDs have already been sent to the cert pipeline this
  // session so the effect below doesn't re-fire on every 10 s poll tick.
  const autoTriggeredRunsRef = React.useRef(new Set<string>());

  // Auto-trigger bucketing for every newly terminal run.
  //
  // Safety contract: we wait for certStatusData to be defined before acting.
  // autoTriggeredRunsRef starts empty on every page load — without this guard,
  // loading a page where a cert is already done would re-fire generateCertification
  // for every run, calling ResetStatusIfCertified and trashing the existing cert.
  //
  // Logic:
  //   • cert ready + no new runs beyond expectedRuns → page loaded for an already-
  //     certified experiment, nothing to do.
  //   • otherwise → fire for terminal runs not yet in the session ref.
  //
  // The backend is idempotent: UpsertRunWorkflowInitial uses $setOnInsert,
  // triggerBucketing skips when Bucketing.TaskID is already set.
  React.useEffect(() => {
    if (!hasAnyRuns || !experimentID || !scope.projectID) return;
    // Block until cert status is loaded — undefined means the query hasn't returned yet.
    if (certStatusData === undefined) return;

    const cs = certStatusData?.getCertificationStatus;
    const isAlreadyCertified = cs?.status === 'EXPERIMENT_CERTIFICATE_READY';
    const allTerminalRuns = (experimentRunsWithExecutionData ?? []).filter(
      r => TERMINAL_PHASES.has(r.phase)
    );

    // Cert is ready and no runs exist beyond what was certified — nothing to trigger.
    // This prevents re-triggering all runs every time the page is opened.
    if (isAlreadyCertified && allTerminalRuns.length <= (cs?.expectedRuns ?? 0)) return;

    const newlyTerminal = allTerminalRuns.filter(
      r => !autoTriggeredRunsRef.current.has(r.experimentRunID)
    );
    if (newlyTerminal.length === 0) return;

    const resolvedAgentID = cs?.agentID ?? agentID;
    // cs.agentName is stored via $setOnInsert on first call; fall back to
    // infra.name so the cert doc gets the display name, not the infraID.
    const resolvedAgentName = cs?.agentName ?? agentName;
    const expectedRuns = multiRunConfig?.maxRuns ?? totalExperimentRuns ?? newlyTerminal.length;

    newlyTerminal.forEach(run => {
      autoTriggeredRunsRef.current.add(run.experimentRunID);
      autoTriggerMutation({
        variables: {
          projectID: scope.projectID,
          request: {
            agentID: resolvedAgentID,
            agentName: resolvedAgentName,
            experimentID,
            experimentRunID: run.experimentRunID,
            expectedRuns
          }
        }
      });
    });
  }, [experimentRunsWithExecutionData, hasAnyRuns, experimentID, scope.projectID, agentID, agentName,
      certStatusData, multiRunConfig, totalExperimentRuns, TERMINAL_PHASES, autoTriggerMutation]);

  // Manual re-trigger: fires for all terminal runs with the updated expectedRuns,
  // and registers them in the ref so the auto-trigger effect doesn't double-fire.
  const handleRetrigger = React.useCallback(() => {
    const runs = experimentRunsWithExecutionData ?? [];
    const cs = certStatusData?.getCertificationStatus;
    const resolvedAgentID = cs?.agentID ?? agentID;
    const resolvedAgentName = cs?.agentName ?? agentName;
    const expectedRuns = multiRunConfig?.maxRuns ?? totalExperimentRuns ?? runs.length;
    const terminalRuns = runs.filter(r => TERMINAL_PHASES.has(r.phase));

    terminalRuns.forEach(run => {
      // Mark in ref so the auto-trigger effect doesn't re-fire for the same runs.
      autoTriggeredRunsRef.current.add(run.experimentRunID);
      generateCertificationMutation({
        variables: {
          projectID: scope.projectID,
          request: {
            agentID: resolvedAgentID,
            agentName: resolvedAgentName,
            experimentID,
            experimentRunID: run.experimentRunID,
            expectedRuns
          }
        }
      });
    });
  }, [experimentRunsWithExecutionData, multiRunConfig, totalExperimentRuns, certStatusData,
      agentID, agentName, experimentID, scope.projectID, TERMINAL_PHASES, generateCertificationMutation]);

  React.useEffect(() => {
    if (experimentName) setExperimentNamePersistent(experimentName);
  }, [experimentName]);

  const experimentRunsTableData: ExperimentRunHistoryTableProps | undefined = experimentRunsWithExecutionData && {
    content: generateExperimentRunTableContent(experimentRunsWithExecutionData),
    pagination: {
      gotoPage: event => setPage(event),
      itemCount: totalExperimentRuns ?? 0,
      pageCount: totalExperimentRuns ? Math.ceil(totalExperimentRuns / limit) : 1,
      pageIndex: page,
      pageSizeOptions: [...new Set([15, 30, limit])].sort(),
      pageSize: limit,
      onPageSizeChange: event => setLimit(event)
    }
  };

  const experimentRunsColumnGraphData =
    experimentRunsWithExecutionData && generateColumnGraphData(experimentRunsWithExecutionData, paths);

  const filterProps: FilterProps = {
    state,
    dispatch,
    resetPage
  };

  const areFiltersSet = !(isEqual(state, initialExperimentRunFilterState) && page === 0);

  const rightSideBarV2 = (
    <RightSideBarV2
      refetchExperimentRuns={refetchExperimentRuns}
      experimentID={experimentID}
      phase={experimentPhase}
      isCronEnabled={isCronEnabled}
      experimentType={experimentType}
      certificateDownloadEnabled={certificateDownload.enabled}
      certificateAgentID={certificateDownload.agentID}
    />
  );

  return (
    <ExperimentRunHistoryView
      statusDropDown={<StatusDropDown {...filterProps} />}
      dateRangePicker={<DateRangePicker {...filterProps} />}
      experimentRunSearchBar={<ExperimentRunSearchBar {...filterProps} />}
      resetFilterButton={<ResetFilterButton {...filterProps} />}
      loading={listExperimentRunsLoading}
      rightSideBar={rightSideBarV2}
      experimentName={experimentName ?? experimentNamePersistent}
      experimentRunsTableData={experimentRunsTableData}
      experimentRunsColumnGraphData={experimentRunsColumnGraphData}
      areFiltersSet={areFiltersSet}
      experimentRunsExists={experimentRunsExists}
      multiRunConfig={multiRunConfig}
      certStatus={certStatusData?.getCertificationStatus}
      totalExperimentRuns={totalExperimentRuns}
      retriggerLoading={retriggerLoading}
      onRetrigger={handleRetrigger}
    />
  );
}
