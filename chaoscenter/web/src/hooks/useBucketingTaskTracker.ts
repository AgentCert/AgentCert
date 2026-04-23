import { useCallback, useEffect, useRef, useState } from 'react';
import {
  BUCKETING_POLL_INTERVAL_MS,
  BUCKETING_POLL_TIMEOUT_MS
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

    const updates: Record<string, BucketingTaskEntry> = {};

    await Promise.allSettled(
      activeTasks.map(async ([runID, entry]) => {
        // Stop polling if the configured timeout has been exceeded
        if (now - entry.registeredAt > BUCKETING_POLL_TIMEOUT_MS) {
          console.warn(`Bucketing task ${entry.taskId} timed out after ${BUCKETING_POLL_TIMEOUT_MS / 60000} minutes`);
          updates[runID] = { ...entry, status: 'failed', stage: 'done' };
          return;
        }

        try {
          const response = await fetch(`/agentcert-api${entry.pollUrl}`);
          if (!response.ok) {
            // 404 means the task no longer exists — mark as failed immediately
            if (response.status === 404) {
              updates[runID] = { ...entry, status: 'failed', stage: 'done' };
            }
            // Other errors (422, 500) — keep current status, will retry next poll
            return;
          }
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
