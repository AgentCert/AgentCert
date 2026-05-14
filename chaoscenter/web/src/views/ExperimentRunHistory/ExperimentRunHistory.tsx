import React from 'react';
import { Container, Layout, TabNavigation, Text } from '@harnessio/uicore';
import { Color, FontVariation } from '@harnessio/design-system';
import { Tooltip as BPTooltip } from '@blueprintjs/core';
import { useParams } from 'react-router-dom';
import ColumnChart from '@components/ColumnChart/ColumnChart';
import { useStrings } from '@strings';
import DefaultLayoutTemplate from '@components/DefaultLayout';
import type { ExperimentRunHistoryTableProps } from '@controllers/ExperimentRunHistory';
import Loader from '@components/Loader';
import NoFilteredData from '@components/NoFilteredData';
import FallbackBox from '@images/FallbackBox.svg';
import type { ColumnData } from '@components/ColumnChart/ColumnChart.types';
import { GenericErrorHandler } from '@errors';
import { getScope } from '@utils';
import { useRouteWithBaseUrl } from '@hooks';
import { StudioTabs } from '@models';
import { MemoisedExperimentRunHistoryTable } from './ExperimentRunHistoryTable';

interface MultiRunConfig {
  maxRuns: number;
  currentRun: number;
  completedRuns: number;
  totalRuns: number;
}

interface ExperimentRunHistoryViewProps {
  statusDropDown: React.ReactElement;
  dateRangePicker: React.ReactElement;
  experimentRunSearchBar: React.ReactElement;
  resetFilterButton: React.ReactElement;
  rightSideBar?: React.ReactElement;
  experimentName: string | undefined;
  experimentRunsTableData: ExperimentRunHistoryTableProps | undefined;
  experimentRunsColumnGraphData: ColumnData[] | undefined;
  loading: boolean;
  areFiltersSet: boolean;
  experimentRunsExists: boolean | undefined;
  multiRunConfig?: MultiRunConfig | null;
  certificateDownloadEnabled?: boolean;
  certificateAgentID?: string;
}

const ExperimentRunHistoryView = ({
  statusDropDown,
  dateRangePicker,
  experimentRunSearchBar,
  resetFilterButton,
  rightSideBar,
  experimentName,
  experimentRunsTableData,
  experimentRunsColumnGraphData,
  loading,
  areFiltersSet,
  experimentRunsExists,
  multiRunConfig,
  certificateDownloadEnabled,
  certificateAgentID
}: ExperimentRunHistoryViewProps): React.ReactElement => {
  const scope = getScope();
  const paths = useRouteWithBaseUrl();
  const { getString } = useStrings();
  const { experimentID } = useParams<{ experimentID: string }>();

  const headerTitle = loading && !experimentName ? undefined : experimentName ?? experimentID;

  const handleDownloadCertificate = React.useCallback(() => {
    if (!certificateDownloadEnabled || !certificateAgentID) return;
    const url = `/api/certification/pdf?agent_id=${encodeURIComponent(
      certificateAgentID
    )}&experiment_id=${encodeURIComponent(experimentID)}`;
    // Open in a new tab so the current page state is preserved.
    window.open(url, '_blank', 'noopener,noreferrer');
  }, [certificateDownloadEnabled, certificateAgentID, experimentID]);

  const downloadCertificateLink = headerTitle ? (
    <BPTooltip
      content={
        certificateDownloadEnabled
          ? 'Download the certification report (PDF) for this experiment'
          : 'Certificate will be available once every run for this experiment has finished.'
      }
      position="bottom"
    >
      <Layout.Horizontal
        spacing="xsmall"
        flex={{ alignItems: 'center', justifyContent: 'flex-start' }}
        style={{
          cursor: certificateDownloadEnabled ? 'pointer' : 'not-allowed',
          opacity: certificateDownloadEnabled ? 1 : 0.6,
          userSelect: 'none',
          padding: '4px 10px',
          borderRadius: 4,
          border: `1px solid ${certificateDownloadEnabled ? 'var(--primary-7)' : 'var(--grey-300)'}`,
          backgroundColor: certificateDownloadEnabled ? 'var(--primary-1)' : 'var(--grey-100)',
          transition: 'background-color 0.15s ease, border-color 0.15s ease'
        }}
        onClick={handleDownloadCertificate}
      >
        <Text
          font={{ size: 'xsmall', weight: 'semi-bold' }}
          color={certificateDownloadEnabled ? Color.PRIMARY_7 : Color.GREY_500}
          style={{ letterSpacing: 0.2, lineHeight: '14px' }}
          icon="download"
          iconProps={{
            size: 12,
            color: certificateDownloadEnabled ? Color.PRIMARY_7 : Color.GREY_500
          }}
        >
          Download Certificate
        </Text>
      </Layout.Horizontal>
    </BPTooltip>
  ) : null;

  const titleNode = headerTitle ? (
    <Layout.Horizontal spacing="medium" flex={{ alignItems: 'center', justifyContent: 'flex-start' }}>
      <Text font={{ variation: FontVariation.H4 }}>{headerTitle}</Text>
      {downloadCertificateLink}
    </Layout.Horizontal>
  ) : (
    headerTitle
  );

  const breadcrumbs = [
    {
      label: getString('chaosExperiments'),
      url: paths.toExperiments()
    }
  ];

  if (experimentRunsExists !== undefined && !experimentRunsExists && !areFiltersSet)
    return (
      <GenericErrorHandler
        errStatusCode={400}
        errorMessage={getString('genericResourceNotFoundError', {
          resource: getString('experimentID'),
          resourceID: experimentID,
          projectID: scope.projectID
        })}
      />
    );

  return (
    <DefaultLayoutTemplate
      title={titleNode}
      breadcrumbs={breadcrumbs}
      rightSideBar={rightSideBar}
      headerToolbar={
        <Container style={{ marginTop: '-2rem' }}>
          <TabNavigation
            size={'small'}
            links={[
              {
                label: getString('chaosStudio'),
                to: paths.toEditExperiment({ experimentKey: experimentID }) + `?tab=${StudioTabs.BUILDER}`
              },
              {
                label: getString('runHistory'),
                to: paths.toExperimentRunHistory({ experimentID: experimentID })
              }
            ]}
          />
        </Container>
      }
    >
      <Layout.Vertical spacing={'medium'} padding={{ left: 'small', right: 'small' }}>
        <Layout.Horizontal flex={{ justifyContent: 'space-between' }}>
          {statusDropDown}
          <Layout.Horizontal spacing={'medium'}>
            {experimentRunSearchBar}
            {dateRangePicker}
          </Layout.Horizontal>
        </Layout.Horizontal>
        <Text font={{ variation: FontVariation.H5 }}>{getString('resilienceScoreTrends')}</Text>
        {/* <!-- Column Chart goes here--> */}
        <ColumnChart
          xAxisLabel="Runs"
          yAxisLabel="Resilience Score"
          gridLines={[0, 35, 65, 100]}
          data={experimentRunsColumnGraphData}
          isLoading={loading}
        />
      </Layout.Vertical>
      <Layout.Vertical margin={{ top: 'xlarge' }} spacing={'medium'} padding={{ left: 'small', right: 'small' }}>
        <Layout.Horizontal flex={{ justifyContent: 'space-between' }}>
          <Layout.Horizontal spacing={'medium'} flex={{ alignItems: 'center' }}>
            <Text font={{ variation: FontVariation.H5 }}>
              {getString('experimentRuns')}
              {` (${experimentRunsTableData?.pagination?.itemCount ?? ''})`}
            </Text>
            {multiRunConfig && (
              <Text
                font={{ variation: FontVariation.BODY }}
                color={Color.PRIMARY_7}
                style={{
                  backgroundColor: 'var(--primary-1)',
                  padding: '4px 12px',
                  borderRadius: '4px',
                  fontWeight: 600
                }}
              >
                Multi-Run: {experimentRunsTableData?.pagination?.itemCount ?? 0}/{multiRunConfig.totalRuns}
              </Text>
            )}
          </Layout.Horizontal>
          <Layout.Horizontal spacing={'medium'}>{/* {statusDropDown} */}</Layout.Horizontal>
        </Layout.Horizontal>
        <Container height={'calc(100vh - 444px)'}>
          <Loader loading={loading}>
            {experimentRunsTableData && experimentRunsTableData.content.length ? (
              // <!-- Run History Table goes here -->
              <MemoisedExperimentRunHistoryTable {...experimentRunsTableData} />
            ) : areFiltersSet ? (
              // <!-- No data after setting filters -->
              <NoFilteredData resetButton={resetFilterButton} />
            ) : (
              // <!-- No data -->
              <Layout.Vertical flex={{ justifyContent: 'center' }} height={'100%'}>
                <img src={FallbackBox} alt={getString('latestRun')} />
                <Text font={{ variation: FontVariation.BODY1 }} padding={{ top: 'large' }} color={Color.GREY_500}>
                  {getString('latestRunFallbackText')}
                </Text>
              </Layout.Vertical>
            )}
          </Loader>
        </Container>
      </Layout.Vertical>
    </DefaultLayoutTemplate>
  );
};

export default ExperimentRunHistoryView;
