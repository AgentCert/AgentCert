# AgentCert Detailed Data Model

## 8.1 Logical Data Categories

AgentCert stores platform state in MongoDB. Langfuse stores deep LLM traces and scoring telemetry that are linked to experiment runs.

This document separates:
- physical persistence model (MongoDB collections, keys, relationships)
- logical data categories (metadata, execution, telemetry, audit)

## 8.1.1 MongoDB Data Model

### Core Entity Map

| Collection | Primary Keys / IDs | Important Fields | Relationships |
|---|---|---|---|
| `project` | `_id` | `name`, `members[].user_id`, `members[].role`, `state`, `created_at`, `updated_at` | `project` 1:N `environment`, 1:N `agentRegistry`, 1:N `chaosExperiments` |
| `environment` | `environment_id` (unique when `is_removed=false`), `project_id` | `name`, `type`, `infra_ids[]`, `is_removed` | `project` 1:N `environment`; `environment` 1:N `apps_registrations` |
| `chaosInfrastructures` | `infra_id` (unique) | `project_id`, `environment_id`, `name`, `platform_name`, `infra_scope`, `is_active`, `is_registered`, `is_infra_confirmed` | `environment` 1:N `chaosInfrastructures`; `chaosInfrastructures` 1:N `chaosExperiments`; 1:N `chaosExperimentRuns` |
| `apps_registrations` | `appId` | `projectId`, `environmentId`, `name`, `namespace`, `method`, `status`, `auditInfo` | Belongs to `project` and `environment` |
| `agentRegistry` | `agentId` (unique) | `projectId`, `name`, `vendor`, `capabilities[]`, `endpoint`, `status`, `langfuseConfig`, `auditInfo` | Belongs to `project`; participates in benchmark workflows and trace correlation |
| `chaosExperiments` | `experiment_id` (unique), `project_id`, `infra_id` | `name`, `experiment_type`, `cron_syntax`, `revision[]`, `recent_experiment_run_details[]`, `total_experiment_runs` | `project` 1:N `chaosExperiments`; `chaosExperiments` 1:N `chaosExperimentRuns` |
| `chaosExperimentRuns` | `experiment_run_id` (indexed), `experiment_id`, `project_id`, `infra_id` | `phase`, `execution_data`, `resiliency_score`, `faults_*`, `probes[]`, `run_sequence`, `completed` | Run evidence linked to `chaosExperiments` and `chaosInfrastructures` |
| Supporting collections | Varies | `chaosProbes`, `chaosHubs`, `imageRegistry`, `serverConfig`, `gitops`, `faultStudios`, `user` | Reference catalogs, workflow settings, governance, and platform configuration |

### Key Cross-Cutting Embedded Structures

1. Audit fields (common in core Litmus collections)
- `created_at`, `updated_at`, `created_by`, `updated_by`, `is_removed`

2. Resource fields (common in core Litmus collections)
- `name`, `description`, `tags[]`

3. Experiment revisions
- `chaosExperiments.revision[]` stores versioned manifests and weightages
- each revision includes `revision_id`, `experiment_manifest`, `updated_at`, `weightages[]`, `probes[]`

4. Run snapshot denormalization
- `chaosExperiments.recent_experiment_run_details[]` keeps the latest run summaries for fast dashboard reads
- full evidence remains in `chaosExperimentRuns` (plus trace systems like Langfuse)

### Index and Constraint Highlights

1. `chaosInfrastructures`
- unique: `infra_id`
- secondary: `name`

2. `chaosExperiments`
- unique: `experiment_id`
- secondary: `name`

3. `chaosExperimentRuns`
- index: `experiment_run_id`

4. `environment`
- unique partial: `environment_id` where `is_removed=false`
- secondary: `name`

5. `chaosProbes`
- unique compound partial: `(name, project_id)` where `is_removed=false`

6. `agentRegistry`
- unique: `agentId`
- unique partial: `(projectId, name)` where `status="REGISTERED"`
- secondary: `projectId`, `status`

7. Other control-plane collections
- `gitops`: unique `project_id`
- `imageRegistry`: unique `project_id`
- `serverConfig`: unique `key`
- `chaosHubs`: unique `hub_id`
- `faultStudios`: unique `studio_id`, secondary `project_id`

### Naming Convention Notes

- Core Litmus collections primarily use snake_case keys: `project_id`, `experiment_id`, `environment_id`.
- Newer app/agent registry modules use camelCase keys: `projectId`, `appId`, `agentId`.
- Query and reporting layers should normalize these naming conventions explicitly to avoid mismatched joins or filters.

## 8.1.2 Logical Data Categories (Conceptual)

| Category | Purpose |
|---|---|
| Experiment Metadata | Experiment definitions, revisioning, scheduling configuration |
| Execution Logs | Runtime execution context, retries, rollout and bootstrap behavior, controller outcomes |
| Telemetry | Traces, metrics, and logs from OTEL/Langfuse/Kubernetes/Azure Monitor |
| Evaluation Results | Resiliency scoring, pass/fail fault statistics, benchmark outcomes |
| Audit Records | Change history, actor metadata, and governance evidence |

## 8.1.3 Relationship Summary

1. `project` is the tenancy root.
2. `environment` and `agentRegistry` are project-scoped.
3. `chaosInfrastructures` link environments to execution targets.
4. `chaosExperiments` bind project + infrastructure + revisioned definitions.
5. `chaosExperimentRuns` are immutable-style execution evidence records tied to experiments and infrastructure.
6. `apps_registrations` map target applications to project/environment context.
7. Supporting collections provide probes, hubs, workflow config, and operational policy data.

## 8.1.4 Correction to Previous Draft

The row labeled "Minimal agent-side integration" was conceptually describing the `agentRegistry` entity but was placed in the `Collection` column. It is corrected in this model as:
- Collection: `agentRegistry`
- Purpose (descriptive note): minimal onboarding footprint with trace stitching and endpoint metadata.
