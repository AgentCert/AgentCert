import type { PaginationProps } from '@harnessio/uicore';
import type { ExperimentRunFaultStatus, ExperimentRunStatus, UserDetails } from '@api/entities';

export interface DataExtractionStatus {
  status: string; // PENDING | RUNNING | COMPLETED | FAILED | NOT_FOUND | UNKNOWN
  stage: string;  // pending | acquiring_trace | running_pipeline | done
}

interface ProbeStatus {
  passed: number;
  failed: number;
  na: number;
}
export interface ExperimentRunFaultDetails {
  faultID: string;
  faultName: string;
  faultStatus: ExperimentRunFaultStatus;
  faultWeight: number;
  probeStatus: ProbeStatus;
  startedAt?: number;
  finishedAt?: number;
}

export interface ExperimentRunFaultDetailsTableProps {
  experimentID: string;
  experimentRunID: string;
  content: Array<ExperimentRunFaultDetails>;
}

export interface ExperimentRunDetails {
  experimentID: string;
  experimentRunName: string;
  experimentRunID: string;
  experimentStatus: ExperimentRunStatus;
  executedBy: UserDetails | undefined;
  resilienceScore: number | undefined;
  startedAt: number;
  finishedAt: number;
  executedAt: number;
  faultTableData: ExperimentRunFaultDetailsTableProps;
}

export interface ExperimentRunHistoryTableProps {
  content: Array<ExperimentRunDetails>;
  pagination?: PaginationProps;
  dataExtractionStatuses?: Record<string, DataExtractionStatus>;
}
