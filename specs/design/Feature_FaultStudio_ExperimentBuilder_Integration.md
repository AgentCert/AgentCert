# Fault Studio + Experiment Builder Integration - Design Document

## Table of Contents
1. [Overview](#overview)
2. [Goals & Non-Goals](#goals--non-goals)
3. [Current Architecture](#current-architecture)
4. [Proposed Changes](#proposed-changes)
5. [User Experience](#user-experience)
6. [Technical Design](#technical-design)
7. [Implementation Phases](#implementation-phases)
8. [Data Flow](#data-flow)
9. [API Changes](#api-changes)
10. [UI Wireframes](#ui-wireframes)
11. [Testing Strategy](#testing-strategy)
12. [Rollout Plan](#rollout-plan)

---

## Overview

### What is this Integration?

This feature integrates **Fault Studio** (custom fault collections) with the **Experiment Builder** (Chaos Studio), allowing users to:
- Quickly add pre-configured faults from their Fault Studios directly into experiments
- Reuse fault configurations across multiple experiments without manual reconfiguration
- Maintain consistency in chaos testing by using standardized fault configurations

### Problem Statement

Currently, when creating chaos experiments:
1. Users must select faults from ChaosHub each time they create an experiment
2. Fault tunables (duration, target, interval) must be configured manually for every experiment
3. There's no way to reuse a "pre-configured fault package" across experiments

**Fault Studio** solves part of this by allowing users to create collections of pre-configured faults. This integration extends that capability by making those collections directly accessible during experiment creation.

---

## Goals & Non-Goals

### Goals
1. ✅ Allow users to select faults from Fault Studios during experiment creation
2. ✅ Apply pre-configured injection settings from Fault Studio to experiments
3. ✅ Support adding multiple faults from a studio at once (bulk add)
4. ✅ Maintain backward compatibility - existing ChaosHub-based fault selection remains functional
5. ✅ Enable filtering to show only enabled faults from studios
6. ✅ Allow users to override pre-configured settings if needed

### Non-Goals
1. ❌ Automatic experiment generation from entire studios (future enhancement)
2. ❌ Two-way sync (changes in experiment do NOT update Fault Studio)
3. ❌ Nested studios or studio inheritance
4. ❌ Studio versioning (future enhancement)

---

## Current Architecture

### Experiment Creation Flow (As-Is)

```
┌──────────────────────────────────────────────────────────────────────────┐
│                      EXPERIMENT CREATION FLOW                            │
└──────────────────────────────────────────────────────────────────────────┘

User → ChaosStudio → ExperimentVisualBuilder → [+] Add Fault Button
                                                      │
                                                      ▼
                                        ExperimentCreationSelectFaultController
                                                      │
                          ┌───────────────────────────┴───────────────────────────┐
                          │                                                       │
                          ▼                                                       ▼
                   listChaosHub()                                    listChaosFaults()
                          │                                                       │
                          ▼                                                       ▼
            ExperimentCreationSelectFaultView ◄───────────────────────────────────┘
                          │
                          │ (Drawer with Hub List + Fault Categories)
                          │
                          ▼
            ExperimentCreationChaosFaultsController
                          │
                          ▼
                    FaultData (faultName, faultCR, engineCR, weight)
                          │
                          ▼
            handleFaultSelection(faultData) → addFaultsToManifest()
                          │
                          ▼
            TuneFaultDrawer (Configure ENV vars, Probes, Advanced)
```

### Key Components

| Component | Location | Purpose |
|-----------|----------|---------|
| `ChaosStudioView` | `views/ChaosStudio/ChaosStudio.tsx` | Main experiment creation UI |
| `ExperimentVisualBuilder` | `views/ExperimentVisualBuilder/` | DAG-based experiment builder |
| `ExperimentCreationSelectFaultController` | `controllers/ExperimentCreationSelectFault/` | Fault selection orchestration |
| `ExperimentCreationSelectFaultView` | `views/ExperimentCreationSelectFault/` | Drawer UI for fault selection |
| `KubernetesYamlService` | `services/experiment/KubernetesYamlService.ts` | YAML manipulation & fault addition |

### FaultData Interface (Current)

```typescript
interface FaultData {
  faultName: string;      // e.g., "pod-delete"
  faultCR?: ChaosExperiment; // ChaosExperiment CR YAML
  engineCR?: ChaosEngine;    // ChaosEngine CR YAML
  weight?: number;           // Fault weight (default: 10)
  probes?: ProbeObj[];       // Attached probes
}
```

---

## Proposed Changes

### High-Level Architecture (To-Be)

```
┌──────────────────────────────────────────────────────────────────────────┐
│                  ENHANCED EXPERIMENT CREATION FLOW                       │
└──────────────────────────────────────────────────────────────────────────┘

User → ChaosStudio → ExperimentVisualBuilder → [+] Add Fault Button
                                                      │
                                                      ▼
                                        ExperimentCreationSelectFaultController
                                                      │
                    ┌─────────────────────────────────┼─────────────────────────────────┐
                    │                                 │                                 │
                    ▼                                 ▼                                 ▼
          ┌─────────────────┐            ┌─────────────────┐            ┌─────────────────┐
          │   TAB: ChaosHub │            │ TAB: Fault      │            │ TAB: Templates  │
          │   (Existing)    │            │      Studios    │            │   (Future)      │
          │                 │            │     (NEW)       │            │                 │
          └─────────────────┘            └─────────────────┘            └─────────────────┘
                    │                                 │
                    │                                 ▼
                    │                    listFaultStudios()
                    │                           │
                    │                           ▼
                    │               FaultStudioSelector
                    │               (Shows Studios + Faults)
                    │                           │
                    │                           ▼
                    │               Select Faults (Multi-Select)
                    │                           │
                    └───────────────────────────┼
                                                │
                                                ▼
                                    FaultData[] (array for bulk add)
                                                │
                                                ▼
                              addMultipleFaultsToManifest() (NEW)
                                                │
                                                ▼
                                    TuneFaultDrawer (per fault)
```

### Key Changes Summary

| Area | Change | Type |
|------|--------|------|
| UI | Add "Fault Studios" tab in fault selection drawer | New |
| UI | Studio list with expandable fault sections | New |
| UI | Multi-select checkboxes for bulk fault addition | New |
| API | `listFaultStudiosWithFaultDetails` query | New |
| Service | `addMultipleFaultsToManifest()` function | New |
| Model | Extended FaultData to include injection config | Modified |

---

## User Experience

### User Journey: Adding Faults from Studio

```
1. User opens Experiment Builder (Visual mode)
   │
2. User clicks [+ Add Fault] button
   │
3. Drawer opens with tabs: "ChaosHub" | "Fault Studios"
   │
4. User clicks "Fault Studios" tab
   │
5. User sees list of their active Fault Studios
   ├── Network Faults (3 enabled / 5 total)
   ├── CPU Stress Tests (2 enabled / 2 total)
   └── Database Chaos (4 enabled / 6 total)
   │
6. User expands "Network Faults" studio
   ├── ✓ pod-network-loss (enabled) ─┐
   ├── ✓ pod-network-latency (enabled) ──┼── Pre-configured settings shown
   ├── ✗ pod-network-corrupt (disabled) ─┘   (duration, target, etc.)
   └── ...
   │
7. User selects checkboxes for faults they want
   │
8. User clicks [Add Selected Faults] button
   │
9. Faults are added to experiment DAG with pre-configured settings
   │
10. (Optional) User can click any fault node to modify tunables
```

### Wireframe: Fault Studios Tab

```
┌────────────────────────────────────────────────────────────────────────────┐
│ Select Chaos Faults                                              [Search] │
├──────────────────────────┬─────────────────────────────────────────────────┤
│                          │                                                 │
│ ┌──────────────────────┐ │  ┌───────────────────────────────────────────┐  │
│ │ Tabs:                │ │  │ My Fault Studios (3)                      │  │
│ │ [ChaosHub] [Studios] │ │  │                                           │  │
│ └──────────────────────┘ │  │ ▼ Network Faults                          │  │
│                          │  │   Source: Litmus ChaosHub                 │  │
│ Studios:                 │  │   3 enabled / 5 total                     │  │
│                          │  │                                           │  │
│ > Network Faults (3/5)   │  │   ☑ pod-network-loss                     │  │
│ > CPU Tests (2/2)        │  │      Duration: 30s | Target: app=nginx   │  │
│ > Database (4/6)         │  │                                           │  │
│                          │  │   ☑ pod-network-latency                  │  │
│ [+ Create New Studio]    │  │      Duration: 60s | Latency: 200ms      │  │
│                          │  │                                           │  │
│                          │  │   ☐ pod-network-corrupt (disabled)       │  │
│                          │  │      ─────────────────────────            │  │
│                          │  │                                           │  │
│                          │  │                                           │  │
│                          │  └───────────────────────────────────────────┘  │
│                          │                                                 │
│                          │        [Add 2 Selected Faults to Experiment]   │
├──────────────────────────┴─────────────────────────────────────────────────┤
│ ℹ️ Tip: Faults will be added with their pre-configured settings.          │
│    You can modify them after adding to the experiment.                     │
└────────────────────────────────────────────────────────────────────────────┘
```

---

## Technical Design

### 1. Extended FaultData Interface

```typescript
// models/experiment.ts - MODIFIED
interface FaultData {
  faultName: string;
  faultCR?: ChaosExperiment;
  engineCR?: ChaosEngine;
  weight?: number;
  probes?: ProbeObj[];
  
  // NEW: From Fault Studio integration
  fromStudio?: {
    studioId: string;
    studioName: string;
    injectionConfig?: InjectionConfig;
  };
}

interface InjectionConfig {
  injectionType: 'continuous' | 'scheduled' | 'one-time';
  schedule?: string;      // Cron expression for scheduled
  duration: string;       // e.g., "30s", "5m"
  targetSelector?: string; // e.g., "app=nginx"
  interval?: string;      // For repeated injections
}
```

### 2. New GraphQL Query

```graphql
# fault_studio.graphqls - NEW QUERY

"""
Get Fault Studios with full fault details for experiment integration.
Returns enabled faults with their ChaosEngine and ChaosExperiment CRs.
"""
query listFaultStudiosForExperiment(
  $projectID: ID!
  $request: ListFaultStudiosForExperimentRequest
): ListFaultStudiosForExperimentResponse!

input ListFaultStudiosForExperimentRequest {
  # Only return active studios
  activeOnly: Boolean = true
  # Only return enabled faults within each studio
  enabledFaultsOnly: Boolean = true
}

type ListFaultStudiosForExperimentResponse {
  faultStudios: [FaultStudioWithDetails!]!
}

type FaultStudioWithDetails {
  id: ID!
  name: String!
  description: String
  sourceHubId: ID!
  sourceHubName: String!
  isActive: Boolean!
  faults: [FaultWithCRs!]!
}

type FaultWithCRs {
  faultCategory: String!
  faultName: String!
  displayName: String
  description: String
  enabled: Boolean!
  weight: Int
  injectionConfig: InjectionConfig
  
  # Full CRs for experiment manifest generation
  faultCR: String!      # ChaosExperiment YAML as string
  engineCR: String!     # ChaosEngine YAML as string
  csv: String           # ChartServiceVersion for metadata
}
```

### 3. Backend Service Extension

```go
// pkg/fault_studio/service.go - NEW FUNCTION

// ListFaultStudiosForExperiment retrieves studios with full fault CRs
// for integration with the experiment builder
func (s *Service) ListFaultStudiosForExperiment(
    ctx context.Context,
    projectID string,
    request *model.ListFaultStudiosForExperimentRequest,
) (*model.ListFaultStudiosForExperimentResponse, error) {
    
    // 1. Get all active studios for the project
    studios, err := s.faultStudioOperator.ListFaultStudios(ctx, projectID, true)
    if err != nil {
        return nil, err
    }
    
    // 2. For each studio, fetch fault CRs from source ChaosHub
    var result []*model.FaultStudioWithDetails
    for _, studio := range studios {
        faultsWithCRs := make([]*model.FaultWithCRs, 0)
        
        for _, fault := range studio.SelectedFaults {
            // Skip disabled faults if enabledFaultsOnly is true
            if request.EnabledFaultsOnly && !fault.Enabled {
                continue
            }
            
            // Fetch fault details from ChaosHub
            faultDetails, err := s.chaosHubService.GetChaosFault(
                ctx, 
                studio.SourceHubID, 
                fault.FaultCategory,
                fault.FaultName,
                projectID,
            )
            if err != nil {
                // Log error but continue with other faults
                logrus.Warnf("Failed to fetch fault %s: %v", fault.FaultName, err)
                continue
            }
            
            // Apply injection config to engine CR
            engineCR := s.applyInjectionConfig(faultDetails.Engine, fault.InjectionConfig)
            
            faultsWithCRs = append(faultsWithCRs, &model.FaultWithCRs{
                FaultCategory:   fault.FaultCategory,
                FaultName:       fault.FaultName,
                DisplayName:     fault.DisplayName,
                Description:     fault.Description,
                Enabled:         fault.Enabled,
                Weight:          fault.Weight,
                InjectionConfig: fault.InjectionConfig,
                FaultCR:         faultDetails.Fault,
                EngineCR:        engineCR,
                Csv:             faultDetails.Csv,
            })
        }
        
        result = append(result, &model.FaultStudioWithDetails{
            ID:            studio.StudioID,
            Name:          studio.Name,
            Description:   studio.Description,
            SourceHubID:   studio.SourceHubID,
            SourceHubName: studio.SourceHubName,
            IsActive:      studio.IsActive,
            Faults:        faultsWithCRs,
        })
    }
    
    return &model.ListFaultStudiosForExperimentResponse{
        FaultStudios: result,
    }, nil
}

// applyInjectionConfig modifies the ChaosEngine CR with studio's injection settings
func (s *Service) applyInjectionConfig(engineYAML string, config *model.InjectionConfig) string {
    if config == nil {
        return engineYAML
    }
    
    // Parse engine YAML
    var engine map[string]interface{}
    if err := yaml.Unmarshal([]byte(engineYAML), &engine); err != nil {
        return engineYAML
    }
    
    // Apply duration to spec.chaosServiceAccount or relevant field
    // Apply target selectors
    // Apply scheduling settings
    
    modifiedYAML, _ := yaml.Marshal(engine)
    return string(modifiedYAML)
}
```

### 4. Frontend API Hook

```typescript
// api/core/faultStudio/listFaultStudiosForExperiment.ts - NEW

import { gql, useLazyQuery, QueryLazyOptions } from '@apollo/client';
import { FaultStudioWithDetails } from '@api/entities';

export const LIST_FAULT_STUDIOS_FOR_EXPERIMENT = gql`
  query listFaultStudiosForExperiment(
    $projectID: ID!
    $request: ListFaultStudiosForExperimentRequest
  ) {
    listFaultStudiosForExperiment(projectID: $projectID, request: $request) {
      faultStudios {
        id
        name
        description
        sourceHubId
        sourceHubName
        isActive
        faults {
          faultCategory
          faultName
          displayName
          description
          enabled
          weight
          injectionConfig {
            injectionType
            schedule
            duration
            targetSelector
            interval
          }
          faultCR
          engineCR
          csv
        }
      }
    }
  }
`;

export interface ListFaultStudiosForExperimentRequest {
  activeOnly?: boolean;
  enabledFaultsOnly?: boolean;
}

export interface ListFaultStudiosForExperimentResponse {
  listFaultStudiosForExperiment: {
    faultStudios: FaultStudioWithDetails[];
  };
}

export function listFaultStudiosForExperimentLazyQuery(
  options?: QueryLazyOptions<ListFaultStudiosForExperimentResponse>
) {
  return useLazyQuery<ListFaultStudiosForExperimentResponse>(
    LIST_FAULT_STUDIOS_FOR_EXPERIMENT,
    options
  );
}
```

### 5. Enhanced KubernetesYamlService

```typescript
// services/experiment/KubernetesYamlService.ts - NEW FUNCTION

/**
 * Add multiple faults from a Fault Studio to the experiment manifest.
 * Handles bulk addition with pre-configured settings.
 */
async addMultipleFaultsFromStudio(
  key: ChaosObjectStoresPrimaryKeys['experiments'],
  faults: FaultData[],
  insertAfterNodeId?: string
): Promise<Experiment | undefined> {
  try {
    const tx = (await this.db).transaction(ChaosObjectStoreNameMap.EXPERIMENTS, 'readwrite');
    const store = tx.objectStore(ChaosObjectStoreNameMap.EXPERIMENTS);
    const experiment = await store.get(key);
    if (!experiment) return;

    experiment.unsavedChanges = true;
    const [templates, steps] = this.getTemplatesAndSteps(
      experiment?.manifest as KubernetesExperimentManifest
    );

    // Find insertion point
    let insertIndex = steps ? steps.length - 1 : 0;
    if (insertAfterNodeId) {
      steps?.forEach((step, i) => {
        step.forEach(s => {
          if (s.template === insertAfterNodeId) insertIndex = i + 1;
        });
      });
    }

    // Add each fault sequentially in the DAG
    for (const fault of faults) {
      const { faultName, faultCR, engineCR, weight, fromStudio } = fault;

      // Add to steps
      steps?.splice(insertIndex, 0, [{ name: faultName, template: faultName }]);
      insertIndex++;

      // Add to install-chaos-faults artifacts
      const installArtifacts = templates?.find(t => t.name === 'install-chaos-faults')?.inputs?.artifacts;
      if (faultCR?.spec?.definition) {
        faultCR.spec.definition.image = updateContainerImage(
          faultCR.spec.definition.image,
          experiment.imageRegistry
        );
      }
      installArtifacts?.push({
        name: faultName,
        path: `/tmp/${faultName}.yaml`,
        raw: { data: yamlStringify(faultCR) }
      });

      // Process engine with studio injection config
      let processedEngine = engineCR;
      if (fromStudio?.injectionConfig) {
        processedEngine = this.applyStudioInjectionConfig(engineCR, fromStudio.injectionConfig);
      }
      const [updatedEngine] = this.postProcessChaosEngineManifest(processedEngine, faultName);

      // Add engine template
      templates?.push({
        name: faultName,
        inputs: {
          artifacts: [{
            name: faultName,
            path: `/tmp/chaosengine-${faultName}.yaml`,
            raw: { data: yamlStringify(updatedEngine) }
          }]
        },
        metadata: {
          labels: { 
            weight: (weight ?? 10).toString(),
            fromStudio: fromStudio?.studioId ?? ''
          }
        },
        container: { /* ... standard container config ... */ }
      });
    }

    await store.put(experiment);
    await tx.done;
    return experiment;
  } catch (error) {
    console.error('Failed to add faults from studio:', error);
    throw error;
  }
}

/**
 * Apply Fault Studio's injection configuration to ChaosEngine
 */
private applyStudioInjectionConfig(
  engine: ChaosEngine | undefined,
  config: InjectionConfig
): ChaosEngine | undefined {
  if (!engine || !config) return engine;

  const modified = { ...engine };
  
  // Apply duration to experiments
  if (config.duration && modified.spec?.experiments) {
    modified.spec.experiments = modified.spec.experiments.map(exp => ({
      ...exp,
      spec: {
        ...exp.spec,
        components: {
          ...exp.spec?.components,
          env: this.updateEnvVar(
            exp.spec?.components?.env ?? [],
            'TOTAL_CHAOS_DURATION',
            this.parseDurationToSeconds(config.duration)
          )
        }
      }
    }));
  }

  // Apply target selector
  if (config.targetSelector && modified.spec) {
    const [key, value] = config.targetSelector.split('=');
    modified.spec.appinfo = {
      ...modified.spec.appinfo,
      applabel: `${key}=${value}`
    };
  }

  return modified;
}
```

---

## Implementation Phases

### Phase 9: API Layer (Backend)

| Task | Description | Effort |
|------|-------------|--------|
| 9.1 | Add `ListFaultStudiosForExperimentRequest` input type to GraphQL schema | 0.5h |
| 9.2 | Add `FaultStudioWithDetails` and `FaultWithCRs` types to schema | 1h |
| 9.3 | Add `listFaultStudiosForExperiment` query to schema | 0.5h |
| 9.4 | Implement `ListFaultStudiosForExperiment` in service layer | 2h |
| 9.5 | Implement `applyInjectionConfig` helper function | 1h |
| 9.6 | Add resolver for new query | 0.5h |
| 9.7 | Add RBAC rule for new query | 0.5h |
| 9.8 | Regenerate gqlgen files | 0.5h |
| 9.9 | Unit tests for service function | 1h |

**Phase 9 Total: ~8 hours**

---

### Phase 10: Frontend API Layer

| Task | Description | Effort |
|------|-------------|--------|
| 10.1 | Create `listFaultStudiosForExperiment.ts` API hook | 0.5h |
| 10.2 | Add `FaultStudioWithDetails` interface to entities | 0.5h |
| 10.3 | Add `FaultWithCRs` interface to entities | 0.5h |
| 10.4 | Export from `api/core/faultStudio/index.ts` | 0.25h |
| 10.5 | Add string constants for new UI text | 0.25h |

**Phase 10 Total: ~2 hours**

---

### Phase 11: Fault Studios Tab Component

| Task | Description | Effort |
|------|-------------|--------|
| 11.1 | Create `FaultStudiosTab` view component | 2h |
| 11.2 | Create `FaultStudiosTab.module.scss` styles | 1h |
| 11.3 | Create `FaultStudioCard` sub-component (expandable) | 1.5h |
| 11.4 | Create `FaultCheckboxList` sub-component (multi-select) | 1.5h |
| 11.5 | Create `FaultStudiosTabController` with data fetching | 1h |
| 11.6 | Add loading skeleton states | 0.5h |
| 11.7 | Add empty state (no studios) | 0.5h |

**Phase 11 Total: ~8 hours**

---

### Phase 12: Integrate Tab into Fault Selection Drawer

| Task | Description | Effort |
|------|-------------|--------|
| 12.1 | Modify `ExperimentCreationSelectFaultView` to add tabs | 1h |
| 12.2 | Update `ExperimentCreationSelectFaultController` to handle tab state | 0.5h |
| 12.3 | Add tab switching logic and state management | 0.5h |
| 12.4 | Wire up `FaultStudiosTab` as second tab content | 0.5h |
| 12.5 | Style tab navigation bar | 0.5h |

**Phase 12 Total: ~3 hours**

---

### Phase 13: Bulk Fault Addition Logic

| Task | Description | Effort |
|------|-------------|--------|
| 13.1 | Add `addMultipleFaultsFromStudio` to `KubernetesYamlService` | 2h |
| 13.2 | Add `applyStudioInjectionConfig` helper | 1h |
| 13.3 | Modify `ExperimentVisualBuilder` to handle bulk selection | 1h |
| 13.4 | Add progress indicator for multi-fault addition | 0.5h |
| 13.5 | Handle errors during bulk addition (partial success) | 1h |
| 13.6 | Update DAG visualization after bulk add | 0.5h |

**Phase 13 Total: ~6 hours**

---

### Phase 14: Tune Faults After Addition

| Task | Description | Effort |
|------|-------------|--------|
| 14.1 | Open tune drawer after first fault is added | 0.5h |
| 14.2 | Add "Next Fault" / "Previous Fault" navigation | 1h |
| 14.3 | Show studio origin badge in tune drawer | 0.5h |
| 14.4 | Pre-populate tunables from studio's injection config | 1h |
| 14.5 | Allow override of studio settings | 0.5h |

**Phase 14 Total: ~3.5 hours**

---

### Phase 15: Polish & Edge Cases

| Task | Description | Effort |
|------|-------------|--------|
| 15.1 | Handle studios with no enabled faults | 0.5h |
| 15.2 | Handle source ChaosHub deletion (orphaned studio) | 1h |
| 15.3 | Add search/filter within Fault Studios tab | 1h |
| 15.4 | Add tooltips explaining pre-configured settings | 0.5h |
| 15.5 | Keyboard navigation support | 0.5h |
| 15.6 | Accessibility audit and fixes | 1h |
| 15.7 | End-to-end testing | 2h |

**Phase 15 Total: ~6.5 hours**

---

## Total Estimated Effort

| Phase | Description | Hours |
|-------|-------------|-------|
| Phase 9 | Backend API Layer | 8h |
| Phase 10 | Frontend API Layer | 2h |
| Phase 11 | Fault Studios Tab Component | 8h |
| Phase 12 | Integrate Tab into Drawer | 3h |
| Phase 13 | Bulk Fault Addition Logic | 6h |
| Phase 14 | Tune Faults After Addition | 3.5h |
| Phase 15 | Polish & Edge Cases | 6.5h |
| **Total** | | **37 hours** (~5 days) |

---

## Data Flow

### Sequence Diagram: Adding Faults from Studio

```
┌──────┐         ┌─────────┐         ┌─────────┐         ┌─────────┐         ┌─────────┐
│ User │         │   UI    │         │ GraphQL │         │ Service │         │  Store  │
└──┬───┘         └────┬────┘         └────┬────┘         └────┬────┘         └────┬────┘
   │                  │                   │                   │                   │
   │ Click [+ Add]    │                   │                   │                   │
   │─────────────────>│                   │                   │                   │
   │                  │                   │                   │                   │
   │                  │ Open Drawer       │                   │                   │
   │                  │ Show Tabs         │                   │                   │
   │<─────────────────│                   │                   │                   │
   │                  │                   │                   │                   │
   │ Click "Studios"  │                   │                   │                   │
   │─────────────────>│                   │                   │                   │
   │                  │                   │                   │                   │
   │                  │ listFaultStudiosForExperiment()       │                   │
   │                  │──────────────────>│                   │                   │
   │                  │                   │                   │                   │
   │                  │                   │ GetStudios()      │                   │
   │                  │                   │──────────────────>│                   │
   │                  │                   │                   │                   │
   │                  │                   │                   │ Query MongoDB     │
   │                  │                   │                   │──────────────────>│
   │                  │                   │                   │                   │
   │                  │                   │                   │<──────────────────│
   │                  │                   │                   │ Studios[]         │
   │                  │                   │                   │                   │
   │                  │                   │ ForEach Studio:   │                   │
   │                  │                   │ GetChaosFault()   │                   │
   │                  │                   │<──────────────────│                   │
   │                  │                   │                   │                   │
   │                  │<──────────────────│ StudiosWithFaultCRs                   │
   │                  │                   │                   │                   │
   │<─────────────────│ Display Studios   │                   │                   │
   │                  │ with Faults       │                   │                   │
   │                  │                   │                   │                   │
   │ Select Faults    │                   │                   │                   │
   │ Click [Add]      │                   │                   │                   │
   │─────────────────>│                   │                   │                   │
   │                  │                   │                   │                   │
   │                  │ addMultipleFaultsFromStudio()         │                   │
   │                  │──────────────────────────────────────────────────────────>│
   │                  │                   │                   │                   │
   │                  │<──────────────────────────────────────────────────────────│
   │                  │                   │                   │ Updated Manifest  │
   │                  │                   │                   │                   │
   │<─────────────────│ Close Drawer      │                   │                   │
   │                  │ Update DAG        │                   │                   │
   │                  │ Open TuneDrawer   │                   │                   │
   │                  │                   │                   │                   │
```

---

## API Changes

### New Query

```graphql
type Query {
  # ... existing queries ...
  
  listFaultStudiosForExperiment(
    projectID: ID!
    request: ListFaultStudiosForExperimentRequest
  ): ListFaultStudiosForExperimentResponse! @authorized
}
```

### New Input Type

```graphql
input ListFaultStudiosForExperimentRequest {
  activeOnly: Boolean = true
  enabledFaultsOnly: Boolean = true
}
```

### New Response Types

```graphql
type ListFaultStudiosForExperimentResponse {
  faultStudios: [FaultStudioWithDetails!]!
}

type FaultStudioWithDetails {
  id: ID!
  name: String!
  description: String
  sourceHubId: ID!
  sourceHubName: String!
  isActive: Boolean!
  faults: [FaultWithCRs!]!
}

type FaultWithCRs {
  faultCategory: String!
  faultName: String!
  displayName: String
  description: String
  enabled: Boolean!
  weight: Int
  injectionConfig: InjectionConfig
  faultCR: String!
  engineCR: String!
  csv: String
}
```

---

## Testing Strategy

### Unit Tests

| Component | Test Cases |
|-----------|------------|
| `ListFaultStudiosForExperiment` service | Studios returned, fault CRs included, injection config applied |
| `applyInjectionConfig` | Duration set, target selector applied, scheduled injection |
| `addMultipleFaultsFromStudio` | Single fault, multiple faults, insertion order correct |
| `FaultStudiosTab` | Loading state, empty state, selection works |

### Integration Tests

| Scenario | Steps |
|----------|-------|
| Happy Path | Create studio → Add faults → Create experiment → Add from studio → Verify manifest |
| Partial Success | Studio with some invalid faults → Add to experiment → Verify valid ones added |
| Disabled Studio | Try to list disabled studio → Should not appear |

### E2E Tests

| Test | Description |
|------|-------------|
| Full workflow | Create experiment, add faults from studio, run experiment |
| Cross-studio | Add faults from multiple studios to same experiment |
| Override settings | Add fault from studio, modify tunables, save experiment |

---

## Rollout Plan

### Milestone 1: Backend Ready (Phases 9-10)
- GraphQL schema updated
- Backend service implemented
- API hooks created
- **Deliverable:** API can be tested via GraphQL Playground

### Milestone 2: UI Component (Phases 11-12)
- Fault Studios tab visible in drawer
- Studios displayed with expandable fault lists
- Multi-select works
- **Deliverable:** Can see studios in UI, no functionality yet

### Milestone 3: Functional Integration (Phases 13-14)
- Faults can be added from studio to experiment
- Tune drawer opens after addition
- Pre-configured settings applied
- **Deliverable:** Feature is functional end-to-end

### Milestone 4: Production Ready (Phase 15)
- Edge cases handled
- Accessibility fixes
- E2E tests passing
- **Deliverable:** Ready for release

---

## Future Enhancements

1. **Templates Tab** - Pre-built experiment templates using studios
2. **Studio Versioning** - Track changes to studios over time
3. **Auto-generate Experiments** - Create entire experiment from a studio
4. **Studio Sharing** - Share studios across projects
5. **Studio Import/Export** - YAML-based studio definitions
6. **Recommended Faults** - AI-based fault recommendations

---

## Appendix

### A. File Locations

| Type | Path |
|------|------|
| GraphQL Schema | `chaoscenter/graphql/definitions/shared/fault_studio.graphqls` |
| Backend Service | `chaoscenter/graphql/server/pkg/fault_studio/service.go` |
| Resolver | `chaoscenter/graphql/server/graph/fault_studio.resolvers.go` |
| Frontend API | `chaoscenter/web/src/api/core/faultStudio/` |
| Views | `chaoscenter/web/src/views/` |
| Controllers | `chaoscenter/web/src/controllers/` |
| Experiment Service | `chaoscenter/web/src/services/experiment/KubernetesYamlService.ts` |

### B. Related Documentation

- [Fault Studio Design Document](../FAULT_STUDIO_DESIGN.md)
- [Litmus Experiment Architecture](../../docs/architecture.md)
- [ChaosHub Integration Guide](../../chaoscenter/graphql/server/pkg/chaos_hub/README.md)

---

*Document Version: 1.0*
*Created: February 2026*
*Author: AI Assistant (GitHub Copilot)*
