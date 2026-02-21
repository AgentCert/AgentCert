import type { Audit, ResourceDetails, UserDetails } from './common';

/**
 * InjectionType represents how a fault should be injected during agent testing
 */
export enum InjectionType {
  SCHEDULED = 'SCHEDULED',
  ON_DEMAND = 'ON_DEMAND',
  CONTINUOUS = 'CONTINUOUS'
}

/**
 * FaultInjectionConfig contains configuration for how and when a fault should be injected
 */
export interface FaultInjectionConfig {
  injectionType: InjectionType;
  schedule?: string;
  duration?: string;
  targetSelector?: string;
  interval?: string;
}

/**
 * FaultInjectionConfigInput is the input type for FaultInjectionConfig
 */
export interface FaultInjectionConfigInput {
  injectionType: InjectionType;
  schedule?: string;
  duration?: string;
  targetSelector?: string;
  interval?: string;
}

/**
 * FaultSelection represents a single fault selected from a ChaosHub for inclusion in a Fault Studio
 */
export interface FaultSelection {
  faultCategory: string;
  faultName: string;
  displayName: string;
  description?: string;
  enabled: boolean;
  injectionConfig?: FaultInjectionConfig;
  customParameters?: string;
  weight: number;
}

/**
 * FaultSelectionInput is the input type for creating/updating fault selections
 */
export interface FaultSelectionInput {
  faultCategory: string;
  faultName: string;
  displayName: string;
  description?: string;
  enabled: boolean;
  injectionConfig?: FaultInjectionConfigInput;
  customParameters?: string;
  weight?: number;
}

/**
 * FaultStudio represents a configured collection of faults from a ChaosHub
 * that can be used for AI agent testing and benchmarking
 */
export interface FaultStudio extends Audit, ResourceDetails {
  id: string;
  name: string;
  description?: string;
  tags?: string[];
  projectId: string;
  sourceHubId: string;
  sourceHubName: string;
  selectedFaults: FaultSelection[];
  isActive: boolean;
  totalFaults: number;
  enabledFaults: number;
  isRemoved: boolean;
  createdAt: string;
  updatedAt: string;
  createdBy?: UserDetails;
  updatedBy?: UserDetails;
}

/**
 * FaultStudioSummary is a lightweight view of a fault studio for list operations
 */
export interface FaultStudioSummary {
  id: string;
  name: string;
  description?: string;
  projectId: string;
  sourceHubId: string;
  sourceHubName: string;
  totalFaults: number;
  enabledFaults: number;
  isActive: boolean;
  createdAt: string;
  updatedAt: string;
}

/**
 * FaultStudioFilterInput provides filtering options for listing fault studios
 */
export interface FaultStudioFilterInput {
  name?: string;
  sourceHubId?: string;
  isActive?: boolean;
  tags?: string[];
}

/**
 * FaultStudioStatsResponse contains statistics about fault studios in a project
 */
export interface FaultStudioStatsResponse {
  totalFaultStudios: number;
  activeFaultStudios: number;
  totalFaultsConfigured: number;
}

/**
 * ToggleFaultResponse is returned after enabling/disabling a fault in a studio
 */
export interface ToggleFaultResponse {
  faultStudio?: FaultStudio;
  success: boolean;
  message?: string;
}

/**
 * ListFaultStudioRequest contains parameters for listing fault studios
 */
export interface ListFaultStudioRequestInput {
  studioIds?: string[];
  filter?: FaultStudioFilterInput;
  limit?: number;
  offset?: number;
}

/**
 * CreateFaultStudioRequest contains the data needed to create a new fault studio
 */
export interface CreateFaultStudioRequestInput {
  name: string;
  description?: string;
  tags?: string[];
  sourceHubId: string;
  selectedFaults: FaultSelectionInput[];
  isActive?: boolean;
}

/**
 * UpdateFaultStudioRequest contains the data for updating an existing fault studio
 */
export interface UpdateFaultStudioRequestInput {
  name?: string;
  description?: string;
  tags?: string[];
  selectedFaults?: FaultSelectionInput[];
  isActive?: boolean;
}
