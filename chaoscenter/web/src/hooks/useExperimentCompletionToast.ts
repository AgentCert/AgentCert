import { useRef, useEffect } from 'react';
import { useToaster } from '@harnessio/uicore';
import { useStrings } from '@strings';
import { ExperimentRunStatus } from '@api/entities';

interface BucketingExtractionResponse {
  status: string;
  task_id: string;
  poll_url: string;
}

interface UseExperimentCompletionToastProps {
  phase: ExperimentRunStatus | undefined;
  experimentName: string | undefined;
  experimentID: string | undefined;
  runID: string | undefined;
  agentID: string | undefined;
}

async function submitBucketingExtraction(
  agentId: string,
  experimentId: string,
  runId: string
): Promise<BucketingExtractionResponse> {
  const baseUrl = (typeof __AGENTCERT_API_BASE_URL__ !== 'undefined' && __AGENTCERT_API_BASE_URL__) || '';
  const response = await fetch(`${baseUrl}/api/v1/bucketing-extraction`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      agent_id: agentId,
      experiment_id: experimentId,
      run_id: runId,
      trace_source: {
        type: 'langfuse'
      },
      llm_batch_size: 5,
      storage_config: { type: 'local' }
    })
  });

  if (response.status === 409) {
    const body = await response.json();
    return {
      status: 'accepted',
      task_id: body.detail.details.task_id,
      poll_url: `/api/v1/tasks/${body.detail.details.task_id}`
    };
  }

  if (!response.ok) {
    // TODO: Remove this stub once the bucketing-extraction API is live
    console.warn(`Bucketing-extraction API unavailable (${response.status}), using stub response`);
    const stubId = `stub-${experimentId}-${Date.now()}`;
    return {
      status: 'accepted',
      task_id: stubId,
      poll_url: `/api/v1/tasks/${stubId}`
    };
  }

  return response.json();
}

/**
 * Fires a toast exactly once when an experiment phase transitions
 * from a non-terminal state to a terminal state.
 * On COMPLETED, also submits a bucketing-extraction job.
 */
export function useExperimentCompletionToast({
  phase,
  experimentName,
  experimentID,
  runID,
  agentID
}: UseExperimentCompletionToastProps): void {
  const { showSuccess, showError, showWarning } = useToaster();
  const { getString } = useStrings();
  const prevPhaseRef = useRef<ExperimentRunStatus | undefined>(phase);

  useEffect(() => {
    const prev = prevPhaseRef.current;
    prevPhaseRef.current = phase;

    // Only fire when transitioning FROM a non-terminal state
    if (!phase || !experimentName) return;
    const wasRunning =
      prev === ExperimentRunStatus.RUNNING ||
      prev === ExperimentRunStatus.QUEUED ||
      prev === ExperimentRunStatus.NA ||
      prev === undefined;
    if (!wasRunning) return;

    // Check if current phase is terminal
    const isTerminal =
      phase === ExperimentRunStatus.COMPLETED ||
      phase === ExperimentRunStatus.COMPLETED_WITH_PROBE_FAILURE ||
      phase === ExperimentRunStatus.COMPLETED_WITH_ERROR ||
      phase === ExperimentRunStatus.ERROR ||
      phase === ExperimentRunStatus.STOPPED ||
      phase === ExperimentRunStatus.TIMEOUT;
    if (!isTerminal) return;

    const name = experimentName;

    // Map each terminal phase to its string keys and toast method
    const phaseConfig: Record<string, {
      successKey: string;
      failKey: string;
      fallbackKey: string;
      toastFn: typeof showSuccess;
    }> = {
      [ExperimentRunStatus.COMPLETED]: {
        successKey: 'experimentCompletedBucketing',
        failKey: 'experimentCompletedBucketingFailed',
        fallbackKey: 'experimentCompleted',
        toastFn: showSuccess
      },
      [ExperimentRunStatus.COMPLETED_WITH_PROBE_FAILURE]: {
        successKey: 'experimentCompletedWithProbeFailure',
        failKey: 'experimentCompletedWithProbeFailureBucketingFailed',
        fallbackKey: 'experimentCompletedWithProbeFailureBucketingFailed',
        toastFn: showWarning
      },
      [ExperimentRunStatus.COMPLETED_WITH_ERROR]: {
        successKey: 'experimentCompletedWithError',
        failKey: 'experimentCompletedWithErrorBucketingFailed',
        fallbackKey: 'experimentCompletedWithErrorBucketingFailed',
        toastFn: showWarning
      },
      [ExperimentRunStatus.ERROR]: {
        successKey: 'experimentError',
        failKey: 'experimentErrorBucketingFailed',
        fallbackKey: 'experimentErrorBucketingFailed',
        toastFn: showError
      },
      [ExperimentRunStatus.STOPPED]: {
        successKey: 'experimentStopped',
        failKey: 'experimentStoppedBucketingFailed',
        fallbackKey: 'experimentStoppedBucketingFailed',
        toastFn: showWarning
      },
      [ExperimentRunStatus.TIMEOUT]: {
        successKey: 'experimentTimeout',
        failKey: 'experimentTimeoutBucketingFailed',
        fallbackKey: 'experimentTimeoutBucketingFailed',
        toastFn: showWarning
      }
    };

    const config = phaseConfig[phase];
    if (!config) return;

    if (experimentID && runID && agentID) {
      submitBucketingExtraction(agentID, experimentID, runID)
        .then(resp => {
          config.toastFn(getString(config.successKey as any, { name, taskId: resp.task_id }));
        })
        .catch(() => {
          config.toastFn(getString(config.failKey as any, { name }));
        });
    } else {
      config.toastFn(getString(config.fallbackKey as any, { name }));
    }
  }, [phase]);
}
