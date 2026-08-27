package sitcon

import (
	"context"
	"slices"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"

	domainboard "example.com/project-template/internal/domain/board"
)

// cardState is the part of a cached card that ordering and merge decisions read.
type cardState struct {
	ListKey   string
	Position  int32
	SyncState domainboard.OperationState
	// GitLabUpdatedAt is GitLab's own clock, compared only against other GitLab
	// timestamps. GitLabObservedAt is PostgreSQL's, and records when a GitLab read
	// last confirmed the card exists.
	GitLabUpdatedAt  *time.Time
	GitLabObservedAt *time.Time
}

// laneOrders holds each lane's manual card order, most recent slot first. PostgreSQL
// owns this order: GitLab has no concept of it, so every writer -- a user drag, a
// webhook reconcile, and the board snapshot merge -- has to preserve it identically.
// Keeping one representation is what stops those three from drifting apart.
type laneOrders map[string][]int64

// loadLaneOrders locks the given lanes and returns the current card state plus each
// lane's order. Passing nil locks every lane that currently holds a card.
//
// Lanes are locked one statement at a time in sorted key order. A single
// `WHERE list_key = ANY(...) ... FOR UPDATE` would not do: PostgreSQL may take row
// locks during the scan and sort afterwards, so the ORDER BY does not constrain lock
// acquisition order and two writers touching the same pair of lanes could deadlock.
func loadLaneOrders(ctx context.Context, tx pgx.Tx, listKeys []string) (map[int64]cardState, laneOrders, error) {
	if listKeys == nil {
		discovered, err := allLaneKeys(ctx, tx)
		if err != nil {
			return nil, nil, err
		}
		listKeys = discovered
	} else {
		listKeys = slices.Clone(listKeys)
		sort.Strings(listKeys)
		listKeys = slices.Compact(listKeys)
	}
	states := make(map[int64]cardState)
	orders := make(laneOrders, len(listKeys))
	for _, listKey := range listKeys {
		rows, err := tx.Query(ctx, `
			SELECT issue_iid, list_key, position, sync_state, gitlab_updated_at, gitlab_observed_at
			FROM issue_cache
			WHERE list_key = $1
			ORDER BY position, issue_iid
			FOR UPDATE
		`, listKey)
		if err != nil {
			return nil, nil, err
		}
		order := make([]int64, 0)
		for rows.Next() {
			var issueIID int64
			var state cardState
			if err := rows.Scan(&issueIID, &state.ListKey, &state.Position, &state.SyncState, &state.GitLabUpdatedAt, &state.GitLabObservedAt); err != nil {
				rows.Close()
				return nil, nil, err
			}
			states[issueIID] = state
			order = append(order, issueIID)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, nil, err
		}
		orders[listKey] = order
	}
	return states, orders, nil
}

func allLaneKeys(ctx context.Context, tx pgx.Tx) ([]string, error) {
	rows, err := tx.Query(ctx, `SELECT DISTINCT list_key FROM issue_cache ORDER BY list_key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	keys := make([]string, 0)
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func (o laneOrders) clone() laneOrders {
	result := make(laneOrders, len(o))
	for listKey, order := range o {
		result[listKey] = slices.Clone(order)
	}
	return result
}

func (o laneOrders) remove(listKey string, issueIID int64) {
	order := o[listKey]
	if index := slices.Index(order, issueIID); index >= 0 {
		o[listKey] = slices.Delete(order, index, index+1)
	}
}

// prepend puts cards at the top of a lane. Newly observed cards and cards GitLab
// moved between lanes enter there, because GitLab cannot tell us where in the lane
// they belong.
func (o laneOrders) prepend(listKey string, issueIIDs ...int64) {
	if len(issueIIDs) == 0 {
		return
	}
	o[listKey] = append(slices.Clone(issueIIDs), o[listKey]...)
}

// insertAt places a card at a caller-chosen slot, clamping to the lane bounds, and
// reports the slot actually used.
func (o laneOrders) insertAt(listKey string, issueIID int64, target int32) int32 {
	o.remove(listKey, issueIID)
	order := o[listKey]
	position := max(min(int(target), len(order)), 0)
	o[listKey] = slices.Insert(order, position, issueIID)
	return int32(position)
}

// positions flattens the orders into the position value each card should hold.
func (o laneOrders) positions() map[int64]int32 {
	result := make(map[int64]int32)
	for _, order := range o {
		for position, issueIID := range order {
			result[issueIID] = int32(position)
		}
	}
	return result
}

// writeChangedLanes renumbers only the lanes whose order actually moved. Rewriting an
// unchanged lane would touch every row in it for nothing, and once the sync action log
// lands it would also broadcast a spurious reordering to every connected client.
func writeChangedLanes(ctx context.Context, tx pgx.Tx, batch *actionBatch, before, after laneOrders) error {
	listKeys := make([]string, 0, len(after))
	for listKey := range after {
		listKeys = append(listKeys, listKey)
	}
	sort.Strings(listKeys)
	for _, listKey := range listKeys {
		order := after[listKey]
		if slices.Equal(before[listKey], order) || len(order) == 0 {
			continue
		}
		if _, err := tx.Exec(ctx, `
			UPDATE issue_cache AS card
			SET position = (ordering.ordinality - 1)::integer
			FROM unnest($1::bigint[]) WITH ORDINALITY AS ordering(issue_iid, ordinality)
			WHERE card.issue_iid = ordering.issue_iid
		`, order); err != nil {
			return err
		}
		batch.laneOrder(listKey, order)
	}
	return nil
}
