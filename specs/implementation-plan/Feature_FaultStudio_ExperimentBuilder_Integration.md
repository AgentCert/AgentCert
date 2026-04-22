# Fault Studio + Experiment Builder Integration - Implementation Plan

## Overview

This document provides a detailed step-by-step implementation plan for integrating Fault Studio with the Experiment Builder (Chaos Studio). Each phase includes specific tasks, files to create/modify, and acceptance criteria.

---

## Phase 9: Backend API Layer

### 9.1 Update GraphQL Schema

**File:** `chaoscenter/graphql/definitions/shared/fault_studio.graphqls`

**Tasks:**
1. Add `ListFaultStudiosForExperimentRequest` input type
2. Add `ListFaultStudiosForExperimentResponse` type
3. Add `FaultStudioWithDetails` type
4. Add `FaultWithCRs` type
5. Add `listFaultStudiosForExperiment` query

**Code to Add:**
```graphql
# Input for listing fault studios for experiment integration
input ListFaultStudiosForExperimentRequest {
  activeOnly: Boolean = true
  enabledFaultsOnly: Boolean = true
}

# Response type for experiment integration
type ListFaultStudiosForExperimentResponse {
  faultStudios: [FaultStudioWithDetails!]!
}

# Studio with full fault CR details
type FaultStudioWithDetails {
  id: ID!
  name: String!
  description: String
  sourceHubId: ID!
  sourceHubName: String!
  isActive: Boolean!
  faults: [FaultWithCRs!]!
}

# Fault with complete CR data for experiment manifest
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

extend type Query {
  listFaultStudiosForExperiment(
    projectID: ID!
    request: ListFaultStudiosForExperimentRequest
  ): ListFaultStudiosForExperimentResponse! @authorized
}
```

**Acceptance Criteria:**
- [ ] Schema validates without errors
- [ ] Types are correctly defined
- [ ] Query is marked as @authorized

---

### 9.2 Implement Service Function

**File:** `chaoscenter/graphql/server/pkg/fault_studio/service.go`

**Tasks:**
1. Add `ListFaultStudiosForExperiment` function
2. Add `applyInjectionConfig` helper function
3. Add `applyInjectionConfigToEngine` helper function

**Function Signature:**
```go
func (s *Service) ListFaultStudiosForExperiment(
    ctx context.Context,
    projectID string,
    request *model.ListFaultStudiosForExperimentRequest,
) (*model.ListFaultStudiosForExperimentResponse, error)
```

**Logic:**
1. Query all active studios for the project
2. For each studio, iterate through selected faults
3. Filter by enabled status if `enabledFaultsOnly` is true
4. Fetch fault CRs from source ChaosHub using `GetChaosFault`
5. Apply injection config to engine CR
6. Return studios with full fault details

**Acceptance Criteria:**
- [ ] Returns studios with fault CRs
- [ ] Filters disabled faults when requested
- [ ] Handles missing source ChaosHub gracefully
- [ ] Applies injection config correctly

---

### 9.3 Add ChaosHub Service Dependency

**File:** `chaoscenter/graphql/server/pkg/fault_studio/service.go`

**Tasks:**
1. Add `chaosHubService` field to Service struct
2. Update constructor to accept ChaosHub service
3. Create interface for ChaosHub operations needed

**Code:**
```go
type ChaosHubProvider interface {
    GetChaosFault(ctx context.Context, hubID, faultCategory, faultName, projectID string) (*model.FaultDetails, error)
}

type Service struct {
    faultStudioOperator *fault_studio.Operator
    chaosHubProvider    ChaosHubProvider
}
```

**Acceptance Criteria:**
- [ ] Service has access to ChaosHub operations
- [ ] Dependency injection is clean

---

### 9.4 Implement Resolver

**File:** `chaoscenter/graphql/server/graph/fault_studio.resolvers.go`

**Tasks:**
1. Add `ListFaultStudiosForExperiment` resolver function

**Code:**
```go
func (r *queryResolver) ListFaultStudiosForExperiment(
    ctx context.Context,
    projectID string,
    request *model.ListFaultStudiosForExperimentRequest,
) (*model.ListFaultStudiosForExperimentResponse, error) {
    // RBAC check
    err := authorization.ValidateRole(
        ctx,
        projectID,
        authorization.MutationRbacRules[authorization.ListFaultStudios],
        model.InvitationAccepted.String(),
    )
    if err != nil {
        return nil, err
    }
    
    return r.faultStudioService.ListFaultStudiosForExperiment(ctx, projectID, request)
}
```

**Acceptance Criteria:**
- [ ] Resolver validates RBAC
- [ ] Resolver calls service correctly

---

### 9.5 Add RBAC Rule

**File:** `chaoscenter/graphql/server/pkg/authorization/roles.go`

**Tasks:**
1. Add `ListFaultStudiosForExperiment` to MutationRbacRules

**Code:**
```go
"ListFaultStudiosForExperiment": {MemberRoleOwnerString, MemberRoleExecutorString, MemberRoleViewerString},
```

**Acceptance Criteria:**
- [ ] All roles can access this query

---

### 9.6 Regenerate gqlgen

**Command:**
```bash
cd chaoscenter/graphql/server
go generate ./...
```

**Acceptance Criteria:**
- [ ] `generated.go` regenerated without errors
- [ ] `models_gen.go` includes new types

---

### 9.7 Wire Up in Server

**File:** `chaoscenter/graphql/server/server.go`

**Tasks:**
1. Update fault studio service initialization to include ChaosHub dependency

**Acceptance Criteria:**
- [ ] Service is correctly instantiated
- [ ] Dependencies are resolved

---

### 9.8 Unit Tests

**File:** `chaoscenter/graphql/server/pkg/fault_studio/service_test.go`

**Test Cases:**
1. `TestListFaultStudiosForExperiment_Success`
2. `TestListFaultStudiosForExperiment_EnabledFaultsOnly`
3. `TestListFaultStudiosForExperiment_NoStudios`
4. `TestApplyInjectionConfig_Duration`
5. `TestApplyInjectionConfig_TargetSelector`

**Acceptance Criteria:**
- [ ] All tests pass
- [ ] Edge cases covered

---

## Phase 10: Frontend API Layer

### 10.1 Create API Hook

**File:** `chaoscenter/web/src/api/core/faultStudio/listFaultStudiosForExperiment.ts`

**Tasks:**
1. Define GraphQL query
2. Create request/response interfaces
3. Export lazy query hook

**Acceptance Criteria:**
- [ ] TypeScript compiles without errors
- [ ] Query matches backend schema

---

### 10.2 Update Entity Types

**File:** `chaoscenter/web/src/api/entities/faultStudio.ts`

**Tasks:**
1. Add `FaultStudioWithDetails` interface
2. Add `FaultWithCRs` interface
3. Export new types

**Acceptance Criteria:**
- [ ] Types match GraphQL schema
- [ ] Exported correctly

---

### 10.3 Export from Index

**File:** `chaoscenter/web/src/api/core/faultStudio/index.ts`

**Tasks:**
1. Export `listFaultStudiosForExperiment.ts` exports

**Acceptance Criteria:**
- [ ] All exports available from index

---

### 10.4 Add String Constants

**File:** `chaoscenter/web/src/strings/strings.en.yaml`

**Tasks:**
1. Add localization strings for new UI

**Strings to Add:**
```yaml
faultStudiosTab: Fault Studios
selectFromStudios: Select from Studios
addSelectedFaults: Add Selected Faults
faultsFromStudio: Faults from Studio
noFaultStudios: No Fault Studios available
createStudioFirst: Create a Fault Studio to use pre-configured faults
studioOrigin: Added from studio
preConfiguredSettings: Pre-configured settings will be applied
```

**Acceptance Criteria:**
- [ ] All strings added
- [ ] Types regenerated

---

## Phase 11: Fault Studios Tab Component

### 11.1 Create View Component

**File:** `chaoscenter/web/src/views/FaultStudiosTab/FaultStudiosTab.tsx`

**Tasks:**
1. Create main component structure
2. Display list of studios
3. Handle studio expansion
4. Manage fault selection state

**Props:**
```typescript
interface FaultStudiosTabProps {
  studios: FaultStudioWithDetails[];
  loading: boolean;
  selectedFaults: Map<string, FaultWithCRs[]>;
  onFaultSelectionChange: (studioId: string, faults: FaultWithCRs[]) => void;
  onAddFaults: () => void;
}
```

**Acceptance Criteria:**
- [ ] Studios displayed in list
- [ ] Expand/collapse works
- [ ] Selection state managed correctly

---

### 11.2 Create Styles

**File:** `chaoscenter/web/src/views/FaultStudiosTab/FaultStudiosTab.module.scss`

**Tasks:**
1. Style studio cards
2. Style fault list items
3. Style selection checkboxes
4. Style add button

**Acceptance Criteria:**
- [ ] Consistent with existing UI
- [ ] Responsive design

---

### 11.3 Create FaultStudioCard Sub-component

**File:** `chaoscenter/web/src/views/FaultStudiosTab/FaultStudioCard.tsx`

**Tasks:**
1. Display studio header (name, description)
2. Show fault count badge
3. Expandable fault list
4. Source hub indicator

**Acceptance Criteria:**
- [ ] Header displays correctly
- [ ] Expansion animation smooth
- [ ] Badge shows correct count

---

### 11.4 Create FaultCheckboxList Sub-component

**File:** `chaoscenter/web/src/views/FaultStudiosTab/FaultCheckboxList.tsx`

**Tasks:**
1. Display fault items with checkboxes
2. Show pre-configured settings preview
3. Handle disabled faults differently
4. Multi-select functionality

**Acceptance Criteria:**
- [ ] Checkboxes work correctly
- [ ] Disabled faults grayed out
- [ ] Settings preview visible

---

### 11.5 Create Controller

**File:** `chaoscenter/web/src/controllers/FaultStudiosTab/FaultStudiosTab.tsx`

**Tasks:**
1. Fetch studios using lazy query
2. Manage loading state
3. Handle errors
4. Pass data to view

**Acceptance Criteria:**
- [ ] Data fetched on mount
- [ ] Loading state shown
- [ ] Errors handled gracefully

---

### 11.6 Loading Skeleton

**Tasks:**
1. Add skeleton cards for loading state
2. Animate while loading

**Acceptance Criteria:**
- [ ] Skeleton matches layout
- [ ] Smooth transition to loaded state

---

### 11.7 Empty State

**Tasks:**
1. Display message when no studios exist
2. Link to create studio page

**Acceptance Criteria:**
- [ ] Helpful message displayed
- [ ] Clear call-to-action

---

## Phase 12: Integrate Tab into Drawer

### 12.1 Modify ExperimentCreationSelectFaultView

**File:** `chaoscenter/web/src/views/ExperimentCreationSelectFault/ExperimentCreationSelectFault.tsx`

**Tasks:**
1. Add `Tabs` component wrapper
2. Create "ChaosHub" tab with existing content
3. Create "Fault Studios" tab with new component
4. Manage active tab state

**Acceptance Criteria:**
- [ ] Tabs visible in drawer
- [ ] Tab switching works
- [ ] Content changes correctly

---

### 12.2 Update Controller

**File:** `chaoscenter/web/src/controllers/ExperimentCreationSelectFault/ExperimentCreationSelectFault.tsx`

**Tasks:**
1. Add tab state management
2. Conditionally fetch studio data
3. Handle selection from both tabs

**Acceptance Criteria:**
- [ ] Tab state persisted during session
- [ ] Only fetches data when tab is active

---

### 12.3 Wire Up Selection

**Tasks:**
1. Map studio fault selection to FaultData[]
2. Pass to parent component

**Acceptance Criteria:**
- [ ] Selection data format correct
- [ ] Parent receives data correctly

---

## Phase 13: Bulk Fault Addition Logic

### 13.1 Add Service Function

**File:** `chaoscenter/web/src/services/experiment/KubernetesYamlService.ts`

**Tasks:**
1. Add `addMultipleFaultsFromStudio` function
2. Handle insertion order
3. Update install-chaos-faults template
4. Add engine templates for each fault

**Acceptance Criteria:**
- [ ] Multiple faults added correctly
- [ ] Order preserved in DAG
- [ ] Manifest valid after addition

---

### 13.2 Add Injection Config Helper

**File:** `chaoscenter/web/src/services/experiment/KubernetesYamlService.ts`

**Tasks:**
1. Add `applyStudioInjectionConfig` function
2. Apply duration to ENV
3. Apply target selector to appinfo
4. Handle scheduled injection

**Acceptance Criteria:**
- [ ] Duration applied correctly
- [ ] Target selector set
- [ ] ENV vars updated

---

### 13.3 Modify ExperimentVisualBuilder

**File:** `chaoscenter/web/src/views/ExperimentVisualBuilder/ExperimentVisualBuilder.tsx`

**Tasks:**
1. Add handler for bulk fault selection
2. Update `handleFaultSelection` to handle array
3. Update DAG after bulk add

**Acceptance Criteria:**
- [ ] DAG updates correctly
- [ ] All faults visible in graph

---

### 13.4 Progress Indicator

**Tasks:**
1. Show progress during multi-fault addition
2. Display count (1/3, 2/3, etc.)

**Acceptance Criteria:**
- [ ] Progress visible to user
- [ ] Smooth experience

---

### 13.5 Error Handling

**Tasks:**
1. Handle partial success scenario
2. Show which faults failed
3. Allow retry for failed faults

**Acceptance Criteria:**
- [ ] Partial success works
- [ ] User informed of failures
- [ ] Retry mechanism available

---

## Phase 14: Tune Faults After Addition

### 14.1 Open Tune Drawer

**Tasks:**
1. Open drawer after first fault is added
2. Pre-populate with studio settings

**Acceptance Criteria:**
- [ ] Drawer opens automatically
- [ ] Settings pre-filled

---

### 14.2 Navigation Between Faults

**Tasks:**
1. Add "Next" button in tune drawer
2. Add "Previous" button
3. Track current fault index

**Acceptance Criteria:**
- [ ] Navigation works correctly
- [ ] Current position indicated

---

### 14.3 Studio Origin Badge

**Tasks:**
1. Show badge indicating studio origin
2. Link to source studio

**Acceptance Criteria:**
- [ ] Badge visible
- [ ] Link works

---

### 14.4 Pre-populate Tunables

**Tasks:**
1. Read injection config from fault
2. Map to form fields
3. Show as pre-filled values

**Acceptance Criteria:**
- [ ] Values appear in form
- [ ] Correctly mapped

---

### 14.5 Allow Override

**Tasks:**
1. Allow editing pre-filled values
2. Mark as "modified from studio"
3. Persist modifications

**Acceptance Criteria:**
- [ ] Editing works
- [ ] Changes saved

---

## Phase 15: Polish & Edge Cases

### 15.1 Empty Studio Handling

**Tasks:**
1. Show message if studio has no enabled faults
2. Disable selection for empty studios

**Acceptance Criteria:**
- [ ] Empty studios handled gracefully

---

### 15.2 Orphaned Studio Handling

**Tasks:**
1. Detect if source ChaosHub is deleted
2. Show warning badge
3. Still allow selecting if CRs are cached

**Acceptance Criteria:**
- [ ] Warning shown
- [ ] Graceful degradation

---

### 15.3 Search/Filter

**Tasks:**
1. Add search input in tab
2. Filter studios by name
3. Filter faults within studios

**Acceptance Criteria:**
- [ ] Search works
- [ ] Filters correctly

---

### 15.4 Tooltips

**Tasks:**
1. Add tooltips for settings preview
2. Explain what each setting does

**Acceptance Criteria:**
- [ ] Tooltips helpful
- [ ] Not intrusive

---

### 15.5 Keyboard Navigation

**Tasks:**
1. Tab navigation through studios
2. Enter to expand
3. Space to select

**Acceptance Criteria:**
- [ ] Full keyboard support

---

### 15.6 Accessibility

**Tasks:**
1. ARIA labels
2. Screen reader support
3. Color contrast check

**Acceptance Criteria:**
- [ ] WCAG compliant

---

### 15.7 End-to-End Tests

**File:** `chaoscenter/web/test/e2e/fault-studio-experiment.spec.ts`

**Test Cases:**
1. Add single fault from studio to experiment
2. Add multiple faults from studio
3. Add faults from multiple studios
4. Override studio settings
5. Run experiment with studio faults

**Acceptance Criteria:**
- [ ] All E2E tests pass
- [ ] Coverage > 80%

---

## Summary Checklist

### Phase 9 - Backend
- [ ] 9.1 GraphQL Schema updated
- [ ] 9.2 Service function implemented
- [ ] 9.3 ChaosHub dependency added
- [ ] 9.4 Resolver implemented
- [ ] 9.5 RBAC rule added
- [ ] 9.6 gqlgen regenerated
- [ ] 9.7 Server wired up
- [ ] 9.8 Unit tests pass

### Phase 10 - Frontend API
- [ ] 10.1 API hook created
- [ ] 10.2 Entity types added
- [ ] 10.3 Exports updated
- [ ] 10.4 Strings added

### Phase 11 - Tab Component
- [ ] 11.1 View component created
- [ ] 11.2 Styles created
- [ ] 11.3 FaultStudioCard created
- [ ] 11.4 FaultCheckboxList created
- [ ] 11.5 Controller created
- [ ] 11.6 Loading skeleton added
- [ ] 11.7 Empty state added

### Phase 12 - Integration
- [ ] 12.1 Tabs added to drawer
- [ ] 12.2 Controller updated
- [ ] 12.3 Selection wired up

### Phase 13 - Bulk Addition
- [ ] 13.1 Service function added
- [ ] 13.2 Injection config helper added
- [ ] 13.3 Visual builder updated
- [ ] 13.4 Progress indicator added
- [ ] 13.5 Error handling implemented

### Phase 14 - Tune Drawer
- [ ] 14.1 Auto-open implemented
- [ ] 14.2 Navigation added
- [ ] 14.3 Origin badge added
- [ ] 14.4 Pre-populate works
- [ ] 14.5 Override works

### Phase 15 - Polish
- [ ] 15.1 Empty studio handled
- [ ] 15.2 Orphaned studio handled
- [ ] 15.3 Search works
- [ ] 15.4 Tooltips added
- [ ] 15.5 Keyboard nav works
- [ ] 15.6 Accessibility pass
- [ ] 15.7 E2E tests pass

---

## Git Branch Strategy

```
main
  └── feature/fault-studio-experiment-integration
        ├── phase-9-backend-api
        ├── phase-10-frontend-api
        ├── phase-11-tab-component
        ├── phase-12-integration
        ├── phase-13-bulk-add
        ├── phase-14-tune-drawer
        └── phase-15-polish
```

---

## Definition of Done

A phase is considered complete when:
1. All code changes are committed
2. Unit tests pass
3. No linting errors
4. Code review approved
5. Manual testing verified
6. Documentation updated

---

*Document Version: 1.0*
*Created: February 2026*
