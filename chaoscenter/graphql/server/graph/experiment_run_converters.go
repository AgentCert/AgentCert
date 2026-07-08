package graph

import (
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
	expdef "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/experiment_definition"
)

func runStatusToGraphQL(s expdef.RunStatus) model.AceRunStatus {
	switch s {
	case expdef.RunStatusQueued:
		return model.AceRunStatusQueued
	case expdef.RunStatusRunning:
		return model.AceRunStatusRunning
	case expdef.RunStatusCompleted:
		return model.AceRunStatusCompleted
	case expdef.RunStatusFailed:
		return model.AceRunStatusFailed
	case expdef.RunStatusAborted:
		return model.AceRunStatusAborted
	default:
		return model.AceRunStatusQueued
	}
}

func graphqlRunStatusToService(s model.AceRunStatus) expdef.RunStatus {
	switch s {
	case model.AceRunStatusQueued:
		return expdef.RunStatusQueued
	case model.AceRunStatusRunning:
		return expdef.RunStatusRunning
	case model.AceRunStatusCompleted:
		return expdef.RunStatusCompleted
	case model.AceRunStatusFailed:
		return expdef.RunStatusFailed
	case model.AceRunStatusAborted:
		return expdef.RunStatusAborted
	default:
		return expdef.RunStatusQueued
	}
}

func statusEventsToGraphQL(events []expdef.StatusEvent) []*model.RunStatusEvent {
	out := make([]*model.RunStatusEvent, len(events))
	for i, e := range events {
		out[i] = &model.RunStatusEvent{
			Status:    runStatusToGraphQL(e.Status),
			Timestamp: e.Timestamp.UTC().Format("2006-01-02T15:04:05Z"),
			Reason:    nilIfEmpty(e.Reason),
		}
	}
	return out
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func runDocToGraphQL(doc *expdef.AceExperimentRunDoc) *model.AceExperimentRun {
	run := &model.AceExperimentRun{
		RunID:             doc.RunID,
		ProjectID:         doc.ProjectID,
		DefinitionName:    doc.DefinitionName,
		DefinitionVersion: doc.DefinitionVersion,
		AgentName:         doc.AgentName,
		AgentVersion:      doc.AgentVersion,
		ModelUsed:         doc.ModelUsed,
		ModelProvider:     doc.ModelProvider,
		ArgoWorkflowName:  doc.ArgoWorkflowName,
		LangfuseTraceID:   nilIfEmpty(doc.LangfuseTraceID),
		CertifierReportID: nilIfEmpty(doc.CertifierReportID),
		Status:            runStatusToGraphQL(doc.Status),
		StatusHistory:     statusEventsToGraphQL(doc.StatusHistory),
		CreatedAt:         doc.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		CreatedBy:         doc.CreatedBy,
	}
	if doc.StartedAt != nil {
		s := doc.StartedAt.UTC().Format("2006-01-02T15:04:05Z")
		run.StartedAt = &s
	}
	if doc.CompletedAt != nil {
		s := doc.CompletedAt.UTC().Format("2006-01-02T15:04:05Z")
		run.CompletedAt = &s
	}
	return run
}

func runDocsToGraphQL(docs []*expdef.AceExperimentRunDoc) []*model.AceExperimentRun {
	out := make([]*model.AceExperimentRun, len(docs))
	for i, d := range docs {
		out[i] = runDocToGraphQL(d)
	}
	return out
}
