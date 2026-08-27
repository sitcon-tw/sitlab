package sitcon

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	domainboard "example.com/project-template/internal/domain/board"
)

// cardObservation is what one GitLab read said about one issue. A nil Card means
// GitLab no longer reports it as a board card, either because the issue is gone or
// because it lost the active Team:: label that puts it on the board.
type cardObservation struct {
	IssueIID int64
	Card     *domainboard.Card
}

// observationBatch is one GitLab read handed to the cache as a unit.
type observationBatch struct {
	Observations []cardObservation
	// Complete marks a read that enumerated the whole board. Only a complete read may
	// prune, because a partial one cannot tell "deleted" from "not mentioned".
	Complete bool
	// StartedAt is PostgreSQL's clock read before the first GitLab page. Pruning
	// compares it against gitlab_observed_at, keeping both sides of that decision in
	// one clock domain. Required when Complete.
	StartedAt time.Time
	// ObservedAt is the app's wall clock, and only stamps updated_at.
	ObservedAt time.Time
	// Retained are issues GitLab reported but that could not be placed on a lane.
	// They are neither written nor pruned, so the board keeps showing the last good
	// version of the card instead of having it vanish while someone fixes GitLab.
	Retained []int64
}

// observationResult reports what a merge did, so callers can decide whether the
// change is worth advertising to clients.
type observationResult struct {
	Changed int
	Deleted int
	// Stale counts observations discarded because our copy was already newer.
	Stale int
}

func (r observationResult) touched() bool { return r.Changed > 0 || r.Deleted > 0 }

// applyCardObservations merges GitLab observations into issue_cache. It is the only
// writer of card rows and lane order for the GitLab-driven paths: the periodic board
// snapshot and single-issue webhook catch-up both route through it, so the rules below
// cannot drift between them.
//
// The rules, all of which protect local state from a slower GitLab read:
//   - A row that is not 'synced' has a local mutation in flight and is never touched.
//   - A row whose gitlab_updated_at is newer than the observation is left alone; both
//     sides of that comparison come from GitLab's clock, so it is skew-free.
//   - A card that stays in its lane keeps its slot.
//   - A new card, or one GitLab moved between lanes, enters at the top of its lane.
//
// Every skip here is self-resolving, which is why none of them holds back the caller's
// cursor. A row with a local mutation in flight is superseded when the operation worker
// completes it; a row we already hold newer data for needs no repair at all.
func applyCardObservations(ctx context.Context, tx pgx.Tx, actions *actionBatch, batch observationBatch) (observationResult, error) {
	var result observationResult
	states, orders, err := loadLaneOrders(ctx, tx, nil)
	if err != nil {
		return result, err
	}
	before := orders.clone()

	observed := make(map[int64]struct{}, len(batch.Observations)+len(batch.Retained))
	for _, observation := range batch.Observations {
		observed[observation.IssueIID] = struct{}{}
	}
	for _, issueIID := range batch.Retained {
		observed[issueIID] = struct{}{}
	}

	deleteIIDs := make([]int64, 0)
	remove := func(issueIID int64) {
		state, exists := states[issueIID]
		if !exists || state.SyncState != domainboard.OperationSynced {
			return
		}
		deleteIIDs = append(deleteIIDs, issueIID)
		orders.remove(state.ListKey, issueIID)
		delete(states, issueIID)
	}
	for _, observation := range batch.Observations {
		if observation.Card == nil {
			remove(observation.IssueIID)
		}
	}
	if batch.Complete && canPrune(states, observed) {
		for issueIID, state := range states {
			if issueIID <= 0 || state.SyncState != domainboard.OperationSynced {
				continue
			}
			if _, seen := observed[issueIID]; seen {
				continue
			}
			// A row confirmed by some other GitLab read at or after this one began is
			// not absent, only newer than what this read could see. The webhook that
			// reconciled it mid-fetch stamps gitlab_observed_at past StartedAt.
			if state.GitLabObservedAt != nil && !state.GitLabObservedAt.Before(batch.StartedAt) {
				continue
			}
			remove(issueIID)
		}
	}
	if len(deleteIIDs) > 0 {
		command, err := tx.Exec(ctx, `DELETE FROM issue_cache WHERE issue_iid = ANY($1::bigint[])`, deleteIIDs)
		if err != nil {
			return result, err
		}
		result.Deleted = int(command.RowsAffected())
		for _, issueIID := range deleteIIDs {
			actions.deleteCard(issueIID)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM board_sync_rejects WHERE issue_iid = ANY($1::bigint[])`, deleteIIDs); err != nil {
			return result, err
		}
	}

	eligible := make([]domainboard.Card, 0, len(batch.Observations))
	for _, observation := range batch.Observations {
		if observation.Card == nil {
			continue
		}
		card := *observation.Card
		state, exists := states[card.IssueIID]
		if exists && state.SyncState != domainboard.OperationSynced {
			continue
		}
		if exists && state.GitLabUpdatedAt != nil && state.GitLabUpdatedAt.After(card.UpdatedAt) {
			result.Stale++
			continue
		}
		eligible = append(eligible, card)
		if !exists || state.ListKey != card.ListKey {
			if exists {
				orders.remove(state.ListKey, card.IssueIID)
			}
			orders.prepend(card.ListKey, card.IssueIID)
		}
	}

	positions := orders.positions()
	for _, card := range eligible {
		command, err := tx.Exec(ctx, upsertCardSQL,
			card.IssueIID, card.GitLabIssueID, card.Title, card.Description, nullableString(card.WebURL),
			card.ListKey, positions[card.IssueIID], card.TeamKey, nullableDate(card.StartDate), nullableDate(card.DueDate),
			card.Labels, card.GitLabStatusName, card.UpdatedAt, card.CreatedAt, batch.ObservedAt)
		if err != nil {
			return result, err
		}
		if command.RowsAffected() == 0 {
			continue
		}
		result.Changed++
		if err := replaceCardAssignees(ctx, tx, card.IssueIID, card.AssigneeGitLabUserIDs); err != nil {
			return result, err
		}
		stored := card
		stored.Position = positions[card.IssueIID]
		stored.SyncState = domainboard.OperationSynced
		stored.SyncError, stored.PendingOperationID = "", ""
		actions.card(stored)
	}

	// Stamp every card GitLab confirmed, changed or not. This is what tells a later
	// sweep that the card still exists, so it must not be limited to the rows a merge
	// happened to rewrite.
	if len(observed) > 0 {
		confirmed := make([]int64, 0, len(observed))
		for _, observation := range batch.Observations {
			if observation.Card != nil {
				confirmed = append(confirmed, observation.IssueIID)
			}
		}
		if len(confirmed) > 0 {
			if _, err := tx.Exec(ctx, `
				UPDATE issue_cache SET gitlab_observed_at = $1 WHERE issue_iid = ANY($2::bigint[])
			`, batch.ObservedAt, confirmed); err != nil {
				return result, err
			}
			if _, err := tx.Exec(ctx, `
				DELETE FROM board_sync_rejects WHERE issue_iid = ANY($1::bigint[])
			`, confirmed); err != nil {
				return result, err
			}
		}
	}

	if err := writeChangedLanes(ctx, tx, actions, before, orders); err != nil {
		return result, err
	}
	return result, nil
}

// canPrune refuses to treat an empty read of a non-empty board as "everything was
// deleted". A GraphQL response that lost access, or any future filter bug, otherwise
// wipes the cache in one transaction.
func canPrune(states map[int64]cardState, observed map[int64]struct{}) bool {
	if len(observed) > 0 {
		return true
	}
	for issueIID, state := range states {
		if issueIID > 0 && state.SyncState == domainboard.OperationSynced {
			return false
		}
	}
	return true
}

// upsertCardSQL writes a card only when GitLab actually reports something different.
//
// The value comparison is what keeps the board quiet: without it the timestamp guard
// alone still rewrites a row whose gitlab_updated_at is merely equal, so every poll
// would touch every recently-changed card and, once the sync action log lands, push a
// no-op update to every connected client.
//
// position is deliberately absent from the comparison. PostgreSQL owns it and
// writeChangedLanes assigns it; including it would make an unrelated reordering look
// like a content change.
const upsertCardSQL = `
	INSERT INTO issue_cache
	    (issue_iid, gitlab_issue_id, title, description, web_url, list_key, position, team_key,
	     start_date, due_date, labels, gitlab_status_name, sync_state, gitlab_updated_at,
	     created_at, updated_at, gitlab_observed_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, COALESCE($11::text[], '{}'),
	        $12, 'synced', $13, $14, $15, $15)
	ON CONFLICT (issue_iid) DO UPDATE
	SET gitlab_issue_id = EXCLUDED.gitlab_issue_id,
	    title = EXCLUDED.title,
	    description = EXCLUDED.description,
	    web_url = EXCLUDED.web_url,
	    list_key = EXCLUDED.list_key,
	    position = EXCLUDED.position,
	    team_key = EXCLUDED.team_key,
	    start_date = EXCLUDED.start_date,
	    due_date = EXCLUDED.due_date,
	    labels = EXCLUDED.labels,
	    gitlab_status_name = EXCLUDED.gitlab_status_name,
	    sync_state = 'synced',
	    sync_error = NULL,
	    pending_operation_id = NULL,
	    gitlab_updated_at = EXCLUDED.gitlab_updated_at,
	    created_at = EXCLUDED.created_at,
	    updated_at = EXCLUDED.updated_at,
	    gitlab_observed_at = EXCLUDED.gitlab_observed_at
	WHERE issue_cache.sync_state = 'synced'
	  AND (issue_cache.gitlab_updated_at IS NULL OR issue_cache.gitlab_updated_at <= EXCLUDED.gitlab_updated_at)
	  AND (issue_cache.gitlab_issue_id, issue_cache.title, issue_cache.description, issue_cache.web_url,
	       issue_cache.list_key, issue_cache.team_key, issue_cache.start_date, issue_cache.due_date,
	       issue_cache.labels, issue_cache.gitlab_status_name, issue_cache.gitlab_updated_at)
	      IS DISTINCT FROM
	      (EXCLUDED.gitlab_issue_id, EXCLUDED.title, EXCLUDED.description, EXCLUDED.web_url,
	       EXCLUDED.list_key, EXCLUDED.team_key, EXCLUDED.start_date, EXCLUDED.due_date,
	       EXCLUDED.labels, EXCLUDED.gitlab_status_name, EXCLUDED.gitlab_updated_at)
`
