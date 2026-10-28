package model_config

import (
	"context"
	"time"

	dbModelConfig "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/model_config"
	"github.com/sirupsen/logrus"
)

const (
	reconcileInterval = 2 * time.Minute
	reconcileRetryMin = 5 * time.Second
	// pendingTimeout: a PENDING model LiteLLM does not have by then never
	// reached it, since a create gives LiteLLM adminTimeout to answer.
	pendingTimeout = 2 * adminTimeout
	// lostGrace: LiteLLM workers pick database models up on a 30 s poll, so
	// a model only just created or re-keyed may not be listed yet.
	lostGrace = 2 * time.Minute
)

// Reconcile settles every model whose status LiteLLM can decide:
//
//	PENDING    listed by LiteLLM → ACTIVE; not listed after pendingTimeout → removed (it never reached LiteLLM)
//	ACTIVE     not listed after lostGrace → NEEDS_KEY (LiteLLM's database lost it; a new key re-registers it)
//	NEEDS_KEY  listed by LiteLLM → ACTIVE
//	DELETING   deleted from LiteLLM, then soft-deleted
//
// A model is only judged missing while LiteLLM is connected to its database;
// without it LiteLLM lists no database models at all. Rows are read before
// LiteLLM's list and every transition is guarded on the row's updated_at, so
// a change made meanwhile by a user is never overwritten.
func (s *service) Reconcile(ctx context.Context) error {
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()

	rows, err := s.store.ListLive(ctx)
	if err != nil || len(rows) == 0 {
		return err
	}
	remote, err := s.gateway.ListModels(ctx)
	if err != nil {
		return err
	}
	connected, err := s.gateway.RegistryConnected(ctx)
	if err != nil {
		return err
	}
	listed := map[string]bool{}
	for _, served := range remote {
		if served.DBModel {
			listed[served.ModelID] = true
		}
	}

	now := s.now()
	recorded := map[string]bool{}
	for _, row := range rows {
		recorded[row.ModelID] = true
		guard := dbModelConfig.Guard{Statuses: []dbModelConfig.SyncStatus{row.SyncStatus}, UpdatedAt: row.UpdatedAt}
		switch row.SyncStatus {
		case dbModelConfig.Pending:
			if listed[row.ModelID] {
				s.transition(ctx, row, guard, dbModelConfig.Active, "")
			} else if connected && now.Sub(time.UnixMilli(row.CreatedAt)) > pendingTimeout {
				if _, err := s.store.Remove(ctx, row.ModelID, guard); err != nil {
					logrus.WithField("modelId", row.ModelID).WithError(err).Warn("[Model Registry] could not remove a model that never reached LiteLLM")
				} else {
					logrus.WithFields(logrus.Fields{"modelId": row.ModelID, "alias": row.Alias}).Info("[Model Registry] removed a model that never reached LiteLLM")
				}
			}
		case dbModelConfig.Active:
			if !listed[row.ModelID] && connected && now.Sub(time.UnixMilli(row.UpdatedAt)) > lostGrace {
				s.transition(ctx, row, guard, dbModelConfig.NeedsKey,
					"LiteLLM no longer has this model (was its database reset?); re-enter the API key to register it again")
			}
		case dbModelConfig.NeedsKey:
			if listed[row.ModelID] {
				s.transition(ctx, row, guard, dbModelConfig.Active, "")
			}
		case dbModelConfig.Deleting:
			if err := s.finishDelete(ctx, row); err != nil {
				logrus.WithField("modelId", row.ModelID).WithError(err).Warn("[Model Registry] delete still not confirmed by LiteLLM")
			}
		}
	}

	// Never delete what ACE did not create.
	for _, served := range remote {
		if served.DBModel && !recorded[served.ModelID] && !s.warnedOrphans[served.ModelID] {
			s.warnedOrphans[served.ModelID] = true
			logrus.WithFields(logrus.Fields{"modelId": served.ModelID, "alias": served.Alias}).
				Warn("[Model Registry] LiteLLM serves a database model ACE has no record of; leaving it alone")
		}
	}
	return nil
}

func (s *service) transition(ctx context.Context, row dbModelConfig.ModelConfig, guard dbModelConfig.Guard, to dbModelConfig.SyncStatus, lastError string) {
	ok, err := s.store.Update(ctx, row.ModelID, guard, dbModelConfig.Changes{Status: to, LastError: &lastError})
	logFields := logrus.Fields{"modelId": row.ModelID, "alias": row.Alias, "from": row.SyncStatus, "to": to}
	switch {
	case err != nil:
		logrus.WithFields(logFields).WithError(err).Warn("[Model Registry] reconcile could not update a model")
	case ok:
		logrus.WithFields(logFields).Info("[Model Registry] reconciled a model with LiteLLM")
	}
}

// RunReconciler reconciles now and every reconcileInterval until ctx ends.
// LiteLLM may start after this server, so failed passes retry sooner, with
// backoff.
func (s *service) RunReconciler(ctx context.Context) {
	retry := reconcileRetryMin
	for {
		next := reconcileInterval
		if err := s.Reconcile(ctx); err != nil {
			logrus.WithError(err).Warnf("[Model Registry] reconcile failed; retrying in %s", retry)
			next, retry = retry, min(retry*2, reconcileInterval)
		} else {
			retry = reconcileRetryMin
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(next):
		}
	}
}
