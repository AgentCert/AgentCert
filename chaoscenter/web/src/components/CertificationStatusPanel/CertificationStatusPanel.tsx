import React from 'react';
import { Button, ButtonVariation, Layout, Text } from '@harnessio/uicore';
import { Color, FontVariation } from '@harnessio/design-system';
import type { CertificationExperimentSummary } from '@api/core';

type StageState = 'pending' | 'running' | 'done' | 'failed';

interface Stage {
  label: string;
  state: StageState;
  detail: string;
}

function plural(n: number, word: string): string {
  return n === 1 ? `1 ${word}` : `${n} ${word}s`;
}

// describeBucketing builds the detail line for the Analysing Run stage.
// totalRuns is the actual number of run-workflow docs (may exceed expectedRuns
// when replacement runs are added after a failure or after a cert is extended).
function describeBucketing(
  completedRuns: number,
  expectedRuns: number,
  failedRuns: number,
  totalRuns: number
): string {
  // Use the larger of expectedRuns and totalRuns as the visible denominator so
  // that replacement / extra runs show correctly (e.g. "30/31 done").
  const denominator = Math.max(expectedRuns, totalRuns);
  const inProgress = denominator - completedRuns - failedRuns;
  if (failedRuns > 0 && inProgress > 0) {
    return `${completedRuns}/${denominator} analysed · ${plural(inProgress, 'run')} in progress · ${plural(failedRuns, 'run')} failed`;
  }
  if (failedRuns > 0) {
    return `${completedRuns}/${denominator} analysed · ${plural(failedRuns, 'run')} failed`;
  }
  if (completedRuns >= denominator) {
    return `All ${plural(denominator, 'run')} analysed`;
  }
  return `${completedRuns} of ${plural(denominator, 'run')} analysed`;
}

// Two-step pipeline:
//   Step 1 — Analysing Run: fault bucketing triggered for every run independently.
//   Step 2 — Generating Certificate: triggered once enough runs have been bucketed.
function derivePipelineStages(cert: CertificationExperimentSummary | null | undefined): Stage[] {
  if (!cert) {
    return [
      { label: 'Analysing Run', state: 'pending', detail: 'Waiting for runs to complete' },
      { label: 'Generating Certificate', state: 'pending', detail: 'Pending' }
    ];
  }

  const { status, completedRuns, expectedRuns, failedRuns, totalRuns } = cert;
  const denominator = Math.max(expectedRuns, totalRuns);
  const inProgressRuns = denominator - completedRuns - failedRuns;
  const bucketingDetail = describeBucketing(completedRuns, expectedRuns, failedRuns, totalRuns);

  switch (status) {
    case 'RUNS_IN_PROGRESS': {
      const allStalled = failedRuns > 0 && inProgressRuns === 0;
      const certDetail = failedRuns > 0
        ? `Blocked — retry ${plural(failedRuns, 'failed run')} or add replacement runs`
        : 'Waiting for all runs to be analysed';
      return [
        { label: 'Analysing Run', state: allStalled ? 'failed' : 'running', detail: bucketingDetail },
        { label: 'Generating Certificate', state: 'pending', detail: certDetail }
      ];
    }
    case 'READY_FOR_AGGREGATION':
    case 'AGGREGATION_IN_PROGRESS':
      return [
        { label: 'Analysing Run', state: 'done', detail: bucketingDetail },
        { label: 'Generating Certificate', state: 'running', detail: 'Aggregating run data and generating report…' }
      ];
    case 'AGGREGATION_FAILED':
      return [
        { label: 'Analysing Run', state: 'done', detail: bucketingDetail },
        { label: 'Generating Certificate', state: 'failed', detail: 'Failed — check the certifier task for details' }
      ];
    case 'EXPERIMENT_CERTIFICATE_READY': {
      const certDetail = cert.generatedAt
        ? `Ready · ${new Date(cert.generatedAt).toLocaleString()}`
        : 'Ready';
      return [
        { label: 'Analysing Run', state: 'done', detail: bucketingDetail },
        { label: 'Generating Certificate', state: 'done', detail: certDetail }
      ];
    }
    default:
      return [
        { label: 'Analysing Run', state: 'running', detail: bucketingDetail },
        { label: 'Generating Certificate', state: 'pending', detail: 'Pending' }
      ];
  }
}

// Inline SVG icons avoid any @harnessio/icons import in this lazy chunk,
// which prevents a webpack TDZ circular-dependency at load time.
function StageIcon({ state }: Readonly<{ state: StageState }>): React.ReactElement {
  const size = 16;
  switch (state) {
    case 'done':
      return (
        <svg width={size} height={size} viewBox="0 0 16 16" fill="none" style={{ flexShrink: 0 }}>
          <circle cx="8" cy="8" r="8" fill="#27AE60" />
          <path d="M4.5 8l2.5 2.5 4.5-4.5" stroke="white" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      );
    case 'running':
      return (
        <svg width={size} height={size} viewBox="0 0 16 16" fill="none" style={{ flexShrink: 0, animation: 'spin 1s linear infinite' }}>
          <circle cx="8" cy="8" r="6.5" stroke="#6938EF" strokeWidth="1.5" strokeDasharray="10 10" strokeLinecap="round" />
        </svg>
      );
    case 'failed':
      return (
        <svg width={size} height={size} viewBox="0 0 16 16" fill="none" style={{ flexShrink: 0 }}>
          <circle cx="8" cy="8" r="8" fill="#E11900" />
          <path d="M5.5 5.5l5 5M10.5 5.5l-5 5" stroke="white" strokeWidth="1.5" strokeLinecap="round" />
        </svg>
      );
    default:
      return (
        <svg width={size} height={size} viewBox="0 0 16 16" fill="none" style={{ flexShrink: 0 }}>
          <circle cx="8" cy="8" r="6.5" stroke="#B0B7C3" strokeWidth="1.5" />
        </svg>
      );
  }
}

function stateColor(state: StageState): Color {
  switch (state) {
    case 'done': return Color.GREEN_700;
    case 'running': return Color.PRIMARY_7;
    case 'failed': return Color.RED_600;
    default: return Color.GREY_500;
  }
}

export interface CertificationStatusPanelProps {
  certStatus: CertificationExperimentSummary | null | undefined;
  // Actual total experiment runs from the list API — may exceed certStatus.expectedRuns
  // when the experiment is extended after a certificate was already issued.
  totalExperimentRuns?: number;
  pendingBucketingCount: number;
  retriggerLoading: boolean;
  onRetrigger: () => void;
}

const SPIN_STYLE = '@keyframes spin { to { transform: rotate(360deg); } }';

export default function CertificationStatusPanel({
  certStatus,
  totalExperimentRuns,
  pendingBucketingCount,
  retriggerLoading,
  onRetrigger
}: Readonly<CertificationStatusPanelProps>): React.ReactElement {
  const stages = derivePipelineStages(certStatus);
  const status = certStatus?.status;
  const isReady = status === 'EXPERIMENT_CERTIFICATE_READY';
  const hasBucketingFailure = (certStatus?.failedRuns ?? 0) > 0;

  // New runs exist beyond what's already in the bucketing pipeline.
  // Use totalRuns (run-workflow doc count) not expectedRuns (gate threshold):
  // a cert covering 6 runs with gate=5 leaves expectedRuns=5, which would
  // make hasNewRuns permanently true and show Re-generate forever.
  const alreadyInPipeline = certStatus?.totalRuns ?? certStatus?.expectedRuns ?? 0;
  const hasNewRuns = (totalExperimentRuns ?? 0) > alreadyInPipeline;

  // Show re-trigger only when user action is actually needed:
  //   • bucketing failures exist (runs need to be retried)
  //   • cert is ready but new runs were added (coverage needs extending)
  // Do NOT show during normal in-progress bucketing — nothing to act on yet.
  const showRetrigger =
    certStatus != null &&
    status !== 'AGGREGATION_IN_PROGRESS' &&
    status !== 'READY_FOR_AGGREGATION' &&
    (hasBucketingFailure || (isReady && hasNewRuns));

  let retriggerLabel = 'Re-trigger';
  if (hasNewRuns && isReady) {
    retriggerLabel = `Re-generate (${plural((totalExperimentRuns ?? 0) - certifiedCount, 'new run')})`;
  } else if (hasBucketingFailure) {
    retriggerLabel = `Retry ${plural(pendingBucketingCount, 'failed run')}`;
  }

  let borderColor = 'var(--primary-2)';
  let bgColor = 'var(--primary-1)';
  if (isReady && !hasNewRuns) {
    borderColor = 'var(--green-200)';
    bgColor = 'var(--green-50)';
  } else if (hasBucketingFailure) {
    borderColor = 'var(--red-200)';
    bgColor = 'var(--red-50)';
  }

  return (
    <Layout.Vertical
      spacing="xsmall"
      padding={{ top: 'small', bottom: 'small', left: 'medium', right: 'medium' }}
      style={{ background: bgColor, borderRadius: 6, border: `1px solid ${borderColor}` }}
    >
      <style>{SPIN_STYLE}</style>
      <Layout.Horizontal flex={{ justifyContent: 'space-between', alignItems: 'center' }}>
        <Text font={{ variation: FontVariation.SMALL_SEMI }} color={Color.GREY_700}>
          Certification Status
        </Text>
        {showRetrigger && (
          <Button
            variation={ButtonVariation.SECONDARY}
            text={retriggerLabel}
            icon="repeat"
            iconProps={{ size: 12 }}
            loading={retriggerLoading}
            onClick={onRetrigger}
            minimal
            small
          />
        )}
      </Layout.Horizontal>

      <Layout.Horizontal spacing="large">
        {stages.map((stage, idx) => (
          <Layout.Horizontal key={stage.label} spacing="xsmall" style={{ alignItems: 'flex-start' }}>
            {idx > 0 && (
              <Text color={Color.GREY_300} font={{ variation: FontVariation.SMALL }} style={{ marginRight: 4, marginLeft: -4, lineHeight: '18px' }}>
                →
              </Text>
            )}
            <span style={{ display: 'flex', paddingTop: 2 }}>
              <StageIcon state={stage.state} />
            </span>
            <Layout.Vertical spacing="none">
              <Text font={{ variation: FontVariation.SMALL_SEMI }} color={stateColor(stage.state)}>
                {stage.label}
              </Text>
              <Text font={{ variation: FontVariation.TINY }} color={Color.GREY_500}>
                {stage.detail}
              </Text>
            </Layout.Vertical>
          </Layout.Horizontal>
        ))}
      </Layout.Horizontal>
    </Layout.Vertical>
  );
}
