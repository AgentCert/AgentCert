import React from 'react';
import { Button, ButtonVariation, Layout, Text } from '@harnessio/uicore';
import { Color, FontVariation } from '@harnessio/design-system';
import { Dialog, ProgressBar, Intent } from '@blueprintjs/core';
import css from './CertificationProgressDialog.module.scss';

interface CertTaskResponse {
  cert_task_id: string;
  experiment_id: string;
  status: string;
  stage: string;
  created_at?: string;
  updated_at?: string;
  started_at?: string;
  completed_at?: string;
  result?: Record<string, unknown> | null;
  error?: { error_code?: string; message?: string; detail?: string } | null;
}

interface ApiErrorBody {
  status?: string;
  cert_task_id?: string;
  poll_url?: string;
  error_code?: string;
  message?: string;
  details?: Record<string, unknown>;
  detail?: string | Array<Record<string, unknown>> | { message?: string; error_code?: string; details?: Record<string, unknown> };
}

interface CertificationProgressDialogProps {
  isOpen: boolean;
  onClose: () => void;
  experimentID: string;
  experimentName?: string;
  agentID?: string;
}

export default function CertificationProgressDialog({
  isOpen,
  onClose,
  experimentID,
  experimentName: _experimentName,
  agentID
}: CertificationProgressDialogProps): React.ReactElement {
  const [phase, setPhase] = React.useState<'submitting' | 'polling' | 'done' | 'error'>('submitting');
  const [certTaskId, setCertTaskId] = React.useState<string>('');
  const [pollUrl, setPollUrl] = React.useState<string>('');
  const [taskData, setTaskData] = React.useState<CertTaskResponse | null>(null);
  const [errorMsg, setErrorMsg] = React.useState<string>('');
  const pollRef = React.useRef<ReturnType<typeof setInterval> | null>(null);

  const cleanup = (): void => {
    if (pollRef.current) {
      clearInterval(pollRef.current);
      pollRef.current = null;
    }
  };

  // Step 1: POST aggregation-certification when dialog opens
  React.useEffect(() => {
    if (!isOpen) {
      // Reset state when dialog closes
      setPhase('submitting');
      setCertTaskId('');
      setPollUrl('');
      setTaskData(null);
      setErrorMsg('');
      cleanup();
      return;
    }

    const buildPollUrl = (): string =>
      `/api/v1/cert-tasks?experiment_id=${encodeURIComponent(experimentID)}`;

    const getErrorMessage = (status: number, body: ApiErrorBody | null): string => {
      if (!body) return `Failed to start certification: ${status}`;

      if (typeof body.detail === 'string') {
        return `Failed to start certification: ${status} ${body.detail}`;
      }

      if (Array.isArray(body.detail)) {
        const first = body.detail[0] as { msg?: string } | undefined;
        return `Failed to start certification: ${status}${first?.msg ? ` ${first.msg}` : ''}`;
      }

      const nested = body.detail as { message?: string } | undefined;
      const message = body.message ?? nested?.message;
      return `Failed to start certification: ${status}${message ? ` ${message}` : ''}`;
    };

    const submitCertification = async (): Promise<void> => {
      setPhase('submitting');
      try {
        const resp = await fetch('/agentcert-api/api/v1/aggregation-certification', {
          method: 'POST',
          headers: { 'Accept': 'application/json', 'Content-Type': 'application/json' },
          body: JSON.stringify({
            agent_id: agentID ?? experimentID,
            agent_name: agentID ?? experimentID,
            experiment_id: experimentID,
            certification_run_id: `cert-${Date.now()}`,
            runs_per_fault: 30
          })
        });

        let body: ApiErrorBody | null = null;
        try {
          body = await resp.json();
        } catch {
          body = null;
        }

        if (resp.status === 409) {
          const existingTaskId =
            (body?.details?.cert_task_id as string | undefined) ??
            ((body?.detail as { details?: { cert_task_id?: string } } | undefined)?.details?.cert_task_id);
          if (existingTaskId) {
            setCertTaskId(existingTaskId);
            setPollUrl(buildPollUrl());
            setPhase('polling');
            return;
          }
        }

        if (!resp.ok) {
          setErrorMsg(getErrorMessage(resp.status, body));
          setPhase('error');
          return;
        }

        const data = body ?? {};
        setCertTaskId(data.cert_task_id ?? '');
        setPollUrl(typeof data.poll_url === 'string' ? data.poll_url : buildPollUrl());
        setPhase('polling');
      } catch (err) {
        setErrorMsg(`Network error: ${err instanceof Error ? err.message : String(err)}`);
        setPhase('error');
      }
    };

    submitCertification();

    return cleanup;
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isOpen, experimentID, agentID]);

  // Step 2: Poll cert-tasks once we have the cert_task_id
  React.useEffect(() => {
    if (phase !== 'polling' || !pollUrl) return;

    const getPollErrorMessage = async (resp: Response): Promise<string | null> => {
      try {
        const body = (await resp.json()) as ApiErrorBody;
        if (typeof body?.detail === 'string') return body.detail;
        if (Array.isArray(body?.detail)) {
          const first = body.detail[0] as { msg?: string } | undefined;
          return first?.msg ?? null;
        }
        if (body?.message) return body.message;
        const nested = body?.detail as { message?: string } | undefined;
        return nested?.message ?? null;
      } catch {
        return null;
      }
    };

    const poll = async (): Promise<void> => {
      try {
        const resp = await fetch(`/agentcert-api${pollUrl}`, {
          headers: { 'Accept': 'application/json' }
        });
        if (!resp.ok) {
          if (resp.status === 404) {
            const message = await getPollErrorMessage(resp);
            setErrorMsg(message ?? 'Certification task not found');
            setPhase('error');
            cleanup();
          }
          return;
        }

        const data = await resp.json();
        // Response could be a single object or array — find our task
        const tasks: CertTaskResponse[] = Array.isArray(data) ? data : [data];
        const ourTask = certTaskId
          ? tasks.find(t => t.cert_task_id === certTaskId)
          : tasks[0];

        if (ourTask) {
          setTaskData(ourTask);
          const status = (ourTask.status ?? '').toUpperCase();
          if (status === 'COMPLETED' || status === 'FAILED') {
            setPhase('done');
            cleanup();
          }
        }
      } catch {
        // Ignore poll errors, will retry
      }
    };

    // Poll immediately, then every 3 seconds
    poll();
    pollRef.current = setInterval(poll, 3000);

    return cleanup;
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [phase, certTaskId, pollUrl]);

  const getProgressIntent = (): Intent => {
    if (phase === 'error' || taskData?.status?.toUpperCase() === 'FAILED') return Intent.DANGER;
    if (phase === 'done' && taskData?.status?.toUpperCase() === 'COMPLETED') return Intent.SUCCESS;
    return Intent.PRIMARY;
  };

  const getProgressValue = (): number => {
    if (phase === 'submitting') return 0.1;
    if (phase === 'error') return 1;
    if (phase === 'done') return 1;
    // Map stages to approximate progress
    const stage = (taskData?.stage ?? '').toLowerCase();
    if (stage.includes('fetch')) return 0.25;
    if (stage.includes('aggregat')) return 0.5;
    if (stage.includes('generat') || stage.includes('certif')) return 0.75;
    if (stage.includes('stor')) return 0.9;
    if (stage.includes('complet') || stage.includes('done')) return 0.95;
    return 0.3;
  };

  const getStatusLabel = (): string => {
    if (phase === 'submitting') return 'Submitting certification request...';
    if (phase === 'error') return 'Error';
    if (phase === 'done') {
      return taskData?.status?.toUpperCase() === 'COMPLETED'
        ? 'Certification complete!'
        : 'Certification failed';
    }
    const stage = taskData?.stage ?? 'pending';
    return `In progress: ${stage}`;
  };

  return (
    <Dialog
      isOpen={isOpen}
      canOutsideClickClose={phase === 'done' || phase === 'error'}
      canEscapeKeyClose={phase === 'done' || phase === 'error'}
      onClose={onClose}
      title="Agent Certification"
      className={css.certProgressDialog}
    >
      <Layout.Vertical padding="xlarge" spacing="large" className={css.body}>
        {/* Status label */}
        <Text font={{ variation: FontVariation.H6 }} color={Color.GREY_800}>
          {getStatusLabel()}
        </Text>

        {/* Progress bar */}
        <ProgressBar
          intent={getProgressIntent()}
          value={getProgressValue()}
          animate={phase === 'submitting' || phase === 'polling'}
          stripes={phase === 'submitting' || phase === 'polling'}
        />

        {/* Task details */}
        {certTaskId && (
          <Text font={{ size: 'small' }} color={Color.GREY_500}>
            Cert Task ID: {certTaskId}
          </Text>
        )}

        {taskData && (
          <Layout.Vertical spacing="small" className={css.details}>
            <DetailRow label="Status" value={taskData.status} />
            <DetailRow label="Stage" value={taskData.stage} />
            {taskData.started_at && <DetailRow label="Started" value={taskData.started_at} />}
            {taskData.completed_at && <DetailRow label="Completed" value={taskData.completed_at} />}
          </Layout.Vertical>
        )}

        {/* Error details */}
        {phase === 'error' && (
          <Text font={{ size: 'small' }} color={Color.RED_600} className={css.errorText}>
            {errorMsg}
          </Text>
        )}
        {phase === 'done' && taskData?.error && (
          <Layout.Vertical spacing="xsmall" className={css.errorBlock}>
            <Text font={{ size: 'small', weight: 'semi-bold' }} color={Color.RED_600}>
              Error: {taskData.error.error_code ?? 'Unknown'}
            </Text>
            <Text font={{ size: 'small' }} color={Color.RED_600}>
              {taskData.error.message}
            </Text>
          </Layout.Vertical>
        )}

        {/* Result */}
        {phase === 'done' && taskData?.status?.toUpperCase() === 'COMPLETED' && taskData.result && (
          <Layout.Vertical spacing="xsmall" className={css.resultBlock}>
            <Text font={{ size: 'small', weight: 'semi-bold' }} color={Color.GREEN_700}>
              Certification Result:
            </Text>
            <pre className={css.resultPre}>{JSON.stringify(taskData.result, null, 2)}</pre>
          </Layout.Vertical>
        )}

        {/* Close button */}
        <Layout.Horizontal flex={{ justifyContent: 'flex-end' }}>
          <Button
            variation={ButtonVariation.TERTIARY}
            text={phase === 'done' || phase === 'error' ? 'Close' : 'Cancel'}
            onClick={onClose}
          />
        </Layout.Horizontal>
      </Layout.Vertical>
    </Dialog>
  );
}

function DetailRow({ label, value }: { label: string; value?: string }): React.ReactElement {
  return (
    <Layout.Horizontal spacing="small">
      <Text font={{ size: 'small', weight: 'semi-bold' }} color={Color.GREY_700} style={{ minWidth: 80 }}>
        {label}:
      </Text>
      <Text font={{ size: 'small' }} color={Color.GREY_600}>
        {value ?? '—'}
      </Text>
    </Layout.Horizontal>
  );
}
