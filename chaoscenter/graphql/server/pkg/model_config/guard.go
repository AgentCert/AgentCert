package model_config

import (
	"context"
	"sort"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/chaos_experiment/ops"
	dbChaosExperiment "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_experiment"
	"github.com/tidwall/gjson"
	"go.mongodb.org/mongo-driver/bson"
)

// experimentsUsing names the experiments of a project that would call alias
// after it is deleted: those with a run in flight, and enabled cron
// experiments, whose runs start on the cluster with no server check. Idle
// experiments are not scanned; their next save or run rejects the alias.
func (s *service) experimentsUsing(ctx context.Context, projectID, alias string) ([]string, error) {
	runs, err := s.runs.GetExperimentRuns(bson.D{
		{"project_id", projectID},
		{"is_removed", false},
		{"completed", false},
		{"phase", bson.D{{"$in", bson.A{string(model.ExperimentRunStatusRunning), string(model.ExperimentRunStatusQueued)}}}},
	})
	if err != nil {
		return nil, err
	}
	inFlight := map[string]bool{}
	ids := bson.A{}
	for _, run := range runs {
		if !inFlight[run.ExperimentID] {
			inFlight[run.ExperimentID] = true
			ids = append(ids, run.ExperimentID)
		}
	}
	experiments, err := s.experiments.GetExperiments(bson.D{
		{"project_id", projectID},
		{"is_removed", false},
		{"$or", bson.A{
			bson.D{{"experiment_id", bson.D{{"$in", ids}}}},
			bson.D{{"cron_syntax", bson.D{{"$ne", ""}}}},
		}},
	})
	if err != nil {
		return nil, err
	}
	var names []string
	for _, experiment := range experiments {
		manifest := newestManifest(experiment)
		if !inFlight[experiment.ExperimentID] && gjson.Get(manifest, "spec.suspend").Bool() {
			continue
		}
		for _, used := range ops.ManifestModelAliases(manifest, "") {
			if used == alias {
				names = append(names, experiment.Name)
				break
			}
		}
	}
	sort.Strings(names)
	return names, nil
}

func newestManifest(experiment dbChaosExperiment.ChaosExperimentRequest) string {
	newest := dbChaosExperiment.ExperimentRevision{}
	for _, revision := range experiment.Revision {
		if revision.UpdatedAt >= newest.UpdatedAt {
			newest = revision
		}
	}
	return newest.ExperimentManifest
}
