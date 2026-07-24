package sitcon

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"example.com/project-template/internal/controller/infrastructure/postgres"
	domainboard "example.com/project-template/internal/domain/board"
)

const bootstrapNotificationChannel = "sitcon_bootstrap_changed"

func bumpBootstrapRevision(ctx context.Context, tx pgx.Tx, changedAt time.Time) (string, error) {
	var revision string
	if err := tx.QueryRow(ctx, `
		UPDATE realtime_state
		SET revision = revision + 1, updated_at = $1
		WHERE topic = 'bootstrap'
		RETURNING revision::text
	`, changedAt).Scan(&revision); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `SELECT pg_notify($1, $2)`, bootstrapNotificationChannel, revision); err != nil {
		return "", err
	}
	return revision, nil
}

func (r *Repository) EnqueueWebhook(ctx context.Context, delivery domainboard.WebhookDelivery) (bool, error) {
	command, err := postgres.Executor(ctx, r.pool).Exec(ctx, `
		INSERT INTO gitlab_webhook_deliveries
		    (id, scope, event_kind, event_name, issue_iid, state, attempts,
		     available_at, received_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 'pending', 0, $6, $6, $6)
		ON CONFLICT (id) DO NOTHING
	`, delivery.ID, delivery.Scope, delivery.EventKind, delivery.EventName, delivery.IssueIID, delivery.ReceivedAt)
	if err != nil {
		return false, fmt.Errorf("enqueue GitLab webhook: %w", err)
	}
	return command.RowsAffected() == 0, nil
}

func (r *Repository) ClaimWebhook(ctx context.Context, now time.Time) (domainboard.WebhookDelivery, error) {
	var delivery domainboard.WebhookDelivery
	var issueIID *int64
	var lastError *string
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			SELECT id, scope, event_kind, event_name, issue_iid, state, attempts,
			       last_error, available_at, received_at, updated_at
			FROM gitlab_webhook_deliveries
			WHERE (state = 'pending' AND available_at <= $1)
			   OR (state = 'processing' AND updated_at < $1 - interval '2 minutes')
			ORDER BY received_at, id
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		`, now).Scan(
			&delivery.ID, &delivery.Scope, &delivery.EventKind, &delivery.EventName,
			&issueIID, &delivery.State, &delivery.Attempts, &lastError,
			&delivery.AvailableAt, &delivery.ReceivedAt, &delivery.UpdatedAt,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return domainboard.ErrOperationNotFound
		}
		if err != nil {
			return err
		}
		delivery.IssueIID = issueIID
		if lastError != nil {
			delivery.LastError = *lastError
		}
		delivery.State = "processing"
		delivery.Attempts++
		delivery.UpdatedAt = now
		_, err = tx.Exec(ctx, `
			UPDATE gitlab_webhook_deliveries
			SET state = 'processing', attempts = attempts + 1, updated_at = $2
			WHERE id = $1
		`, delivery.ID, now)
		return err
	})
	if err != nil {
		return domainboard.WebhookDelivery{}, err
	}
	return delivery, nil
}

func (r *Repository) CompleteWebhook(ctx context.Context, id string, completedAt time.Time) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			UPDATE gitlab_webhook_deliveries
			SET state = 'completed', last_error = NULL, updated_at = $2
			WHERE id = $1
		`, id, completedAt); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			DELETE FROM gitlab_webhook_deliveries
			WHERE state = 'completed' AND updated_at < $1::timestamptz - interval '30 days'
		`, completedAt)
		return err
	})
}

func (r *Repository) FailWebhook(ctx context.Context, delivery domainboard.WebhookDelivery, failedAt time.Time, detail string) error {
	state := "pending"
	availableAt := failedAt.Add(webhookBackoff(delivery.Attempts))
	if delivery.Attempts >= 10 {
		state = "dead"
		availableAt = failedAt
	}
	_, err := postgres.Executor(ctx, r.pool).Exec(ctx, `
		UPDATE gitlab_webhook_deliveries
		SET state = $2, available_at = $3, last_error = $4, updated_at = $5
		WHERE id = $1
	`, delivery.ID, state, availableAt, detail, failedAt)
	return err
}

func webhookBackoff(attempt int32) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := time.Second << min(attempt-1, 6)
	return min(delay, time.Minute)
}

func (r *Repository) ReconcileIssue(ctx context.Context, issueIID int64, card *domainboard.Card, reconciledAt time.Time) (bool, error) {
	changed := false
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		var currentState domainboard.OperationState
		var currentList string
		var currentPosition int32
		var currentGitLabUpdatedAt *time.Time
		err := tx.QueryRow(ctx, `
			SELECT sync_state, list_key, position, gitlab_updated_at
			FROM issue_cache
			WHERE issue_iid = $1
			FOR UPDATE
		`, issueIID).Scan(&currentState, &currentList, &currentPosition, &currentGitLabUpdatedAt)
		exists := err == nil
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if exists && currentState != domainboard.OperationSynced {
			return nil
		}
		if card == nil {
			if !exists {
				return nil
			}
			command, deleteErr := tx.Exec(ctx, `DELETE FROM issue_cache WHERE issue_iid = $1 AND sync_state = 'synced'`, issueIID)
			if deleteErr != nil {
				return deleteErr
			}
			changed = command.RowsAffected() > 0
			if changed {
				if _, compactErr := tx.Exec(ctx, `
					UPDATE issue_cache SET position = position - 1
					WHERE list_key = $1 AND position > $2
				`, currentList, currentPosition); compactErr != nil {
					return compactErr
				}
			}
		} else {
			if currentGitLabUpdatedAt != nil && currentGitLabUpdatedAt.After(card.UpdatedAt) {
				return nil
			}
			if exists {
				if _, compactErr := tx.Exec(ctx, `
					UPDATE issue_cache SET position = position - 1
					WHERE list_key = $1 AND position > $2 AND issue_iid <> $3
				`, currentList, currentPosition, issueIID); compactErr != nil {
					return compactErr
				}
			}
			if _, shiftErr := tx.Exec(ctx, `
				UPDATE issue_cache SET position = position + 1
				WHERE list_key = $1 AND issue_iid <> $2
			`, card.ListKey, issueIID); shiftErr != nil {
				return shiftErr
			}
			_, upsertErr := tx.Exec(ctx, `
				INSERT INTO issue_cache
				    (issue_iid, gitlab_issue_id, title, description, web_url, list_key, position,
				     team_key, start_date, due_date, labels, sync_state, gitlab_updated_at,
				     created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, 0, $7, $8, $9, COALESCE($10::text[], '{}'),
				        'synced', $11, $12, $13)
				ON CONFLICT (issue_iid) DO UPDATE
				SET gitlab_issue_id = EXCLUDED.gitlab_issue_id,
				    title = EXCLUDED.title,
				    description = EXCLUDED.description,
				    web_url = EXCLUDED.web_url,
				    list_key = EXCLUDED.list_key,
				    position = 0,
				    team_key = EXCLUDED.team_key,
				    start_date = EXCLUDED.start_date,
				    due_date = EXCLUDED.due_date,
				    labels = EXCLUDED.labels,
				    sync_state = 'synced',
				    sync_error = NULL,
				    pending_operation_id = NULL,
				    gitlab_updated_at = EXCLUDED.gitlab_updated_at,
				    created_at = EXCLUDED.created_at,
				    updated_at = EXCLUDED.updated_at
			`, card.IssueIID, card.GitLabIssueID, card.Title, card.Description, nullableString(card.WebURL),
				card.ListKey, card.TeamKey, nullableDate(card.StartDate), nullableDate(card.DueDate), card.Labels,
				card.UpdatedAt, card.CreatedAt, reconciledAt)
			if upsertErr != nil {
				return upsertErr
			}
			if err := replaceCardAssignees(ctx, tx, issueIID, card.AssigneeGitLabUserIDs); err != nil {
				return err
			}
			changed = true
		}
		if !changed {
			return nil
		}
		if _, err := tx.Exec(ctx, `
			UPDATE sync_snapshots
			SET last_success_at = $1, last_attempt_at = $1, last_error = NULL, updated_at = $1
			WHERE resource = 'board'
		`, reconciledAt); err != nil {
			return err
		}
		_, err = bumpBootstrapRevision(ctx, tx, reconciledAt)
		return err
	})
	if err != nil {
		return false, fmt.Errorf("reconcile GitLab issue %d: %w", issueIID, err)
	}
	return changed, nil
}

func (r *Repository) SubscribeRevisions() (<-chan string, func()) {
	r.subscriberMu.Lock()
	r.nextSubscriber++
	id := r.nextSubscriber
	updates := make(chan string, 1)
	r.subscribers[id] = updates
	r.subscriberMu.Unlock()
	return updates, func() {
		r.subscriberMu.Lock()
		delete(r.subscribers, id)
		r.subscriberMu.Unlock()
	}
}

func (r *Repository) RunRevisionListener(ctx context.Context) {
	for ctx.Err() == nil {
		conn, err := r.pool.Acquire(ctx)
		if err != nil {
			if !waitForRetry(ctx) {
				return
			}
			continue
		}
		_, err = conn.Exec(ctx, `LISTEN `+bootstrapNotificationChannel)
		if err == nil {
			if revision, revisionErr := r.Revision(ctx); revisionErr == nil {
				r.broadcastRevision(revision)
			}
			for ctx.Err() == nil {
				notification, waitErr := conn.Conn().WaitForNotification(ctx)
				if waitErr != nil {
					break
				}
				r.broadcastRevision(notification.Payload)
			}
		}
		conn.Release()
		if !waitForRetry(ctx) {
			return
		}
	}
}

func (r *Repository) broadcastRevision(revision string) {
	r.subscriberMu.Lock()
	defer r.subscriberMu.Unlock()
	for _, subscriber := range r.subscribers {
		select {
		case subscriber <- revision:
		default:
			select {
			case <-subscriber:
			default:
			}
			select {
			case subscriber <- revision:
			default:
			}
		}
	}
}

func waitForRetry(ctx context.Context) bool {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (r *Repository) WebhookQueueStats(ctx context.Context) (int64, int64, float64, error) {
	var pending, dead int64
	var oldestSeconds float64
	err := postgres.Executor(ctx, r.pool).QueryRow(ctx, `
		SELECT
		    COUNT(*) FILTER (WHERE state IN ('pending', 'processing')),
		    COUNT(*) FILTER (WHERE state = 'dead'),
		    COALESCE(EXTRACT(EPOCH FROM (now() - MIN(received_at)
		        FILTER (WHERE state IN ('pending', 'processing')))), 0)
		FROM gitlab_webhook_deliveries
	`).Scan(&pending, &dead, &oldestSeconds)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("load GitLab webhook queue stats: %w", err)
	}
	return pending, dead, oldestSeconds, nil
}
