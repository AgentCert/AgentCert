import React, { useEffect } from 'react';
import { useParams } from 'react-router-dom';
import { Button, ButtonVariation, Container, Layout, Text } from '@harnessio/uicore';
import { Color, FontVariation } from '@harnessio/design-system';
import { useGetRun } from '@api/core';
import { useRouteWithBaseUrl } from '@hooks';
import { useHistory } from 'react-router-dom';
import DefaultLayoutTemplate from '@components/DefaultLayout';
import { getScope } from '@utils';

const TERMINAL_STATUSES = new Set(['COMPLETED', 'FAILED', 'ABORTED']);

const STATUS_COLORS: Record<string, string> = {
  QUEUED: '#f39c12',
  RUNNING: '#4a90e2',
  COMPLETED: '#27ae60',
  FAILED: '#e74c3c',
  ABORTED: '#95a5a6'
};

function StatusBadge({ status }: { status: string }): React.ReactElement {
  return (
    <span
      style={{
        display: 'inline-block',
        padding: '3px 10px',
        borderRadius: 12,
        background: STATUS_COLORS[status] ?? '#95a5a6',
        color: '#fff',
        fontSize: 12,
        fontWeight: 700,
        letterSpacing: 0.5
      }}
    >
      {status}
    </span>
  );
}

export default function RunStatusPage(): React.ReactElement {
  const scope = getScope();
  const history = useHistory();
  const paths = useRouteWithBaseUrl();
  const { runID } = useParams<{ runID: string }>();

  const { data, startPolling, stopPolling } = useGetRun({
    variables: { projectID: scope.projectID, runID },
    skip: !runID
  });

  const run = data?.getRun;
  const isTerminal = run ? TERMINAL_STATUSES.has(run.status) : false;

  useEffect(() => {
    if (isTerminal) {
      stopPolling();
    } else {
      startPolling(5000);
    }
    return () => stopPolling();
  }, [isTerminal, startPolling, stopPolling]);

  const breadcrumbs = [
    { label: 'Chaos Studio', url: paths.toChaosStudioNew() }
  ];

  const rowStyle: React.CSSProperties = {
    display: 'flex',
    padding: '8px 0',
    borderBottom: '1px solid #f0f2f5'
  };
  const labelCellStyle: React.CSSProperties = {
    width: 180,
    flexShrink: 0,
    fontWeight: 600,
    fontSize: 13,
    color: '#6b7280'
  };
  const valueCellStyle: React.CSSProperties = { fontSize: 13, color: '#344054' };

  if (!run) {
    return (
      <DefaultLayoutTemplate
        title="Run Status"
        breadcrumbs={breadcrumbs}
        loading={!data && !!runID}
      >
        <Container padding="xlarge">
          <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_500}>
            {runID ? `Loading run ${runID}...` : 'Run ID not specified'}
          </Text>
        </Container>
      </DefaultLayoutTemplate>
    );
  }

  return (
    <DefaultLayoutTemplate
      title={`Run: ${run.runID.slice(0, 12)}...`}
      breadcrumbs={breadcrumbs}
      subTitle={`Experiment: ${run.definitionName}`}
    >
      <Container padding="xlarge">
        <Layout.Vertical spacing="large">
          {/* Status header */}
          <Layout.Horizontal
            flex={{ justifyContent: 'space-between', alignItems: 'center' }}
          >
            <Layout.Horizontal spacing="medium" flex={{ alignItems: 'center' }}>
              <Text font={{ variation: FontVariation.H4 }} color={Color.GREY_800}>
                {run.definitionName}
              </Text>
              <StatusBadge status={run.status} />
            </Layout.Horizontal>
            <Layout.Horizontal spacing="small">
              {!isTerminal && (
                <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_400}>
                  Polling every 5s...
                </Text>
              )}
              <Button
                variation={ButtonVariation.TERTIARY}
                text="Back to Studio"
                onClick={() => history.push(paths.toChaosStudioNew())}
              />
            </Layout.Horizontal>
          </Layout.Horizontal>

          {/* Run details table */}
          <div
            style={{
              background: '#fff',
              border: '1px solid #e8eaed',
              borderRadius: 8,
              padding: 20
            }}
          >
            <Text font={{ variation: FontVariation.H6 }} color={Color.GREY_700}>
              Run Details
            </Text>
            <div style={{ marginTop: 12 }}>
              {[
                { label: 'Run ID', value: run.runID },
                { label: 'Experiment', value: `${run.definitionName} v${run.definitionVersion}` },
                { label: 'Agent', value: `${run.agentName} v${run.agentVersion}` },
                { label: 'Model', value: `${run.modelUsed} (${run.modelProvider})` },
                { label: 'Argo Workflow', value: run.argoWorkflowName },
                run.langfuseTraceId
                  ? { label: 'Langfuse Trace', value: run.langfuseTraceId, isLink: true }
                  : null,
                run.certifierReportId
                  ? { label: 'Certifier Report', value: run.certifierReportId }
                  : null,
                { label: 'Started At', value: run.startedAt ?? '—' },
                { label: 'Completed At', value: run.completedAt ?? '—' }
              ]
                .filter(Boolean)
                .map(row => {
                  if (!row) return null;
                  return (
                    <div key={row.label} style={rowStyle}>
                      <span style={labelCellStyle}>{row.label}</span>
                      <span style={valueCellStyle}>{row.value}</span>
                    </div>
                  );
                })}
            </div>
          </div>

          {/* Status history */}
          {run.statusHistory && run.statusHistory.length > 0 && (
            <div
              style={{
                background: '#fff',
                border: '1px solid #e8eaed',
                borderRadius: 8,
                padding: 20
              }}
            >
              <Text font={{ variation: FontVariation.H6 }} color={Color.GREY_700}>
                Status History
              </Text>
              <Layout.Vertical spacing="xsmall" style={{ marginTop: 12 }}>
                {run.statusHistory.map((evt, idx) => (
                  <Layout.Horizontal
                    key={idx}
                    spacing="medium"
                    flex={{ alignItems: 'center' }}
                    style={{
                      padding: '6px 0',
                      borderBottom:
                        idx < run.statusHistory.length - 1
                          ? '1px solid #f0f2f5'
                          : 'none'
                    }}
                  >
                    <StatusBadge status={evt.status} />
                    <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>
                      {new Date(evt.timestamp).toLocaleString()}
                    </Text>
                    {evt.reason && (
                      <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_600}>
                        — {evt.reason}
                      </Text>
                    )}
                  </Layout.Horizontal>
                ))}
              </Layout.Vertical>
            </div>
          )}
        </Layout.Vertical>
      </Container>
    </DefaultLayoutTemplate>
  );
}
