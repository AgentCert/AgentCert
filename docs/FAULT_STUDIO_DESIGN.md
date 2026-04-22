# Fault Studio Feature - Design Document

## Table of Contents
1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Data Model](#data-model)
4. [Implementation Phases](#implementation-phases)
5. [Artifacts Changed](#artifacts-changed)
6. [API Reference](#api-reference)
7. [UI Components](#ui-components)
8. [Future Enhancements](#future-enhancements)

---

## Overview

### What is Fault Studio?

Fault Studio is a feature that extends the existing ChaosHub architecture to allow users to create customized collections of chaos faults. It provides a way to:

- **Create Fault Categories** (called "Fault Studios") - Logical groupings like Network, CPU, Memory, etc.
- **Select Faults from ChaosHub** - Pick specific faults from the default Litmus ChaosHub or custom hubs
- **Enable/Disable Individual Faults** - Toggle faults on/off without removing them
- **Manage Fault Configuration** - Configure injection settings for each fault

### Key Concepts

```
Fault Studio (container)
  └── Source ChaosHub (reference to where faults come from)
  └── Selected Faults (array of faults with configurations)
       ├── Fault 1 (Network category)
       │    ├── enabled: true/false
       │    ├── injectionConfig
       │    └── customParameters
       ├── Fault 2 (CPU category)
       └── Fault 3 (Memory category)
```

### Design Decision

The implementation follows **Option A: Extend ChaosHub** approach, which:
- Reuses existing ChaosHub infrastructure for fetching fault definitions
- Adds a new `faultStudios` MongoDB collection for storing user selections
- Maintains separation between fault definitions (ChaosHub) and fault configurations (Fault Studio)

---

## Architecture

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Frontend (React)                         │
├─────────────────────────────────────────────────────────────────┤
│  Views:                    │  API Hooks:                        │
│  - FaultStudios (list)     │  - listFaultStudios                │
│  - FaultStudio (detail)    │  - getFaultStudio                  │
│  - CreateFaultStudioModal  │  - createFaultStudio               │
│  - EditFaultStudioModal    │  - updateFaultStudio               │
│  - AddFaultsModal          │  - deleteFaultStudio               │
│  - DeleteFaultStudioDialog │  - toggleFaultInStudio             │
│                            │  - removeFaultFromStudio           │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     GraphQL Server (Go)                         │
├─────────────────────────────────────────────────────────────────┤
│  Resolvers:                │  Service Layer:                    │
│  - fault_studio.resolvers  │  - FaultStudioService              │
│                            │    ├── CreateFaultStudio           │
│  Schema:                   │    ├── GetFaultStudio              │
│  - fault_studio.graphqls   │    ├── ListFaultStudios            │
│                            │    ├── UpdateFaultStudio           │
│                            │    ├── DeleteFaultStudio           │
│                            │    ├── ToggleFaultInStudio         │
│                            │    ├── RemoveFaultFromStudio       │
│                            │    └── SetFaultStudioActive        │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                        MongoDB                                   │
├─────────────────────────────────────────────────────────────────┤
│  Collections:                                                    │
│  - faultStudios (new)                                           │
│  - chaosHubs (existing, referenced)                             │
└─────────────────────────────────────────────────────────────────┘
```

### Integration with ChaosHub

Fault Studio integrates with ChaosHub by:
1. Referencing a source ChaosHub by ID
2. Using `listChaosFaults` query to fetch available faults
3. Storing selected faults with their configurations locally

**Special Handling for Default Hub:**
The default "Litmus ChaosHub" is NOT stored in MongoDB - it's generated dynamically with hardcoded ID `6f39cea9-6264-4951-83a8-29976b614289`. The service layer handles this specially.

---

## Data Model

### MongoDB Collection: `faultStudios`

```javascript
{
  "_id": ObjectId,
  "studio_id": "uuid-string",
  "name": "Network Faults",
  "description": "Collection of network-related chaos faults",
  "tags": ["network", "latency", "packet-loss"],
  "project_id": "project-uuid",
  "source_hub_id": "chaoshub-uuid",
  "source_hub_name": "Litmus ChaosHub",
  "selected_faults": [
    {
      "fault_category": "pod-network-loss",
      "fault_name": "pod-network-loss",
      "display_name": "Pod Network Loss",
      "description": "Injects network packet loss...",
      "enabled": true,
      "injection_config": {
        "injection_type": "continuous",
        "schedule": "",
        "duration": "30s",
        "target_selector": "",
        "interval": ""
      },
      "custom_parameters": {},
      "weight": 10
    }
  ],
  "is_active": true,
  "is_removed": false,
  "created_at": "timestamp",
  "updated_at": "timestamp",
  "created_by": {
    "user_id": "user-uuid",
    "username": "admin"
  },
  "updated_by": {
    "user_id": "user-uuid",
    "username": "admin"
  }
}
```

### GraphQL Types

```graphql
type FaultStudio {
  id: ID!
  name: String!
  description: String
  tags: [String!]
  projectId: ID!
  sourceHubId: ID!
  sourceHubName: String!
  selectedFaults: [FaultSelection!]
  isActive: Boolean!
  totalFaults: Int!
  enabledFaults: Int!
  isRemoved: Boolean!
  createdAt: String!
  updatedAt: String!
  createdBy: UserDetails
  updatedBy: UserDetails
}

type FaultSelection {
  faultCategory: String!
  faultName: String!
  displayName: String
  description: String
  enabled: Boolean!
  injectionConfig: InjectionConfig
  customParameters: Map
  weight: Int
}

type InjectionConfig {
  injectionType: String
  schedule: String
  duration: String
  targetSelector: String
  interval: String
}
```

---

## Implementation Phases

### Phase 1: Database Layer ✅
**Objective:** Create MongoDB operations for Fault Studio

**Files Created:**
- `chaoscenter/graphql/server/pkg/database/mongodb/fault_studio/schema.go`
- `chaoscenter/graphql/server/pkg/database/mongodb/fault_studio/operations.go`

**Key Operations:**
- `CreateFaultStudio`
- `GetFaultStudioByID`
- `ListFaultStudios`
- `UpdateFaultStudio`
- `DeleteFaultStudio` (soft delete)
- `IsFaultStudioNameUnique`

---

### Phase 2: GraphQL Schema ✅
**Objective:** Define GraphQL types, queries, and mutations

**Files Created:**
- `chaoscenter/graphql/definitions/shared/fault_studio.graphqls`

**Schema Includes:**
- Types: `FaultStudio`, `FaultStudioSummary`, `FaultSelection`, `InjectionConfig`
- Input Types: `CreateFaultStudioRequest`, `UpdateFaultStudioRequest`, `FaultSelectionInput`
- Queries: `listFaultStudios`, `getFaultStudio`, `listAvailableFaultsForStudio`, `getFaultStudioStats`, `isFaultStudioNameAvailable`
- Mutations: `createFaultStudio`, `updateFaultStudio`, `deleteFaultStudio`, `toggleFaultInStudio`, `setFaultStudioActive`, `addFaultToStudio`, `removeFaultFromStudio`, `updateFaultInStudio`

---

### Phase 3: Service Layer ✅
**Objective:** Implement business logic

**Files Created:**
- `chaoscenter/graphql/server/pkg/fault_studio/service.go`

**Key Functions:**
- `CreateFaultStudio` - Creates new studio with source hub validation
- `GetFaultStudio` - Retrieves studio by ID
- `ListFaultStudios` - Lists studios with filtering/pagination
- `UpdateFaultStudio` - Updates studio metadata and faults
- `DeleteFaultStudio` - Soft deletes a studio
- `ToggleFaultInStudio` - Enables/disables individual faults
- `RemoveFaultFromStudio` - Removes a fault from studio
- `SetFaultStudioActive` - Activates/deactivates entire studio

**Special Handling:**
- Default ChaosHub detection (ID: `6f39cea9-6264-4951-83a8-29976b614289`)
- Name uniqueness validation
- Computed fields (totalFaults, enabledFaults)

---

### Phase 4: Resolvers ✅
**Objective:** Connect GraphQL to service layer

**Files Created:**
- `chaoscenter/graphql/server/graph/fault_studio.resolvers.go`

**Resolvers Implemented:**
- All query resolvers with RBAC validation
- All mutation resolvers with appropriate permission checks

---

### Phase 5: Server Integration ✅
**Objective:** Wire up all components

**Files Modified:**
- `chaoscenter/graphql/server/server.go` - Added service initialization
- `chaoscenter/graphql/server/pkg/authorization/roles.go` - Added RBAC rules

**RBAC Rules Added:**
```go
CreateFaultStudio: {MemberRoleOwnerString}
UpdateFaultStudio: {MemberRoleOwnerString}
DeleteFaultStudio: {MemberRoleOwnerString}
ListFaultStudios:  {MemberRoleOwnerString, MemberRoleExecutorString, MemberRoleViewerString}
GetFaultStudio:    {MemberRoleOwnerString, MemberRoleExecutorString, MemberRoleViewerString}
```

---

### Phase 6: Frontend API Layer ✅
**Objective:** Create React hooks and routing

**Files Created:**
- `chaoscenter/web/src/api/core/faultStudio/listFaultStudios.ts`
- `chaoscenter/web/src/api/core/faultStudio/getFaultStudio.ts`
- `chaoscenter/web/src/api/core/faultStudio/createFaultStudio.ts`
- `chaoscenter/web/src/api/core/faultStudio/updateFaultStudio.ts`
- `chaoscenter/web/src/api/core/faultStudio/deleteFaultStudio.ts`
- `chaoscenter/web/src/api/core/faultStudio/toggleFaultInStudio.ts`
- `chaoscenter/web/src/api/core/faultStudio/setFaultStudioActive.ts`
- `chaoscenter/web/src/api/core/faultStudio/getFaultStudioStats.ts`
- `chaoscenter/web/src/api/core/faultStudio/isFaultStudioNameAvailable.ts`
- `chaoscenter/web/src/api/core/faultStudio/removeFaultFromStudio.ts`
- `chaoscenter/web/src/api/core/faultStudio/index.ts`

**Files Modified:**
- `chaoscenter/web/src/api/entities/faultStudio.ts` - TypeScript interfaces
- `chaoscenter/web/src/routes/RouteDefinitions.ts` - Route paths
- `chaoscenter/web/src/routes/RouteDestinations.tsx` - Route components
- `chaoscenter/web/src/components/SideNav/SideNav.tsx` - Navigation link
- `chaoscenter/web/src/strings/strings.en.yaml` - Localization strings

---

### Phase 7A: Create Fault Studio Modal ✅
**Objective:** UI for creating new studios

**Files Created:**
- `chaoscenter/web/src/views/CreateFaultStudioModal/CreateFaultStudioModal.tsx`
- `chaoscenter/web/src/views/CreateFaultStudioModal/CreateFaultStudioModal.module.scss`
- `chaoscenter/web/src/views/CreateFaultStudioModal/index.ts`
- `chaoscenter/web/src/controllers/CreateFaultStudioModal/CreateFaultStudioModal.tsx`
- `chaoscenter/web/src/controllers/CreateFaultStudioModal/index.ts`

**Features:**
- Form with name, description, tags, source ChaosHub dropdown
- Formik validation
- ChaosHub selection with live loading

---

### Phase 7B: Edit Fault Studio Modal ✅
**Objective:** UI for editing studio metadata

**Files Created:**
- `chaoscenter/web/src/views/EditFaultStudioModal/EditFaultStudioModal.tsx`
- `chaoscenter/web/src/views/EditFaultStudioModal/EditFaultStudioModal.module.scss`
- `chaoscenter/web/src/views/EditFaultStudioModal/index.ts`
- `chaoscenter/web/src/controllers/EditFaultStudioModal/EditFaultStudioModal.tsx`
- `chaoscenter/web/src/controllers/EditFaultStudioModal/index.ts`

**Features:**
- Edit name, description, tags
- Active/inactive toggle
- Source hub shown as read-only

---

### Phase 7C: Delete Fault Studio Dialog ✅
**Objective:** Confirmation dialog for deletion

**Files Created:**
- `chaoscenter/web/src/components/DeleteFaultStudioDialog/DeleteFaultStudioDialog.tsx`
- `chaoscenter/web/src/components/DeleteFaultStudioDialog/index.ts`

**Features:**
- Confirmation dialog using ConfirmationDialog component
- Shows studio name being deleted
- Success/error toast messages

---

### Phase 7 Additional: Views & Add Faults ✅
**Objective:** Main views and fault selection

**Files Created:**
- `chaoscenter/web/src/views/FaultStudios/FaultStudios.tsx` - List view
- `chaoscenter/web/src/views/FaultStudios/FaultStudios.module.scss`
- `chaoscenter/web/src/views/FaultStudio/FaultStudio.tsx` - Detail view
- `chaoscenter/web/src/views/FaultStudio/FaultStudio.module.scss`
- `chaoscenter/web/src/views/AddFaultsModal/AddFaultsModal.tsx` - Fault selection modal
- `chaoscenter/web/src/views/AddFaultsModal/AddFaultsModal.module.scss`
- `chaoscenter/web/src/controllers/FaultStudios/FaultStudios.tsx`
- `chaoscenter/web/src/controllers/FaultStudio/FaultStudio.tsx`

**Features:**
- Card-based studio list with search
- Studio detail view with tabs
- Add faults modal with category grouping and checkbox selection

---

### Phase 8: Enable/Disable Faults Toggle ✅
**Objective:** Toggle individual faults

**Files Modified:**
- `chaoscenter/web/src/views/FaultStudio/FaultStudio.tsx`
- `chaoscenter/web/src/views/FaultStudio/FaultStudio.module.scss`

**Features:**
- Toggle switch on each fault card
- Visual feedback during toggle operation
- Success/error toast messages
- Automatic UI refresh

---

## Artifacts Changed

### Backend Files

| File | Change Type | Description |
|------|-------------|-------------|
| `chaoscenter/graphql/definitions/shared/fault_studio.graphqls` | Created | GraphQL schema |
| `chaoscenter/graphql/server/pkg/database/mongodb/fault_studio/schema.go` | Created | MongoDB schema |
| `chaoscenter/graphql/server/pkg/database/mongodb/fault_studio/operations.go` | Created | Database operations |
| `chaoscenter/graphql/server/pkg/fault_studio/service.go` | Created | Business logic |
| `chaoscenter/graphql/server/graph/fault_studio.resolvers.go` | Created | GraphQL resolvers |
| `chaoscenter/graphql/server/graph/generated/generated.go` | Regenerated | gqlgen output |
| `chaoscenter/graphql/server/graph/model/models_gen.go` | Regenerated | gqlgen models |
| `chaoscenter/graphql/server/server.go` | Modified | Service wiring |
| `chaoscenter/graphql/server/pkg/authorization/roles.go` | Modified | RBAC rules |
| `chaoscenter/graphql/server/pkg/database/mongodb/chaos_hub/operations.go` | Modified | Added GetHubByIDOnly |

### Frontend Files

| File | Change Type | Description |
|------|-------------|-------------|
| `chaoscenter/web/src/api/core/faultStudio/*.ts` | Created | API hooks (10 files) |
| `chaoscenter/web/src/api/entities/faultStudio.ts` | Created | TypeScript interfaces |
| `chaoscenter/web/src/views/FaultStudios/*` | Created | List view |
| `chaoscenter/web/src/views/FaultStudio/*` | Created | Detail view |
| `chaoscenter/web/src/views/CreateFaultStudioModal/*` | Created | Create modal |
| `chaoscenter/web/src/views/EditFaultStudioModal/*` | Created | Edit modal |
| `chaoscenter/web/src/views/AddFaultsModal/*` | Created | Add faults modal |
| `chaoscenter/web/src/components/DeleteFaultStudioDialog/*` | Created | Delete dialog |
| `chaoscenter/web/src/controllers/FaultStudios/*` | Created | List controller |
| `chaoscenter/web/src/controllers/FaultStudio/*` | Created | Detail controller |
| `chaoscenter/web/src/controllers/CreateFaultStudioModal/*` | Created | Create controller |
| `chaoscenter/web/src/controllers/EditFaultStudioModal/*` | Created | Edit controller |
| `chaoscenter/web/src/routes/RouteDefinitions.ts` | Modified | Route paths |
| `chaoscenter/web/src/routes/RouteDestinations.tsx` | Modified | Route components |
| `chaoscenter/web/src/components/SideNav/SideNav.tsx` | Modified | Navigation |
| `chaoscenter/web/src/strings/strings.en.yaml` | Modified | Localization |
| `chaoscenter/web/src/strings/types.ts` | Modified | String types |

---

## API Reference

### Queries

#### listFaultStudios
```graphql
query listFaultStudios($projectID: ID!, $request: ListFaultStudioRequest) {
  listFaultStudios(projectID: $projectID, request: $request) {
    totalCount
    faultStudios {
      id
      name
      description
      isActive
      totalFaults
      enabledFaults
    }
  }
}
```

#### getFaultStudio
```graphql
query getFaultStudio($projectID: ID!, $studioID: ID!) {
  getFaultStudio(projectID: $projectID, studioID: $studioID) {
    id
    name
    description
    sourceHubName
    selectedFaults {
      faultName
      enabled
    }
  }
}
```

### Mutations

#### createFaultStudio
```graphql
mutation createFaultStudio($projectID: ID!, $request: CreateFaultStudioRequest!) {
  createFaultStudio(projectID: $projectID, request: $request) {
    id
    name
  }
}
```

#### toggleFaultInStudio
```graphql
mutation toggleFaultInStudio($projectID: ID!, $studioID: ID!, $faultName: String!, $enabled: Boolean!) {
  toggleFaultInStudio(projectID: $projectID, studioID: $studioID, faultName: $faultName, enabled: $enabled) {
    success
    message
  }
}
```

#### removeFaultFromStudio
```graphql
mutation removeFaultFromStudio($projectID: ID!, $studioID: ID!, $faultName: String!) {
  removeFaultFromStudio(projectID: $projectID, studioID: $studioID, faultName: $faultName) {
    id
    totalFaults
  }
}
```

---

## UI Components

### Fault Studios List (`/fault-studios`)
- Card-based layout
- Search functionality
- "+New Fault Category" button
- 3-dot menu with View/Delete options

### Fault Studio Detail (`/fault-studios/:studioID`)
- Info card with studio metadata
- Tabs: Faults, Settings
- Fault cards with toggle switches
- "+Add Faults" button
- Edit/Delete buttons in header

### Fault Card
- Displays fault name and category
- Toggle switch for enable/disable
- 3-dot menu with Remove option
- Injection config display

---

## Bug Fixes Applied

### 1. Source ChaosHub Not Found
**Issue:** Creating a Fault Studio with "Litmus ChaosHub" failed
**Root Cause:** Default hub is dynamically generated, not stored in DB
**Fix:** Added special handling for default hub ID in `CreateFaultStudio`

### 2. 422 Error on View
**Issue:** Viewing Fault Studio returned 422 error
**Root Cause:** Parameter name mismatch (`studioId` vs `studioID`)
**Fix:** Updated schema to use `studioID` consistently

### 3. Toggle Not Working
**Issue:** Fault toggle switch didn't respond to clicks
**Root Cause:** `killEvent` was blocking the Switch's onChange
**Fix:** Changed to handle click on container div with stopPropagation

---

## Future Enhancements

### Phase 9: Activate/Deactivate Studio
- Wire up the existing Activate/Deactivate button
- Visual indication when studio is inactive

### Phase 10: Integration with Chaos Experiments
- Add "Select from Fault Studio" option in Experiment Builder
- Auto-inject enabled faults from selected studio

### Phase 11: Polish & UX
- Loading skeletons
- Search/filter within studio
- Bulk operations (enable/disable all)
- Drag-and-drop reordering

---

## Document History

| Date | Version | Author | Changes |
|------|---------|--------|---------|
| 2026-02-05 | 1.0 | GitHub Copilot | Initial design document covering Phases 1-8 |

