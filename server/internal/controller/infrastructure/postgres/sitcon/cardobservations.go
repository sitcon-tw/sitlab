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

// observationResult reports what a merge did, so callers can decide whether the
// change is worth advertising to clients.
type observationResult struct {
	Changed    int
	Deleted    int
	Incomplete bool
}

func (r observationResult) touched() bool { return r.Changed > 0 || r.Deleted > 0 }

// applyCardObservations merges GitLab observations into issue_cache. It is the only
// writer of card rows and lane order for the GitLab-driven paths: the periodic board
// snapshot and single-issue webhook catch-up both route through it, so the rules below
// cannot drift between them.
//
// complete says the observations enumerate the whole board. Only a complete set may
// prune, because a partial read cannot distinguish "deleted" from "not mentioned".
//
// The rules, all of which protect local state from a slower GitLab read:
//   - A row that is not 'synced' has a local mutation in flight and is never touched.
//   - A row whose gitlab_updated_at is newer than the observation is left alone; both
//     sides of that comparison come from GitLab's clock, so it is skew-free.
//   - A card that stays in its lane keeps its slot.
//   - A new card, or one GitLab moved between lanes, enters at the top of its lane.
func applyCardObservations(ctx context.Context, tx pgx.Tx, observations []cardObservation, complete bool, observedAt time.Time) (observationResult, error) {
	var result observationResult
	states, orders, err := loadLaneOrders(ctx, tx, nil)
	if err != nil {
		return result, err
	}
	before := orders.clone()

	observed := make(map[int64]struct{}, len(observations))
	for _, observation := range observations {
		observed[observation.IssueIID] = struct{}{}
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
	for _, observation := range observations {
		if observation.Card == nil {
			remove(observation.IssueIID)
		}
	}
	if complete {
		for issueIID, state := range states {
			if issueIID <= 0 || state.SyncState != domainboard.OperationSynced {
				continue
			}
			if _, seen := observed[issueIID]; seen {
				continue
			}
			// A row GitLab did not mention but that we observed more recently than
			// this read was taken is not absent, only ahead of it.
			if state.GitLabUpdatedAt != nil && state.GitLabUpdatedAt.After(observedAt) {
				result.Incomplete = true
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
	}

	eligible := make([]domainboard.Card, 0, len(observations))
	for _, observation := range observations {
		if observation.Card == nil {
			continue
		}
		card := *observation.Card
		state, exists := states[card.IssueIID]
		if exists && state.SyncState != domainboard.OperationSynced {
			continue
		}
		if exists && state.GitLabUpdatedAt != nil && state.GitLabUpdatedAt.After(card.UpdatedAt) {
			result.Incomplete = true
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
		command, err := tx.Exec(ctx, `
			INSERT INTO issue_cache
			    (issue_iid, gitlab_issue_id, title, description, web_url, list_key, position, team_key,
			     start_date, due_date, labels, gitlab_status_name, sync_state, gitlab_updated_at,
			     created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, COALESCE($11::text[], '{}'),
			        $12, 'synced', $13, $14, $15)
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
			    updated_at = EXCLUDED.updated_at
			WHERE issue_cache.sync_state = 'synced'
			  AND (issue_cache.gitlab_updated_at IS NULL OR issue_cache.gitlab_updated_at <= EXCLUDED.gitlab_updated_at)
		`, card.IssueIID, card.GitLabIssueID, card.Title, card.Description, nullableString(card.WebURL),
			card.ListKey, positions[card.IssueIID], card.TeamKey, nullableDate(card.StartDate), nullableDate(card.DueDate),
			card.Labels, card.GitLabStatusName, card.UpdatedAt, card.CreatedAt, observedAt)
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
	}
	if err := writeChangedLanes(ctx, tx, before, orders); err != nil {
		return result, err
	}
	return result, nil
}
