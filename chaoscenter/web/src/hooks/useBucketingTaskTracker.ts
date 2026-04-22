import { useCallback, useEffect, useRef, useState } from 'react';
import {
  BUCKETING_POLL_INTERVAL_MS,
  BUCKETING_POLL_TIMEOUT_MS,
  BUCKETING_USE_STUB_POLL_RESPONSE,
  BUCKETING_STUB_DELAY_MS
} from '@configs/bucketingConfig';

export type BucketingTaskStatus = 'pending' | 'running' | 'completed' | 'failed' | 'unknown';

export type BucketingTaskStage = 'pending' | 'acquiring_trace' | 'running_pipeline' | 'done';

export interface BucketingTaskEntry {
  taskId: string;
  status: BucketingTaskStatus;
  stage: BucketingTaskStage;
  pollUrl: string;
  registeredAt: number;
}

export interface BucketingTaskTrackerResult {
  /** Get the bucketing task status for a given experiment run ID */
  getTaskStatus: (experimentRunID: string) => BucketingTaskEntry | undefined;
  /** Register a new task for an experiment run */
  registerTask: (experimentRunID: string, taskId: string, pollUrl: string) => void;
}

/**
 * TODO: Remove this function once the actual /api/v1/tasks/{task_id} API is implemented.
 * Returns a mock COMPLETED response matching the API spec for stub task IDs.
 */
function buildStubCompletedResponse(entry: BucketingTaskEntry): Record<string, unknown> {
  const now = new Date().toISOString();
  return {
    task_id: entry.taskId,
    agent_id: 'stub-agent',
    experiment_id: 'stub-experiment',
    run_id: 'stub-run',
    status: 'COMPLETED',
    stage: 'done',
    created_at: now,
    updated_at: now,
    started_at: now,
    completed_at: now,
    result: {
      total_observations: 0,
      total_faults_detected: 0,
      faults: [],
      storage_paths: {
        traces_dir: '',
        fault_buckets_dir: '',
        metrics_dir: '',
        summary: '',
        log: ''
      },
      token_usage: {
        bucketing_input_tokens: 0,
        bucketing_output_tokens: 0,
        extraction_input_tokens: 0,
        extraction_output_tokens: 0,
        total_tokens: 0
      },
      processing_time_seconds: 0
    }
  };
}

/**
 * Tracks bucketing-extraction task status for experiment runs.
 * Polls /api/v1/tasks/{task_id} at the configured interval.
 * Polls immediately when a new task is registered.
 * Stops polling when status is 'completed', 'failed', or the configured timeout is reached.
 */
export function useBucketingTaskTracker(): BucketingTaskTrackerResult {
  const [tasks, setTasks] = useState<Record<string, BucketingTaskEntry>>({});
  const tasksRef = useRef(tasks);
  tasksRef.current = tasks;

  const [pollTrigger, setPollTrigger] = useState(0);

  const registerTask = useCallback((experimentRunID: string, taskId: string, pollUrl: string) => {
    setTasks(prev => ({
      ...prev,
      [experimentRunID]: { taskId, status: 'pending', stage: 'pending', pollUrl, registeredAt: Date.now() }
    }));
    setPollTrigger(n => n + 1);
  }, []);

  const getTaskStatus = useCallback(
    (experimentRunID: string): BucketingTaskEntry | undefined => {
      return tasks[experimentRunID];
    },
    [tasks]
  );

  const pollActiveTasks = useCallback(async (): Promise<void> => {
    const currentTasks = tasksRef.current;
    const now = Date.now();
    const activeTasks = Object.entries(currentTasks).filter(
      ([, entry]) => entry.status === 'pending' || entry.status === 'running'
    );

    if (activeTasks.length === 0) return;

    const baseUrl = (typeof __AGENTCERT_API_BASE_URL__ !== 'undefined' && __AGENTCERT_API_BASE_URL__) || '';

    const updates: Record<string, BucketingTaskEntry> = {};

    await Promise.allSettled(
      activeTasks.map(async ([runID, entry]) => {
        // Stop polling if the configured timeout has been exceeded
        if (now - entry.registeredAt > BUCKETING_POLL_TIMEOUT_MS) {
          console.warn(`Bucketing task ${entry.taskId} timed out after ${BUCKETING_POLL_TIMEOUT_MS / 60000} minutes`);
          updates[runID] = { ...entry, status: 'failed', stage: 'done' };
          return;
        }

        // TODO: Remove this stub block once the actual /api/v1/tasks/{task_id} API is implemented.
        if (BUCKETING_USE_STUB_POLL_RESPONSE && entry.taskId.startsWith('stub-')) {
          if (now - entry.registeredAt < BUCKETING_STUB_DELAY_MS) {
            // Simulate RUNNING state until the stub delay elapses
            if (entry.status !== 'running') {
              updates[runID] = { ...entry, status: 'running', stage: 'running_pipeline' };
            }
            return;
          }
          const stubData = buildStubCompletedResponse(entry);
          console.info(`[stub] Returning mock COMPLETED for task ${entry.taskId}`, stubData);
          updates[runID] = { ...entry, status: 'completed', stage: 'done' };
          return;
        }

        try {
          const response = await fetch(`${baseUrl}${entry.pollUrl}`);
          if (!response.ok) return;
          const data = await response.json();
          const apiStatus: string = data.status ?? '';
          const apiStage: BucketingTaskStage = data.stage ?? 'pending';
          const status: BucketingTaskStatus =
            apiStatus === 'COMPLETED' ? 'completed' :
            apiStatus === 'FAILED' ? 'failed' :
            apiStatus === 'RUNNING' ? 'running' :
            apiStatus === 'PENDING' ? 'pending' : 'unknown';
          if (status !== entry.status || apiStage !== entry.stage) {
            updates[runID] = { ...entry, status, stage: apiStage };
          }
        } catch {
          // Network error - keep current status, will retry next poll
        }
      })
    );

    if (Object.keys(updates).length > 0) {
      setTasks(prev => ({ ...prev, ...updates }));
    }
  }, []);

  useEffect(() => {
    pollActiveTasks();
    const interval = setInterval(pollActiveTasks, BUCKETING_POLL_INTERVAL_MS);
    return () => clearInterval(interval);
  }, [pollTrigger, pollActiveTasks]);

  return { getTaskStatus, registerTask };
}
