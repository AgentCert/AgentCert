import { useToaster } from '@harnessio/uicore';
import React from 'react';
import { useParams } from 'react-router-dom';
import { ExecutionData, ExperimentType, ExperimentRunStatus } from '@api/entities';
import { cronEnabled, getScope } from '@utils';
import { useExperimentCompletionToast } from '@hooks';
import ExperimentRunDetailsView from '@views/ExperimentRunDetails';
import RightSideBarV2 from '@components/RightSideBarV2';
import { getExperimentRun } from '@api/core/experiments/getExperimentRun';

export default function ExperimentRunDetailsController(): React.ReactElement {
  const { experimentID, runID, notifyID } = useParams<{ experimentID: string; runID: string; notifyID: string }>();
  const scope = getScope();
  const { showError } = useToaster();

  const {
    data: listExperimentRunData,
    loading: listExperimentRunLoading,
    exists: specificRunExists,
    startPolling,
    stopPolling
  } = getExperimentRun({
    ...scope,
    experimentRunID: runID,
    notifyID,
    options: { onError: error => showError(error.message) }
  });

  const specificRunData = listExperimentRunData?.getExperimentRun;

  // Extract agentId from the experiment manifest workflow parameters.
  // Falls back to infraID if agentId param is not present (agent not registered).
  const agentID = React.useMemo(() => {
    if (!specificRunData?.experimentManifest) return undefined;
    try {
      const manifest = JSON.parse(specificRunData.experimentManifest);
      const params: { name: string; value?: string }[] = manifest?.spec?.arguments?.parameters ?? [];
      return params.find(p => p.name === 'agentId')?.value;
    } catch {
      return undefined;
    }
  }, [specificRunData?.experimentManifest]);

  useExperimentCompletionToast({
    phase: specificRunData?.phase,
    experimentName: specificRunData?.experimentName,
    experimentID,
    runID: specificRunData?.experimentRunID ?? runID,
    agentID: agentID ?? specificRunData?.infra?.infraID
  });

  React.useEffect(() => {
    if (
      specificRunData?.phase === ExperimentRunStatus.RUNNING ||
      specificRunData?.phase === ExperimentRunStatus.QUEUED ||
      specificRunData?.phase === ExperimentRunStatus.NA
    ) {
      startPolling(3000);
    } else {
      stopPolling();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [specificRunData]);

  const executionData =
    specificRunExists && specificRunData?.executionData.length
      ? (JSON.parse(specificRunData.executionData) as ExecutionData)
      : undefined;

  const parsedManifest =
    specificRunData && specificRunData?.experimentManifest ? JSON.parse(specificRunData.experimentManifest) : undefined;
  const isCronEnabled =
    specificRunExists && specificRunData?.experimentType === ExperimentType.CRON && cronEnabled(parsedManifest);

  const rightSideBarV2 = (
    <RightSideBarV2
      experimentID={experimentID}
      experimentRunID={runID}
      notifyID={notifyID}
      isCronEnabled={isCronEnabled}
      phase={specificRunData?.phase}
      experimentType={specificRunData?.experimentType}
    />
  );

  return (
    <ExperimentRunDetailsView
      experimentID={experimentID}
      runSequence={specificRunData?.runSequence}
      experimentRunID={specificRunData?.experimentRunID ?? runID}
      runExists={specificRunExists}
      infra={specificRunData?.infra}
      experimentName={specificRunData?.experimentName}
      experimentExecutionDetails={executionData}
      manifest={specificRunData?.experimentManifest}
      phase={specificRunData?.phase as ExperimentRunStatus}
      resiliencyScore={specificRunData?.resiliencyScore}
      rightSideBar={rightSideBarV2}
      loading={{
        listExperimentRun: listExperimentRunLoading
      }}
    />
  );
}
