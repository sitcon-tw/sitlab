package sitcon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/project-template/internal/controller/infrastructure/postgres"
	domainboard "example.com/project-template/internal/domain/board"
	domaindirectory "example.com/project-template/internal/domain/directory"
	"example.com/project-template/internal/domain/identity"
)

type Repository struct {
	pool           *pgxpool.Pool
	subscriberMu   sync.Mutex
	subscribers    map[uint64]chan string
	nextSubscriber uint64
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, subscribers: make(map[uint64]chan string)}
}

func (r *Repository) Revision(ctx context.Context) (string, error) {
	var revision string
	if err := postgres.Executor(ctx, r.pool).QueryRow(ctx, `
		SELECT revision::text FROM realtime_state WHERE topic = 'bootstrap'
	`).Scan(&revision); err != nil {
		return "", fmt.Errorf("load bootstrap revision: %w", err)
	}
	return revision, nil
}

func (r *Repository) Status(ctx context.Context) (domainboard.SyncStatus, error) {
	var status domainboard.SyncStatus
	var hasError bool
	var message *string
	err := postgres.Executor(ctx, r.pool).QueryRow(ctx, `
		SELECT MIN(last_success_at), BOOL_OR(last_error IS NOT NULL),
		       NULLIF(string_agg(last_error, '; ' ORDER BY resource)
		           FILTER (WHERE last_error IS NOT NULL), '')
		FROM sync_snapshots
		HAVING COUNT(*) = 3
	`).Scan(&status.LastSuccessAt, &hasError, &message)
	if errors.Is(err, pgx.ErrNoRows) {
		return domainboard.SyncStatus{}, domainboard.ErrSnapshotNotFound
	}
	if err != nil {
		return domainboard.SyncStatus{}, fmt.Errorf("load sync status: %w", err)
	}
	status.State = "synced"
	if hasError {
		status.State = "offline"
	}
	if message != nil {
		status.Message = *message
	}
	return status, nil
}

func (r *Repository) ReadySnapshots(ctx context.Context) error {
	var ready bool
	err := postgres.Executor(ctx, r.pool).QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) = 3 FROM sync_snapshots) AND
			EXISTS (SELECT 1 FROM directory_teams WHERE active) AND
			EXISTS (SELECT 1 FROM directory_members WHERE state = 'active') AND
			EXISTS (SELECT 1 FROM board_lists)
	`).Scan(&ready)
	if err != nil {
		return fmt.Errorf("check snapshot readiness: %w", err)
	}
	if !ready {
		return domainboard.ErrSnapshotNotFound
	}
	return nil
}

func (r *Repository) ReplaceDirectory(ctx context.Context, snapshot domaindirectory.Snapshot) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		// Read what is stored before the rewrite so the log can carry the difference
		// rather than the whole directory.
		before, beforeErr := loadDirectorySnapshot(ctx, tx)
		if beforeErr != nil && !errors.Is(beforeErr, pgx.ErrNoRows) {
			return beforeErr
		}
		if _, err := tx.Exec(ctx, `
			UPDATE directory_teams
			SET active = false, source_revision = $1, updated_at = $2
		`, snapshot.SourceRevision, snapshot.SyncedAt); err != nil {
			return err
		}
		for _, team := range snapshot.Teams {
			if _, err := tx.Exec(ctx, `
				INSERT INTO directory_teams
				    (key, display_name, title_prefix, gitlab_label, sort_order, active, source_revision, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
				ON CONFLICT (key) DO UPDATE
				SET display_name = EXCLUDED.display_name,
				    title_prefix = EXCLUDED.title_prefix,
				    gitlab_label = EXCLUDED.gitlab_label,
				    sort_order = EXCLUDED.sort_order,
				    active = EXCLUDED.active,
				    source_revision = EXCLUDED.source_revision,
				    updated_at = EXCLUDED.updated_at
			`, team.Key, team.Name, team.TitlePrefix, team.GitLabLabel, team.SortOrder,
				team.Active, snapshot.SourceRevision, snapshot.SyncedAt); err != nil {
				return err
			}
		}

		memberIDs := make([]int64, 0, len(snapshot.Members))
		for _, member := range snapshot.Members {
			memberIDs = append(memberIDs, member.GitLabUserID)
			if _, err := tx.Exec(ctx, `
				INSERT INTO directory_members
				    (gitlab_user_id, username, display_name, avatar_url, profile_url,
				     access_level, state, last_synced_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
				ON CONFLICT (gitlab_user_id) DO UPDATE
				SET username = EXCLUDED.username,
				    display_name = EXCLUDED.display_name,
				    avatar_url = EXCLUDED.avatar_url,
				    profile_url = EXCLUDED.profile_url,
				    access_level = EXCLUDED.access_level,
				    state = EXCLUDED.state,
				    last_synced_at = EXCLUDED.last_synced_at
			`, member.GitLabUserID, member.Username, member.DisplayName, nullableString(member.AvatarURL),
				member.ProfileURL, member.AccessLevel, member.State, snapshot.SyncedAt); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM directory_members
			WHERE NOT (gitlab_user_id = ANY($1::bigint[]))
		`, memberIDs); err != nil {
			return err
		}
		credentialRows, err := tx.Query(ctx, `
			SELECT credential.user_id, credential.access_token_ciphertext,
			       credential.refresh_token_ciphertext
			FROM gitlab_oauth_credentials AS credential
			JOIN users AS account ON account.id = credential.user_id
			WHERE NOT EXISTS (
			    SELECT 1
			    FROM directory_members AS member
			    WHERE member.gitlab_user_id = account.gitlab_user_id
			      AND member.state = 'active'
			      AND member.access_level >= $1
			)
			FOR UPDATE OF credential
		`, identity.PlannerAccessLevel)
		if err != nil {
			return err
		}
		type revokedCredential struct {
			userID  uuid.UUID
			access  []byte
			refresh []byte
		}
		credentials := make([]revokedCredential, 0)
		for credentialRows.Next() {
			var userID uuid.UUID
			var access, refresh []byte
			if err := credentialRows.Scan(&userID, &access, &refresh); err != nil {
				credentialRows.Close()
				return err
			}
			credentials = append(credentials, revokedCredential{userID: userID, access: access, refresh: refresh})
		}
		rowsErr := credentialRows.Err()
		credentialRows.Close()
		if rowsErr != nil {
			return rowsErr
		}
		for _, credential := range credentials {
			if _, err := tx.Exec(ctx, `
				INSERT INTO gitlab_oauth_token_revocations
				    (id, user_id, access_token_ciphertext, refresh_token_ciphertext,
				     attempts, available_at, created_at, updated_at)
				VALUES ($1, $2, $3, $4, 0, $5, $5, $5)
			`, uuid.New(), credential.userID, credential.access, credential.refresh, snapshot.SyncedAt); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM auth_sessions AS session
			USING users AS account
			WHERE session.user_id = account.id
			  AND NOT EXISTS (
			      SELECT 1
			      FROM directory_members AS member
			      WHERE member.gitlab_user_id = account.gitlab_user_id
			        AND member.state = 'active'
			        AND member.access_level >= $1
			  )
		`, identity.PlannerAccessLevel); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM gitlab_oauth_credentials AS credential
			USING users AS account
			WHERE credential.user_id = account.id
			  AND NOT EXISTS (
			      SELECT 1
			      FROM directory_members AS member
			      WHERE member.gitlab_user_id = account.gitlab_user_id
			        AND member.state = 'active'
			        AND member.access_level >= $1
			  )
		`, identity.PlannerAccessLevel); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM directory_team_memberships WHERE source = 'gitlab_directory'`); err != nil {
			return err
		}
		for _, team := range snapshot.Teams {
			leaderIDs := make(map[int64]struct{}, len(team.LeaderGitLabUserIDs))
			for _, leaderID := range team.LeaderGitLabUserIDs {
				leaderIDs[leaderID] = struct{}{}
			}
			for _, memberID := range team.MemberGitLabUserIDs {
				_, isLeader := leaderIDs[memberID]
				if _, err := tx.Exec(ctx, `
					INSERT INTO directory_team_memberships (team_key, gitlab_user_id, source, is_leader, updated_at)
					VALUES ($1, $2, 'gitlab_directory', $3, $4)
				`, team.Key, memberID, isLeader, snapshot.SyncedAt); err != nil {
					return err
				}
			}
		}
		if _, err := tx.Exec(ctx, `DELETE FROM directory_milestones`); err != nil {
			return err
		}
		for _, milestone := range snapshot.Milestones {
			if _, err := tx.Exec(ctx, `
				INSERT INTO directory_milestones (date, name, kind, source_revision, updated_at)
				VALUES ($1::date, $2, $3, $4, $5)
			`, milestone.Date, milestone.Name, milestone.Kind, snapshot.SourceRevision, snapshot.SyncedAt); err != nil {
				return err
			}
		}
		for _, resource := range []string{"directory", "members"} {
			if _, err := tx.Exec(ctx, `
				INSERT INTO sync_snapshots
				    (resource, source_revision, last_success_at, last_attempt_at, last_error, updated_at)
				VALUES ($1, $2, $3, $3, NULL, $3)
				ON CONFLICT (resource) DO UPDATE
				SET source_revision = EXCLUDED.source_revision,
				    last_success_at = EXCLUDED.last_success_at,
				    last_attempt_at = EXCLUDED.last_attempt_at,
				    last_error = NULL,
				    updated_at = EXCLUDED.updated_at
			`, resource, snapshot.SourceRevision, snapshot.SyncedAt); err != nil {
				return err
			}
		}
		batch := newActionBatch("", snapshot.SyncedAt)
		if err := emitDirectoryChanges(ctx, tx, batch, before, snapshot); err != nil {
			return err
		}
		_, err = batch.flush(ctx, tx)
		return err
	})
}

// emitDirectoryChanges records only what a directory refresh actually altered. The
// refresh runs every five minutes and on every member webhook, so emitting the whole
// directory each time would be roughly seventeen thousand rows of noise a day.
func emitDirectoryChanges(ctx context.Context, tx pgx.Tx, batch *actionBatch, before, after domaindirectory.Snapshot) error {
	previousTeams := make(map[string]domaindirectory.Team, len(before.Teams))
	for _, team := range before.Teams {
		previousTeams[team.Key] = team
	}
	for _, team := range after.Teams {
		if existing, found := previousTeams[team.Key]; !found || !teamsEqual(existing, team) {
			batch.team(team)
		}
		delete(previousTeams, team.Key)
	}
	for key := range previousTeams {
		batch.deleteTeam(key)
	}

	previousMembers := make(map[int64]domaindirectory.Member, len(before.Members))
	for _, member := range before.Members {
		previousMembers[member.GitLabUserID] = member
	}
	for _, member := range after.Members {
		if existing, found := previousMembers[member.GitLabUserID]; !found || !membersEqual(existing, member) {
			batch.member(member)
		}
		delete(previousMembers, member.GitLabUserID)
	}
	for gitLabUserID := range previousMembers {
		batch.deleteMember(gitLabUserID)
	}

	if !slices.Equal(before.Milestones, after.Milestones) {
		batch.milestones(after.Milestones)
	}
	return nil
}

func teamsEqual(a, b domaindirectory.Team) bool {
	return a.Key == b.Key && a.Name == b.Name && a.TitlePrefix == b.TitlePrefix && a.GitLabLabel == b.GitLabLabel &&
		a.Active == b.Active && a.SortOrder == b.SortOrder &&
		slices.Equal(a.MemberGitLabUserIDs, b.MemberGitLabUserIDs) &&
		slices.Equal(a.LeaderGitLabUserIDs, b.LeaderGitLabUserIDs) &&
		slices.Equal(a.DirectoryMemberUsernames, b.DirectoryMemberUsernames) &&
		slices.Equal(a.DirectoryLeaderUsernames, b.DirectoryLeaderUsernames)
}

func membersEqual(a, b domaindirectory.Member) bool {
	return a.GitLabUserID == b.GitLabUserID && a.Username == b.Username && a.DisplayName == b.DisplayName &&
		a.AvatarURL == b.AvatarURL && a.ProfileURL == b.ProfileURL && a.AccessLevel == b.AccessLevel &&
		a.State == b.State && slices.Equal(a.TeamKeys, b.TeamKeys)
}

// syncActionRetention bounds how far back a client can be and still catch up
// incrementally. A week covers a laptop suspended on Friday and reopened on Monday; at
// this board's write rate that is a few megabytes.
const syncActionRetention = 7 * 24 * time.Hour

// Prune trims the tables that would otherwise grow without limit. durable_operations
// had no pruning at all, and dead webhook deliveries were kept forever.
func (r *Repository) Prune(ctx context.Context, now time.Time) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		// Raising the floor in the same statement that deletes is what keeps a client
		// from being told it can resume from a checkpoint that has just been removed.
		if _, err := tx.Exec(ctx, `
			WITH pruned AS (
			    DELETE FROM sync_actions WHERE created_at < $1 RETURNING sync_id
			)
			UPDATE realtime_state
			SET action_floor = GREATEST(action_floor, COALESCE((SELECT max(sync_id) FROM pruned), action_floor))
			WHERE topic = 'bootstrap'
		`, now.Add(-syncActionRetention)); err != nil {
			return err
		}
		// Never touch a failed operation: the board still offers it for retry. Never
		// touch one a card still points at.
		if _, err := tx.Exec(ctx, `
			DELETE FROM durable_operations
			WHERE state = 'synced'
			  AND updated_at < $1
			  AND NOT EXISTS (SELECT 1 FROM issue_cache WHERE pending_operation_id = durable_operations.id)
		`, now.Add(-syncActionRetention)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM gitlab_webhook_deliveries WHERE state = 'dead' AND updated_at < $1
		`, now.Add(-30*24*time.Hour)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM oauth_states WHERE expires_at < $1`, now); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `DELETE FROM auth_sessions WHERE expires_at < $1`, now)
		return err
	})
}

// SweepStartedAt reads PostgreSQL's clock so a full-board read can be bounded in the
// same clock domain the prune guard compares against.
func (r *Repository) SweepStartedAt(ctx context.Context) (time.Time, error) {
	var startedAt time.Time
	if err := postgres.Executor(ctx, r.pool).QueryRow(ctx, `SELECT now()`).Scan(&startedAt); err != nil {
		return time.Time{}, fmt.Errorf("read sweep start: %w", err)
	}
	return startedAt.UTC(), nil
}

// QuarantineCard records an issue the mapper could not place on the board. Keying the
// row by the GitLab timestamp we rejected means it is retried only once someone edits
// the issue, instead of on every poll.
func (r *Repository) QuarantineCard(ctx context.Context, issueIID int64, gitLabUpdatedAt time.Time, reason string, at time.Time) error {
	_, err := postgres.Executor(ctx, r.pool).Exec(ctx, `
		INSERT INTO board_sync_rejects (issue_iid, gitlab_updated_at, reason, attempts, first_seen_at, last_seen_at)
		VALUES ($1, $2, $3, 1, $4, $4)
		ON CONFLICT (issue_iid) DO UPDATE
		SET gitlab_updated_at = EXCLUDED.gitlab_updated_at,
		    reason = EXCLUDED.reason,
		    attempts = board_sync_rejects.attempts + 1,
		    last_seen_at = EXCLUDED.last_seen_at
	`, issueIID, gitLabUpdatedAt, reason, at)
	if err != nil {
		return fmt.Errorf("quarantine GitLab issue %d: %w", issueIID, err)
	}
	return nil
}

// EnsureBoardLists writes the fixed lane configuration. The lanes are compile-time
// constants, so this runs at startup rather than on every board read.
func (r *Repository) EnsureBoardLists(ctx context.Context, lists []domainboard.List, at time.Time) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		unchanged, err := boardListsMatch(ctx, tx, lists)
		if err != nil {
			return err
		}
		if unchanged {
			return nil
		}
		batch := newActionBatch("", at)
		for _, list := range lists {
			batch.list(list)
			if _, err := tx.Exec(ctx, `
				INSERT INTO board_lists (key, display_name, gitlab_status_name, position, closed, color, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
				ON CONFLICT (key) DO UPDATE
				SET display_name = EXCLUDED.display_name,
				    gitlab_status_name = EXCLUDED.gitlab_status_name,
				    position = EXCLUDED.position,
				    closed = EXCLUDED.closed,
				    color = EXCLUDED.color,
				    updated_at = EXCLUDED.updated_at
			`, list.Key, list.Name, list.GitLabStatusName, list.Position, list.Closed, list.Color, at); err != nil {
				return err
			}
		}
		_, err = batch.flush(ctx, tx)
		return err
	})
}

// BoardCursor reports where the incremental read left off. The watermark is stored as
// RFC3339Nano in sync_snapshots.source_revision, which used to hold a content hash of
// the whole board.
func (r *Repository) BoardCursor(ctx context.Context) (domainboard.SyncCursor, error) {
	var raw string
	err := postgres.Executor(ctx, r.pool).QueryRow(ctx, `
		SELECT source_revision FROM sync_snapshots WHERE resource = 'board'
	`).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return domainboard.SyncCursor{}, nil
	}
	if err != nil {
		return domainboard.SyncCursor{}, fmt.Errorf("load board cursor: %w", err)
	}
	watermark, parseErr := time.Parse(time.RFC3339Nano, raw)
	if parseErr != nil {
		// A snapshot written before the cursor became a timestamp, or seeded by a
		// test. Treat it as no cursor so the next read is a full sweep.
		return domainboard.SyncCursor{}, nil
	}
	return domainboard.SyncCursor{Watermark: &watermark}, nil
}

// ApplyBoardObservation merges one GitLab read and records that the board sync is
// healthy. It advances the incremental cursor only when the read established a new
// boundary; a sweep enumerates the project without moving it.
func (r *Repository) ApplyBoardObservation(ctx context.Context, observation domainboard.BoardObservation) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		var hadError bool
		snapshotErr := tx.QueryRow(ctx, `
			SELECT last_error IS NOT NULL FROM sync_snapshots WHERE resource = 'board'
		`).Scan(&hadError)
		if snapshotErr != nil && !errors.Is(snapshotErr, pgx.ErrNoRows) {
			return snapshotErr
		}
		cards := observation.Cards
		observations := make([]cardObservation, 0, len(cards)+len(observation.Removed))
		for index := range cards {
			observations = append(observations, cardObservation{IssueIID: cards[index].IssueIID, Card: &cards[index]})
		}
		for _, issueIID := range observation.Removed {
			observations = append(observations, cardObservation{IssueIID: issueIID})
		}
		actions := newActionBatch("", observation.SyncedAt)
		merge, err := applyCardObservations(ctx, tx, actions, observationBatch{
			Observations: observations, Complete: observation.Complete, Retained: observation.Retained,
			StartedAt: observation.StartedAt, ObservedAt: observation.SyncedAt,
		})
		if err != nil {
			return err
		}
		if observation.Watermark != nil {
			if _, err := tx.Exec(ctx, `
				INSERT INTO sync_snapshots
				    (resource, source_revision, last_success_at, last_attempt_at, last_error, updated_at)
				VALUES ('board', $1, $2, $2, NULL, $2)
				ON CONFLICT (resource) DO UPDATE
				SET source_revision = EXCLUDED.source_revision,
				    last_success_at = EXCLUDED.last_success_at,
				    last_attempt_at = EXCLUDED.last_attempt_at,
				    last_error = NULL,
				    updated_at = EXCLUDED.updated_at
			`, observation.Watermark.UTC().Format(time.RFC3339Nano), observation.SyncedAt); err != nil {
				return err
			}
		} else if _, err := tx.Exec(ctx, `
			INSERT INTO sync_snapshots
			    (resource, source_revision, last_success_at, last_attempt_at, last_error, updated_at)
			VALUES ('board', '', $1, $1, NULL, $1)
			ON CONFLICT (resource) DO UPDATE
			SET last_success_at = EXCLUDED.last_success_at,
			    last_attempt_at = EXCLUDED.last_attempt_at,
			    last_error = NULL,
			    updated_at = EXCLUDED.updated_at
		`, observation.SyncedAt); err != nil {
			return err
		}
		// A read that changed nothing must stay silent, or every poll wakes every
		// browser. Clearing a previous failure is itself worth announcing, because it
		// flips the board out of its offline indicator.
		if !merge.touched() && !hadError {
			return nil
		}
		if hadError {
			actions.syncStatus(domainboard.SyncStatus{State: "synced", LastSuccessAt: observation.SyncedAt})
		}
		_, err = actions.flush(ctx, tx)
		return err
	})
}

func boardListsMatch(ctx context.Context, tx pgx.Tx, expected []domainboard.List) (bool, error) {
	rows, err := tx.Query(ctx, `
		SELECT key, display_name, gitlab_status_name, position, closed, color
		FROM board_lists
		ORDER BY position, key
	`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	actual := make([]domainboard.List, 0, len(expected))
	for rows.Next() {
		var list domainboard.List
		if err := rows.Scan(&list.Key, &list.Name, &list.GitLabStatusName, &list.Position, &list.Closed, &list.Color); err != nil {
			return false, err
		}
		actual = append(actual, list)
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return slices.Equal(actual, expected), nil
}

func (r *Repository) RecordSyncFailure(ctx context.Context, resource string, attemptedAt time.Time, detail string) error {
	_, err := postgres.Executor(ctx, r.pool).Exec(ctx, `
		UPDATE sync_snapshots
		SET last_attempt_at = $2, last_error = $3, updated_at = $2
		WHERE resource = $1
	`, resource, attemptedAt, detail)
	return err
}

func (r *Repository) ClaimOperation(ctx context.Context, now time.Time) (domainboard.PendingOperation, error) {
	var pending domainboard.PendingOperation
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		var issueIID *int64
		var lastError *string
		err := tx.QueryRow(ctx, `
			SELECT operation.id, operation.kind, operation.issue_iid, operation.state,
			       operation.attempts, operation.last_error_detail, operation.requested_by_user_id,
			       operation.created_at, operation.updated_at
			FROM durable_operations operation
			WHERE (
			        (operation.state = 'pending' AND operation.available_at <= $1)
			        OR (operation.state = 'processing' AND operation.updated_at < $1 - interval '2 minutes')
			      )
			  AND (operation.kind = 'create_card' OR operation.issue_iid > 0)
			  AND NOT EXISTS (
			      SELECT 1
			      FROM durable_operations earlier
			      WHERE earlier.issue_iid = operation.issue_iid
			        AND (
			            earlier.created_at < operation.created_at
			            OR (earlier.created_at = operation.created_at AND earlier.id < operation.id)
			        )
			        AND earlier.state IN ('pending', 'processing')
			  )
			ORDER BY operation.created_at, operation.id
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		`, now).Scan(
			&pending.Operation.ID, &pending.Operation.Kind, &issueIID,
			&pending.Operation.State, &pending.Operation.Attempts, &lastError,
			&pending.RequestedByUserID,
			&pending.Operation.CreatedAt, &pending.Operation.UpdatedAt,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return domainboard.ErrOperationNotFound
		}
		if err != nil {
			return err
		}
		pending.Operation.IssueIID = issueIID
		if lastError != nil {
			pending.Operation.LastError = *lastError
		}
		if issueIID == nil {
			return fmt.Errorf("durable operation %s has no issue", pending.Operation.ID)
		}
		pending.Card, err = scanCard(tx.QueryRow(ctx, selectCards+` WHERE card.issue_iid = $1`, *issueIID))
		if err != nil {
			return err
		}
		pending.Operation.State = domainboard.OperationProcessing
		pending.Operation.Attempts++
		pending.Operation.UpdatedAt = now
		_, err = tx.Exec(ctx, `
			UPDATE durable_operations
			SET state = 'processing', attempts = attempts + 1, updated_at = $2
			WHERE id = $1
		`, uuid.MustParse(pending.Operation.ID), now)
		return err
	})
	if err != nil {
		return domainboard.PendingOperation{}, err
	}
	return pending, nil
}

func (r *Repository) CompleteOperation(ctx context.Context, pending domainboard.PendingOperation, issue domainboard.CanonicalIssue, completedAt time.Time) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		operationID := uuid.MustParse(pending.Operation.ID)
		// A create swaps the locally assigned negative IID for GitLab's real one, so
		// the card changes identity rather than merely changing content.
		renamed := pending.Operation.Kind == domainboard.OperationCreateCard && pending.Card.IssueIID != issue.IssueIID
		if pending.Operation.Kind == domainboard.OperationCreateCard {
			if _, err := tx.Exec(ctx, `
				DELETE FROM issue_cache
				WHERE issue_iid = $1 AND sync_state = 'synced'
			`, issue.IssueIID); err != nil {
				return err
			}
			command, err := tx.Exec(ctx, `
				UPDATE issue_cache
				SET issue_iid = $2,
				    gitlab_issue_id = $3,
				    web_url = $4,
				    description = $5,
				    labels = COALESCE($6::text[], '{}'),
				    gitlab_status_name = $7,
				    gitlab_updated_at = $8,
				    gitlab_observed_at = $11,
				    created_at = $9,
				    sync_state = CASE WHEN pending_operation_id = $10 THEN 'synced' ELSE sync_state END,
				    sync_error = CASE WHEN pending_operation_id = $10 THEN NULL ELSE sync_error END,
				    pending_operation_id = CASE WHEN pending_operation_id = $10 THEN NULL ELSE pending_operation_id END,
				    updated_at = $11
				WHERE issue_iid = $1
			`, pending.Card.IssueIID, issue.IssueIID, issue.GitLabIssueID,
				nullableString(issue.WebURL), issue.Description, issue.Labels, issue.GitLabStatusName, issue.UpdatedAt,
				issue.CreatedAt, operationID, completedAt)
			if err != nil {
				return err
			}
			if command.RowsAffected() == 0 {
				return domainboard.ErrCardNotFound
			}
		} else {
			command, err := tx.Exec(ctx, `
				UPDATE issue_cache
				SET gitlab_issue_id = $2,
				    web_url = $3,
				    description = CASE WHEN pending_operation_id = $4 THEN $5 ELSE description END,
				    labels = CASE WHEN pending_operation_id = $4 THEN COALESCE($6::text[], '{}') ELSE labels END,
				    gitlab_status_name = CASE WHEN pending_operation_id = $4 THEN $7 ELSE gitlab_status_name END,
				    gitlab_updated_at = $8,
				    gitlab_observed_at = $9,
				    sync_state = CASE WHEN pending_operation_id = $4 THEN 'synced' ELSE sync_state END,
				    sync_error = CASE WHEN pending_operation_id = $4 THEN NULL ELSE sync_error END,
				    pending_operation_id = CASE WHEN pending_operation_id = $4 THEN NULL ELSE pending_operation_id END,
				    updated_at = $9
				WHERE issue_iid = $1
			`, pending.Card.IssueIID, issue.GitLabIssueID, nullableString(issue.WebURL),
				operationID, issue.Description, issue.Labels, issue.GitLabStatusName, issue.UpdatedAt, completedAt)
			if err != nil {
				return err
			}
			if command.RowsAffected() == 0 {
				return domainboard.ErrCardNotFound
			}
		}
		_, err := tx.Exec(ctx, `
			UPDATE durable_operations
			SET state = 'synced', last_error_code = NULL, last_error_detail = NULL, updated_at = $2
			WHERE id = $1
		`, operationID, completedAt)
		if err != nil {
			return err
		}
		batch := newActionBatch(pending.RequestedByUserID, completedAt)
		if renamed {
			// The temporary negative IID no longer exists. Emitting the delete first
			// keeps a client from briefly holding the card under both identities.
			batch.deleteCard(pending.Card.IssueIID)
		}
		if err := emitCard(ctx, tx, batch, issue.IssueIID); err != nil {
			return err
		}
		if renamed {
			if err := emitLaneOrder(ctx, tx, batch, pending.Card.ListKey); err != nil {
				return err
			}
		}
		_, err = batch.flush(ctx, tx)
		return err
	})
}

func (r *Repository) FailOperation(ctx context.Context, pending domainboard.PendingOperation, failedAt time.Time, code, detail string) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		operationID := uuid.MustParse(pending.Operation.ID)
		if _, err := tx.Exec(ctx, `
			UPDATE durable_operations
			SET state = 'failed', last_error_code = $2, last_error_detail = $3, updated_at = $4
			WHERE id = $1
		`, operationID, code, detail, failedAt); err != nil {
			return err
		}
		if pending.Operation.Kind == domainboard.OperationCreateCard {
			_, err := tx.Exec(ctx, `
				UPDATE issue_cache
				SET sync_state = 'failed', sync_error = $2, pending_operation_id = $3, updated_at = $4
				WHERE issue_iid = $1
			`, pending.Card.IssueIID, detail, operationID, failedAt)
			if err != nil {
				return err
			}
		} else {
			if _, err := tx.Exec(ctx, `
				UPDATE issue_cache
				SET sync_state = 'failed', sync_error = $2, updated_at = $3
				WHERE issue_iid = $1 AND pending_operation_id = $4
			`, pending.Card.IssueIID, detail, failedAt, operationID); err != nil {
				return err
			}
		}
		batch := newActionBatch(pending.RequestedByUserID, failedAt)
		if err := emitCard(ctx, tx, batch, pending.Card.IssueIID); err != nil {
			return err
		}
		_, err := batch.flush(ctx, tx)
		return err
	})
}

// loadDirectorySnapshot reads teams and members inside the caller's transaction. It
// exists so ReplaceDirectory can diff against what is stored without reaching for a
// second connection, which would block on the locks this transaction already holds.
func loadDirectorySnapshot(ctx context.Context, tx pgx.Tx) (domaindirectory.Snapshot, error) {
	var snapshot domaindirectory.Snapshot
	teamRows, err := tx.Query(ctx, `
		SELECT key, display_name, title_prefix, gitlab_label, sort_order, active
		FROM directory_teams
		ORDER BY sort_order, key
	`)
	if err != nil {
		return snapshot, err
	}
	for teamRows.Next() {
		var team domaindirectory.Team
		if err := teamRows.Scan(&team.Key, &team.Name, &team.TitlePrefix, &team.GitLabLabel, &team.SortOrder, &team.Active); err != nil {
			teamRows.Close()
			return snapshot, err
		}
		snapshot.Teams = append(snapshot.Teams, team)
	}
	teamRows.Close()
	if err := teamRows.Err(); err != nil {
		return snapshot, err
	}

	memberRows, err := tx.Query(ctx, `
		SELECT member.gitlab_user_id, member.username, member.display_name,
		       member.avatar_url, member.profile_url, member.access_level, member.state,
		       COALESCE(array_agg(DISTINCT membership.team_key)
		           FILTER (WHERE membership.team_key IS NOT NULL), '{}')::text[]
		FROM directory_members member
		LEFT JOIN directory_team_memberships membership
		  ON membership.gitlab_user_id = member.gitlab_user_id
		GROUP BY member.gitlab_user_id
		ORDER BY lower(member.display_name), lower(member.username)
	`)
	if err != nil {
		return snapshot, err
	}
	defer memberRows.Close()
	for memberRows.Next() {
		var member domaindirectory.Member
		var avatarURL *string
		if err := memberRows.Scan(
			&member.GitLabUserID, &member.Username, &member.DisplayName, &avatarURL,
			&member.ProfileURL, &member.AccessLevel, &member.State, &member.TeamKeys,
		); err != nil {
			return snapshot, err
		}
		if avatarURL != nil {
			member.AvatarURL = *avatarURL
		}
		snapshot.Members = append(snapshot.Members, member)
	}
	if err := memberRows.Err(); err != nil {
		return snapshot, err
	}

	snapshot.Milestones, err = queryDirectoryMilestones(ctx, tx)
	return snapshot, err
}

func queryDirectoryMilestones(ctx context.Context, db postgres.DBTX) ([]domaindirectory.Milestone, error) {
	rows, err := db.Query(ctx, `
		SELECT name, date, kind
		FROM directory_milestones
		ORDER BY date, name
	`)
	if err != nil {
		return nil, fmt.Errorf("list directory milestones: %w", err)
	}
	defer rows.Close()
	milestones := make([]domaindirectory.Milestone, 0)
	for rows.Next() {
		var milestone domaindirectory.Milestone
		var date pgtype.Date
		if err := rows.Scan(&milestone.Name, &date, &milestone.Kind); err != nil {
			return nil, fmt.Errorf("scan directory milestone: %w", err)
		}
		milestone.Date = date.Time.Format(time.DateOnly)
		milestones = append(milestones, milestone)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate directory milestones: %w", err)
	}
	return milestones, nil
}

func (r *Repository) Snapshot(ctx context.Context) (domaindirectory.Snapshot, error) {
	db := postgres.Executor(ctx, r.pool)
	var revision string
	var syncedAt time.Time
	err := db.QueryRow(ctx, `
		SELECT directory.source_revision, LEAST(directory.last_success_at, members.last_success_at)
		FROM sync_snapshots directory
		JOIN sync_snapshots members ON members.resource = 'members'
		WHERE directory.resource = 'directory'
	`).Scan(&revision, &syncedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domaindirectory.Snapshot{}, domaindirectory.ErrSnapshotNotFound
	}
	if err != nil {
		return domaindirectory.Snapshot{}, fmt.Errorf("load directory revision: %w", err)
	}

	teamRows, err := db.Query(ctx, `
		SELECT key, display_name, title_prefix, gitlab_label, sort_order, active
		FROM directory_teams
		ORDER BY sort_order, key
	`)
	if err != nil {
		return domaindirectory.Snapshot{}, fmt.Errorf("list directory teams: %w", err)
	}
	defer teamRows.Close()
	teams := make([]domaindirectory.Team, 0)
	for teamRows.Next() {
		var team domaindirectory.Team
		if err := teamRows.Scan(&team.Key, &team.Name, &team.TitlePrefix, &team.GitLabLabel, &team.SortOrder, &team.Active); err != nil {
			return domaindirectory.Snapshot{}, fmt.Errorf("scan directory team: %w", err)
		}
		teams = append(teams, team)
	}
	if err := teamRows.Err(); err != nil {
		return domaindirectory.Snapshot{}, fmt.Errorf("iterate directory teams: %w", err)
	}

	memberRows, err := db.Query(ctx, `
		SELECT member.gitlab_user_id, member.username, member.display_name,
		       member.avatar_url, member.profile_url, member.access_level, member.state,
		       COALESCE(array_agg(DISTINCT membership.team_key)
		           FILTER (WHERE membership.team_key IS NOT NULL), '{}')::text[]
		FROM directory_members member
		LEFT JOIN directory_team_memberships membership
		  ON membership.gitlab_user_id = member.gitlab_user_id
		GROUP BY member.gitlab_user_id
		ORDER BY lower(member.display_name), lower(member.username)
	`)
	if err != nil {
		return domaindirectory.Snapshot{}, fmt.Errorf("list directory members: %w", err)
	}
	defer memberRows.Close()
	members := make([]domaindirectory.Member, 0)
	for memberRows.Next() {
		var member domaindirectory.Member
		var avatarURL *string
		if err := memberRows.Scan(
			&member.GitLabUserID, &member.Username, &member.DisplayName, &avatarURL,
			&member.ProfileURL, &member.AccessLevel, &member.State, &member.TeamKeys,
		); err != nil {
			return domaindirectory.Snapshot{}, fmt.Errorf("scan directory member: %w", err)
		}
		if avatarURL != nil {
			member.AvatarURL = *avatarURL
		}
		members = append(members, member)
	}
	if err := memberRows.Err(); err != nil {
		return domaindirectory.Snapshot{}, fmt.Errorf("iterate directory members: %w", err)
	}

	for memberIndex := range members {
		for _, teamKey := range members[memberIndex].TeamKeys {
			for teamIndex := range teams {
				if teams[teamIndex].Key == teamKey {
					teams[teamIndex].MemberGitLabUserIDs = append(teams[teamIndex].MemberGitLabUserIDs, members[memberIndex].GitLabUserID)
				}
			}
		}
	}
	directoryMembershipRows, err := db.Query(ctx, `
		SELECT membership.team_key, member.gitlab_user_id, member.username, membership.is_leader
		FROM directory_team_memberships membership
		JOIN directory_members member ON member.gitlab_user_id = membership.gitlab_user_id
		JOIN directory_teams team ON team.key = membership.team_key
		WHERE membership.source = 'gitlab_directory'
		ORDER BY team.sort_order, lower(member.display_name), lower(member.username)
	`)
	if err != nil {
		return domaindirectory.Snapshot{}, fmt.Errorf("list GitLab directory memberships: %w", err)
	}
	defer directoryMembershipRows.Close()
	for directoryMembershipRows.Next() {
		var teamKey, username string
		var memberID int64
		var isLeader bool
		if err := directoryMembershipRows.Scan(&teamKey, &memberID, &username, &isLeader); err != nil {
			return domaindirectory.Snapshot{}, fmt.Errorf("scan GitLab directory membership: %w", err)
		}
		for teamIndex := range teams {
			if teams[teamIndex].Key == teamKey {
				teams[teamIndex].DirectoryMemberUsernames = append(teams[teamIndex].DirectoryMemberUsernames, username)
				if isLeader {
					teams[teamIndex].LeaderGitLabUserIDs = append(teams[teamIndex].LeaderGitLabUserIDs, memberID)
					teams[teamIndex].DirectoryLeaderUsernames = append(teams[teamIndex].DirectoryLeaderUsernames, username)
				}
				break
			}
		}
	}
	if err := directoryMembershipRows.Err(); err != nil {
		return domaindirectory.Snapshot{}, fmt.Errorf("iterate GitLab directory memberships: %w", err)
	}
	milestones, err := queryDirectoryMilestones(ctx, db)
	if err != nil {
		return domaindirectory.Snapshot{}, err
	}
	return domaindirectory.Snapshot{Teams: teams, Members: members, Milestones: milestones, SourceRevision: revision, SyncedAt: syncedAt.UTC()}, nil
}

func (r *Repository) Preferences(ctx context.Context, userID string) (domaindirectory.Preferences, error) {
	var defaultTeamKey *string
	var confirmedAt *time.Time
	var directoryTeamKeys []string
	err := postgres.Executor(ctx, r.pool).QueryRow(ctx, `
		SELECT preference.default_team_key, preference.confirmed_at,
		       COALESCE(array_agg(DISTINCT membership.team_key)
		           FILTER (WHERE membership.source = 'gitlab_directory'), '{}')::text[]
		FROM users person
		LEFT JOIN user_preferences preference ON preference.user_id = person.id
		LEFT JOIN directory_team_memberships membership
		  ON membership.gitlab_user_id = person.gitlab_user_id
		WHERE person.id = $1
		GROUP BY person.id, preference.default_team_key, preference.confirmed_at
	`, uuid.MustParse(userID)).Scan(&defaultTeamKey, &confirmedAt, &directoryTeamKeys)
	if errors.Is(err, pgx.ErrNoRows) {
		return domaindirectory.Preferences{}, domaindirectory.ErrPreferencesNotFound
	}
	if err != nil {
		return domaindirectory.Preferences{}, fmt.Errorf("load preferences: %w", err)
	}
	return domaindirectory.Preferences{DefaultTeamKey: defaultTeamKey, ConfirmedAt: confirmedAt, DirectoryTeamKeys: directoryTeamKeys}, nil
}

func (r *Repository) SetPreferences(ctx context.Context, userID, teamKey string, confirmedAt time.Time) (domaindirectory.Preferences, error) {
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		var gitLabUserID int64
		if err := tx.QueryRow(ctx, `SELECT gitlab_user_id FROM users WHERE id = $1`, uuid.MustParse(userID)).Scan(&gitLabUserID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO user_preferences (user_id, default_team_key, confirmed_at, updated_at)
			VALUES ($1, $2, $3, $3)
			ON CONFLICT (user_id) DO UPDATE
			SET default_team_key = EXCLUDED.default_team_key,
			    confirmed_at = EXCLUDED.confirmed_at,
			    updated_at = EXCLUDED.updated_at
		`, uuid.MustParse(userID), teamKey, confirmedAt); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM directory_team_memberships
			WHERE gitlab_user_id = $1 AND source = 'self_selected'
		`, gitLabUserID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO directory_team_memberships (team_key, gitlab_user_id, source, updated_at)
			VALUES ($1, $2, 'self_selected', $3)
			ON CONFLICT (team_key, gitlab_user_id, source) DO UPDATE
			SET updated_at = EXCLUDED.updated_at
		`, teamKey, gitLabUserID, confirmedAt)
		if err != nil {
			return err
		}
		batch := newActionBatch(userID, confirmedAt)
		// Two actions, not one. The preference itself is private to this user, but the
		// self-selected membership it writes changes the team list everyone sees on
		// that member, so it has to be broadcast as well.
		preferences, err := loadPreferences(ctx, tx, userID)
		if err != nil {
			return err
		}
		batch.preferences(userID, preferences)
		member, err := loadDirectoryMember(ctx, tx, gitLabUserID)
		if err != nil {
			return err
		}
		batch.member(member)
		_, err = batch.flush(ctx, tx)
		return err
	})
	if err != nil {
		return domaindirectory.Preferences{}, fmt.Errorf("set preferences transaction: %w", err)
	}
	return r.Preferences(ctx, userID)
}

func loadPreferences(ctx context.Context, tx pgx.Tx, userID string) (domaindirectory.Preferences, error) {
	var preferences domaindirectory.Preferences
	err := tx.QueryRow(ctx, `
		SELECT preference.default_team_key, preference.confirmed_at,
		       COALESCE(array_agg(DISTINCT membership.team_key)
		           FILTER (WHERE membership.source = 'gitlab_directory'), '{}')::text[]
		FROM users person
		LEFT JOIN user_preferences preference ON preference.user_id = person.id
		LEFT JOIN directory_team_memberships membership
		  ON membership.gitlab_user_id = person.gitlab_user_id
		WHERE person.id = $1
		GROUP BY person.id, preference.default_team_key, preference.confirmed_at
	`, uuid.MustParse(userID)).Scan(&preferences.DefaultTeamKey, &preferences.ConfirmedAt, &preferences.DirectoryTeamKeys)
	return preferences, err
}

func loadDirectoryMember(ctx context.Context, tx pgx.Tx, gitLabUserID int64) (domaindirectory.Member, error) {
	var member domaindirectory.Member
	var avatarURL *string
	err := tx.QueryRow(ctx, `
		SELECT member.gitlab_user_id, member.username, member.display_name,
		       member.avatar_url, member.profile_url, member.access_level, member.state,
		       COALESCE(array_agg(DISTINCT membership.team_key)
		           FILTER (WHERE membership.team_key IS NOT NULL), '{}')::text[]
		FROM directory_members member
		LEFT JOIN directory_team_memberships membership
		  ON membership.gitlab_user_id = member.gitlab_user_id
		WHERE member.gitlab_user_id = $1
		GROUP BY member.gitlab_user_id
	`, gitLabUserID).Scan(
		&member.GitLabUserID, &member.Username, &member.DisplayName, &avatarURL,
		&member.ProfileURL, &member.AccessLevel, &member.State, &member.TeamKeys,
	)
	if avatarURL != nil {
		member.AvatarURL = *avatarURL
	}
	return member, err
}

func (r *Repository) Board(ctx context.Context) (domainboard.Snapshot, error) {
	db := postgres.Executor(ctx, r.pool)
	var syncedAt time.Time
	if err := db.QueryRow(ctx, `SELECT last_success_at FROM sync_snapshots WHERE resource = 'board'`).Scan(&syncedAt); errors.Is(err, pgx.ErrNoRows) {
		return domainboard.Snapshot{}, domainboard.ErrSnapshotNotFound
	} else if err != nil {
		return domainboard.Snapshot{}, fmt.Errorf("load board revision: %w", err)
	}

	listRows, err := db.Query(ctx, `
		SELECT key, display_name, gitlab_status_name, position, closed, color
		FROM board_lists
		ORDER BY position, key
	`)
	if err != nil {
		return domainboard.Snapshot{}, fmt.Errorf("list board lists: %w", err)
	}
	defer listRows.Close()
	lists := make([]domainboard.List, 0)
	for listRows.Next() {
		var list domainboard.List
		if err := listRows.Scan(&list.Key, &list.Name, &list.GitLabStatusName, &list.Position, &list.Closed, &list.Color); err != nil {
			return domainboard.Snapshot{}, fmt.Errorf("scan board list: %w", err)
		}
		lists = append(lists, list)
	}
	if err := listRows.Err(); err != nil {
		return domainboard.Snapshot{}, fmt.Errorf("iterate board lists: %w", err)
	}

	cardRows, err := db.Query(ctx, selectCards+` ORDER BY board_list.position, card.position, card.issue_iid`)
	if err != nil {
		return domainboard.Snapshot{}, fmt.Errorf("list board cards: %w", err)
	}
	defer cardRows.Close()
	cards := make([]domainboard.Card, 0)
	for cardRows.Next() {
		card, err := scanCard(cardRows)
		if err != nil {
			return domainboard.Snapshot{}, fmt.Errorf("scan board card: %w", err)
		}
		cards = append(cards, card)
	}
	if err := cardRows.Err(); err != nil {
		return domainboard.Snapshot{}, fmt.Errorf("iterate board cards: %w", err)
	}
	return domainboard.Snapshot{Lists: lists, Cards: cards, SyncedAt: syncedAt.UTC()}, nil
}

func (r *Repository) Card(ctx context.Context, issueIID int64) (domainboard.Card, error) {
	row := postgres.Executor(ctx, r.pool).QueryRow(ctx, selectCards+` WHERE card.issue_iid = $1`, issueIID)
	card, err := scanCard(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domainboard.Card{}, domainboard.ErrCardNotFound
	}
	if err != nil {
		return domainboard.Card{}, fmt.Errorf("get board card: %w", err)
	}
	return card, nil
}

func (r *Repository) ByOperation(ctx context.Context, operationID string) (domainboard.Result, error) {
	var operation domainboard.Operation
	var issueIID *int64
	var lastError *string
	err := postgres.Executor(ctx, r.pool).QueryRow(ctx, `
		SELECT id, kind, issue_iid, state, attempts, last_error_detail, created_at, updated_at
		FROM durable_operations
		WHERE id = $1
	`, uuid.MustParse(operationID)).Scan(
		&operation.ID, &operation.Kind, &issueIID, &operation.State, &operation.Attempts,
		&lastError, &operation.CreatedAt, &operation.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domainboard.Result{}, domainboard.ErrOperationNotFound
	}
	if err != nil {
		return domainboard.Result{}, fmt.Errorf("get durable operation: %w", err)
	}
	operation.IssueIID = issueIID
	if lastError != nil {
		operation.LastError = *lastError
	}
	if issueIID == nil {
		return domainboard.Result{Operation: operation}, nil
	}
	card, err := r.Card(ctx, *issueIID)
	if err != nil {
		return domainboard.Result{}, err
	}
	return domainboard.Result{Card: card, Operation: operation}, nil
}

func (r *Repository) CreateCard(ctx context.Context, mutation domainboard.Mutation) (domainboard.Result, error) {
	payload, err := json.Marshal(mutation.Payload)
	if err != nil {
		return domainboard.Result{}, fmt.Errorf("encode create card operation: %w", err)
	}
	var result domainboard.Result
	err = pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO durable_operations
			    (id, kind, issue_iid, requested_by_user_id, payload, state, attempts, available_at, created_at, updated_at)
			VALUES ($1, $2, NULL, $3, $4, $5, 0, $6, $6, $6)
		`, uuid.MustParse(mutation.Operation.ID), mutation.Operation.Kind, uuid.MustParse(mutation.RequestedByUserID), payload, mutation.Operation.State, mutation.Operation.CreatedAt); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE issue_cache SET position = position + 1 WHERE list_key = $1`, mutation.Card.ListKey); err != nil {
			return err
		}
		var issueIID int64
		if err := tx.QueryRow(ctx, `
			INSERT INTO issue_cache
			    (title, description, list_key, position, team_key, start_date, due_date,
			     labels, gitlab_status_name, sync_state, pending_operation_id, created_at, updated_at)
			VALUES ($1, $2, $3, 0, $4, $5, $6, COALESCE($7::text[], '{}'), $8, $9, $10, $11, $12)
			RETURNING issue_iid
		`, mutation.Card.Title, mutation.Card.Description, mutation.Card.ListKey, mutation.Card.TeamKey,
			nullableDate(mutation.Card.StartDate), nullableDate(mutation.Card.DueDate), mutation.Card.Labels, mutation.Card.GitLabStatusName, mutation.Card.SyncState,
			uuid.MustParse(mutation.Card.PendingOperationID), mutation.Card.CreatedAt, mutation.Card.UpdatedAt,
		).Scan(&issueIID); err != nil {
			return err
		}
		if err := replaceCardAssignees(ctx, tx, issueIID, mutation.Card.AssigneeGitLabUserIDs); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE durable_operations SET issue_iid = $1 WHERE id = $2`, issueIID, uuid.MustParse(mutation.Operation.ID)); err != nil {
			return err
		}
		mutation.Card.IssueIID = issueIID
		mutation.Operation.IssueIID = &issueIID
		result = domainboard.Result{Card: mutation.Card, Operation: mutation.Operation}
		batch := newActionBatch(mutation.RequestedByUserID, mutation.Card.UpdatedAt)
		if err := emitCard(ctx, tx, batch, issueIID); err != nil {
			return err
		}
		// The insert shifted every other card in the lane down by one.
		if err := emitLaneOrder(ctx, tx, batch, mutation.Card.ListKey); err != nil {
			return err
		}
		_, err := batch.flush(ctx, tx)
		return err
	})
	if operationConflict(err) {
		return domainboard.Result{}, domainboard.ErrOperationConflict
	}
	if err != nil {
		return domainboard.Result{}, fmt.Errorf("create optimistic card transaction: %w", err)
	}
	return result, nil
}

func (r *Repository) UpdateCard(ctx context.Context, mutation domainboard.Mutation) (domainboard.Result, error) {
	payload, err := json.Marshal(mutation.Payload)
	if err != nil {
		return domainboard.Result{}, fmt.Errorf("encode card operation: %w", err)
	}
	err = pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		batch := newActionBatch(mutation.RequestedByUserID, mutation.Card.UpdatedAt)
		if _, err := tx.Exec(ctx, `
			INSERT INTO durable_operations
			    (id, kind, issue_iid, requested_by_user_id, payload, state, attempts, available_at, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, 0, $7, $7, $7)
		`, uuid.MustParse(mutation.Operation.ID), mutation.Operation.Kind, mutation.Card.IssueIID,
			uuid.MustParse(mutation.RequestedByUserID), payload, mutation.Operation.State, mutation.Operation.CreatedAt); err != nil {
			return err
		}
		var currentList string
		var currentPosition int32
		if err := tx.QueryRow(ctx, `
			SELECT list_key, position
			FROM issue_cache
			WHERE issue_iid = $1
		`, mutation.Card.IssueIID).Scan(&currentList, &currentPosition); errors.Is(err, pgx.ErrNoRows) {
			return domainboard.ErrCardNotFound
		} else if err != nil {
			return err
		}
		if currentList != mutation.Card.ListKey || currentPosition != mutation.Card.Position {
			position, err := reorderCardPositions(ctx, tx, batch, mutation.Card.IssueIID, currentList, mutation.Card.ListKey, mutation.Card.Position)
			if err != nil {
				return err
			}
			mutation.Card.Position = position
		}
		command, err := tx.Exec(ctx, `
			UPDATE issue_cache
			SET title = $2, description = $3, list_key = $4, position = $5, team_key = $6,
			    start_date = $7, due_date = $8, labels = COALESCE($9::text[], '{}'), gitlab_status_name = $10,
			    sync_state = $11, sync_error = NULL, pending_operation_id = $12, updated_at = $13
			WHERE issue_iid = $1
		`, mutation.Card.IssueIID, mutation.Card.Title, mutation.Card.Description, mutation.Card.ListKey, mutation.Card.Position,
			mutation.Card.TeamKey, nullableDate(mutation.Card.StartDate), nullableDate(mutation.Card.DueDate),
			mutation.Card.Labels, mutation.Card.GitLabStatusName, mutation.Card.SyncState, uuid.MustParse(mutation.Card.PendingOperationID), mutation.Card.UpdatedAt)
		if err != nil {
			return err
		}
		if command.RowsAffected() == 0 {
			return domainboard.ErrCardNotFound
		}
		if err := replaceCardAssignees(ctx, tx, mutation.Card.IssueIID, mutation.Card.AssigneeGitLabUserIDs); err != nil {
			return err
		}
		if err := emitCard(ctx, tx, batch, mutation.Card.IssueIID); err != nil {
			return err
		}
		_, err = batch.flush(ctx, tx)
		return err
	})
	if operationConflict(err) {
		return domainboard.Result{}, domainboard.ErrOperationConflict
	}
	if err != nil {
		return domainboard.Result{}, fmt.Errorf("update optimistic card transaction: %w", err)
	}
	return domainboard.Result{Card: mutation.Card, Operation: mutation.Operation}, nil
}

func (r *Repository) RetryOperation(ctx context.Context, operationID string) (domainboard.Operation, error) {
	var operation domainboard.Operation
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		var issueIID *int64
		err := tx.QueryRow(ctx, `
			UPDATE durable_operations
			SET state = 'pending', available_at = now(), last_error_code = NULL,
			    last_error_detail = NULL, updated_at = now()
			WHERE id = $1 AND state = 'failed'
			RETURNING id, kind, issue_iid, state, attempts, created_at, updated_at
		`, uuid.MustParse(operationID)).Scan(
			&operation.ID, &operation.Kind, &issueIID, &operation.State,
			&operation.Attempts, &operation.CreatedAt, &operation.UpdatedAt,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			var exists bool
			if existsErr := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM durable_operations WHERE id = $1)`, uuid.MustParse(operationID)).Scan(&exists); existsErr != nil {
				return existsErr
			}
			if exists {
				return domainboard.ErrOperationConflict
			}
			return domainboard.ErrOperationNotFound
		}
		if err != nil {
			return err
		}
		operation.IssueIID = issueIID
		batch := newActionBatch("", operation.UpdatedAt)
		if issueIID != nil {
			if _, err := tx.Exec(ctx, `
				UPDATE issue_cache
				SET sync_state = 'pending', sync_error = NULL, pending_operation_id = $2, updated_at = now()
				WHERE issue_iid = $1
			`, *issueIID, uuid.MustParse(operationID)); err != nil {
				return err
			}
			if err := emitCard(ctx, tx, batch, *issueIID); err != nil {
				return err
			}
		}
		_, err = batch.flush(ctx, tx)
		return err
	})
	if err != nil {
		if errors.Is(err, domainboard.ErrOperationConflict) || errors.Is(err, domainboard.ErrOperationNotFound) {
			return domainboard.Operation{}, err
		}
		return domainboard.Operation{}, fmt.Errorf("retry durable operation: %w", err)
	}
	return operation, nil
}

const selectCards = `
	SELECT card.issue_iid, card.gitlab_issue_id, card.title, card.description, card.web_url,
	       card.list_key, card.position, card.team_key,
	       COALESCE((
	           SELECT array_agg(assignee.gitlab_user_id ORDER BY assignee.gitlab_user_id)
	           FROM issue_cache_assignees assignee
	           WHERE assignee.issue_iid = card.issue_iid
	       ), '{}'),
	       card.start_date, card.due_date, card.labels, card.gitlab_status_name, card.sync_state, card.sync_error,
	       card.pending_operation_id, card.created_at, card.updated_at
	FROM issue_cache card
	JOIN board_lists board_list ON board_list.key = card.list_key
`

type rowScanner interface {
	Scan(...any) error
}

func scanCard(row rowScanner) (domainboard.Card, error) {
	var card domainboard.Card
	var webURL, syncError *string
	var startDate, dueDate pgtype.Date
	var pendingOperationID *uuid.UUID
	err := row.Scan(
		&card.IssueIID, &card.GitLabIssueID, &card.Title, &card.Description, &webURL,
		&card.ListKey, &card.Position, &card.TeamKey, &card.AssigneeGitLabUserIDs,
		&startDate, &dueDate, &card.Labels, &card.GitLabStatusName, &card.SyncState, &syncError,
		&pendingOperationID, &card.CreatedAt, &card.UpdatedAt,
	)
	if err != nil {
		return domainboard.Card{}, err
	}
	if webURL != nil {
		card.WebURL = *webURL
	}
	if dueDate.Valid {
		card.DueDate = dueDate.Time.Format(time.DateOnly)
	}
	if startDate.Valid {
		card.StartDate = startDate.Time.Format(time.DateOnly)
	}
	if syncError != nil {
		card.SyncError = *syncError
	}
	if pendingOperationID != nil {
		card.PendingOperationID = pendingOperationID.String()
	}
	return card, nil
}

func replaceCardAssignees(ctx context.Context, tx pgx.Tx, issueIID int64, gitLabUserIDs []int64) error {
	if _, err := tx.Exec(ctx, `DELETE FROM issue_cache_assignees WHERE issue_iid = $1`, issueIID); err != nil {
		return err
	}
	if len(gitLabUserIDs) == 0 {
		return nil
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO issue_cache_assignees (issue_iid, gitlab_user_id)
		SELECT $1, assignee_id
		FROM unnest($2::bigint[]) AS assignee_id
		ON CONFLICT DO NOTHING
	`, issueIID, gitLabUserIDs)
	return err
}

// emitCard records a card as it now stands in the cache. The payload has to come from
// a re-read: several of these statements carry CASE guards that can leave a newer
// pending operation in place, so what the caller passed in is not necessarily what was
// stored.
func emitCard(ctx context.Context, tx pgx.Tx, batch *actionBatch, issueIID int64) error {
	card, err := scanCard(tx.QueryRow(ctx, selectCards+` WHERE card.issue_iid = $1`, issueIID))
	if err != nil {
		return err
	}
	batch.card(card)
	return nil
}

// emitLaneOrder reads a lane back for the paths that shift positions with arithmetic
// rather than by rewriting an explicit order.
func emitLaneOrder(ctx context.Context, tx pgx.Tx, batch *actionBatch, listKey string) error {
	rows, err := tx.Query(ctx, `SELECT issue_iid FROM issue_cache WHERE list_key = $1 ORDER BY position, issue_iid`, listKey)
	if err != nil {
		return err
	}
	defer rows.Close()
	order := make([]int64, 0)
	for rows.Next() {
		var issueIID int64
		if err := rows.Scan(&issueIID); err != nil {
			return err
		}
		order = append(order, issueIID)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	batch.laneOrder(listKey, order)
	return nil
}

func reorderCardPositions(ctx context.Context, tx pgx.Tx, batch *actionBatch, issueIID int64, sourceList, destinationList string, targetPosition int32) (int32, error) {
	_, orders, err := loadLaneOrders(ctx, tx, []string{sourceList, destinationList})
	if err != nil {
		return 0, err
	}
	before := orders.clone()
	orders.remove(sourceList, issueIID)
	normalizedPosition := orders.insertAt(destinationList, issueIID, targetPosition)
	if err := writeChangedLanes(ctx, tx, batch, before, orders); err != nil {
		return 0, err
	}
	return normalizedPosition, nil
}

func nullableDate(value string) any {
	if value == "" {
		return nil
	}
	parsed, _ := time.Parse(time.DateOnly, value)
	return parsed
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func operationConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "durable_operations_pkey"
}
