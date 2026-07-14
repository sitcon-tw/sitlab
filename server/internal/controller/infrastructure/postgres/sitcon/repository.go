package sitcon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	appboard "example.com/project-template/internal/controller/application/board"
	appbootstrap "example.com/project-template/internal/controller/application/bootstrap"
	appdirectory "example.com/project-template/internal/controller/application/directory"
	appsync "example.com/project-template/internal/controller/application/sync"
	"example.com/project-template/internal/controller/infrastructure/postgres"
	domainboard "example.com/project-template/internal/domain/board"
	domaindirectory "example.com/project-template/internal/domain/directory"
)

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Status(ctx context.Context) (appbootstrap.SyncStatus, error) {
	var status appbootstrap.SyncStatus
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
		return appbootstrap.SyncStatus{}, domainboard.ErrSnapshotNotFound
	}
	if err != nil {
		return appbootstrap.SyncStatus{}, fmt.Errorf("load sync status: %w", err)
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
		if _, err := tx.Exec(ctx, `DELETE FROM directory_team_memberships WHERE source = 'gitlab_directory'`); err != nil {
			return err
		}
		for _, team := range snapshot.Teams {
			for _, memberID := range team.MemberGitLabUserIDs {
				if _, err := tx.Exec(ctx, `
					INSERT INTO directory_team_memberships (team_key, gitlab_user_id, source, updated_at)
					VALUES ($1, $2, 'gitlab_directory', $3)
				`, team.Key, memberID, snapshot.SyncedAt); err != nil {
					return err
				}
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
		return nil
	})
}

func (r *Repository) ReplaceBoard(ctx context.Context, lists []domainboard.List, cards []domainboard.Card, revision string, syncedAt time.Time) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		for _, list := range lists {
			if _, err := tx.Exec(ctx, `
				INSERT INTO board_lists (key, display_name, gitlab_label, position, closed, color, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
				ON CONFLICT (key) DO UPDATE
				SET display_name = EXCLUDED.display_name,
				    gitlab_label = EXCLUDED.gitlab_label,
				    position = EXCLUDED.position,
				    closed = EXCLUDED.closed,
				    color = EXCLUDED.color,
				    updated_at = EXCLUDED.updated_at
			`, list.Key, list.Name, list.GitLabLabel, list.Position, list.Closed, list.Color, syncedAt); err != nil {
				return err
			}
		}
		issueIIDs := make([]int64, 0, len(cards))
		for _, card := range cards {
			issueIIDs = append(issueIIDs, card.IssueIID)
			if _, err := tx.Exec(ctx, `
				INSERT INTO issue_cache
				    (issue_iid, gitlab_issue_id, title, web_url, list_key, position, team_key,
				     assignee_gitlab_user_id, due_date, labels, sync_state, gitlab_updated_at,
				     created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, COALESCE($10::text[], '{}'),
				        'synced', $11, $12, $12)
				ON CONFLICT (issue_iid) DO UPDATE
				SET gitlab_issue_id = EXCLUDED.gitlab_issue_id,
				    title = EXCLUDED.title,
				    web_url = EXCLUDED.web_url,
				    list_key = EXCLUDED.list_key,
				    position = EXCLUDED.position,
				    team_key = EXCLUDED.team_key,
				    assignee_gitlab_user_id = EXCLUDED.assignee_gitlab_user_id,
				    due_date = EXCLUDED.due_date,
				    labels = EXCLUDED.labels,
				    sync_state = 'synced',
				    sync_error = NULL,
				    pending_operation_id = NULL,
				    gitlab_updated_at = EXCLUDED.gitlab_updated_at,
				    updated_at = EXCLUDED.updated_at
				WHERE issue_cache.sync_state = 'synced'
			`, card.IssueIID, card.GitLabIssueID, card.Title, nullableString(card.WebURL),
				card.ListKey, card.Position, card.TeamKey, card.AssigneeGitLabUserID,
				nullableDate(card.DueDate), card.Labels, card.UpdatedAt, syncedAt); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM issue_cache
			WHERE issue_iid > 0 AND sync_state = 'synced'
			  AND NOT (issue_iid = ANY($1::bigint[]))
		`, issueIIDs); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO sync_snapshots
			    (resource, source_revision, last_success_at, last_attempt_at, last_error, updated_at)
			VALUES ('board', $1, $2, $2, NULL, $2)
			ON CONFLICT (resource) DO UPDATE
			SET source_revision = EXCLUDED.source_revision,
			    last_success_at = EXCLUDED.last_success_at,
			    last_attempt_at = EXCLUDED.last_attempt_at,
			    last_error = NULL,
			    updated_at = EXCLUDED.updated_at
		`, revision, syncedAt)
		return err
	})
}

func (r *Repository) RecordSyncFailure(ctx context.Context, resource string, attemptedAt time.Time, detail string) error {
	_, err := postgres.Executor(ctx, r.pool).Exec(ctx, `
		UPDATE sync_snapshots
		SET last_attempt_at = $2, last_error = $3, updated_at = $2
		WHERE resource = $1
	`, resource, attemptedAt, detail)
	return err
}

func (r *Repository) ClaimOperation(ctx context.Context, now time.Time) (appsync.PendingOperation, error) {
	var pending appsync.PendingOperation
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		var issueIID *int64
		var lastError *string
		err := tx.QueryRow(ctx, `
			SELECT operation.id, operation.kind, operation.issue_iid, operation.state,
			       operation.attempts, operation.last_error_detail,
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
		return appsync.PendingOperation{}, err
	}
	return pending, nil
}

func (r *Repository) CompleteOperation(ctx context.Context, pending appsync.PendingOperation, issue appsync.GitLabIssue, completedAt time.Time) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		operationID := uuid.MustParse(pending.Operation.ID)
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
				    labels = COALESCE($5::text[], '{}'),
				    gitlab_updated_at = $6,
				    sync_state = CASE WHEN pending_operation_id = $7 THEN 'synced' ELSE sync_state END,
				    sync_error = CASE WHEN pending_operation_id = $7 THEN NULL ELSE sync_error END,
				    pending_operation_id = CASE WHEN pending_operation_id = $7 THEN NULL ELSE pending_operation_id END,
				    updated_at = $8
				WHERE issue_iid = $1
			`, pending.Card.IssueIID, issue.IssueIID, issue.GitLabIssueID,
				nullableString(issue.WebURL), issue.Labels, issue.UpdatedAt, operationID, completedAt)
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
				    labels = CASE WHEN pending_operation_id = $4 THEN COALESCE($5::text[], '{}') ELSE labels END,
				    gitlab_updated_at = $6,
				    sync_state = CASE WHEN pending_operation_id = $4 THEN 'synced' ELSE sync_state END,
				    sync_error = CASE WHEN pending_operation_id = $4 THEN NULL ELSE sync_error END,
				    pending_operation_id = CASE WHEN pending_operation_id = $4 THEN NULL ELSE pending_operation_id END,
				    updated_at = $7
				WHERE issue_iid = $1
			`, pending.Card.IssueIID, issue.GitLabIssueID, nullableString(issue.WebURL),
				operationID, issue.Labels, issue.UpdatedAt, completedAt)
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
		return err
	})
}

func (r *Repository) FailOperation(ctx context.Context, pending appsync.PendingOperation, failedAt time.Time, code, detail string) error {
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
			return err
		}
		_, err := tx.Exec(ctx, `
			UPDATE issue_cache
			SET sync_state = 'failed', sync_error = $2, updated_at = $3
			WHERE issue_iid = $1 AND pending_operation_id = $4
		`, pending.Card.IssueIID, detail, failedAt, operationID)
		return err
	})
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
		SELECT membership.team_key, member.username
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
		if err := directoryMembershipRows.Scan(&teamKey, &username); err != nil {
			return domaindirectory.Snapshot{}, fmt.Errorf("scan GitLab directory membership: %w", err)
		}
		for teamIndex := range teams {
			if teams[teamIndex].Key == teamKey {
				teams[teamIndex].DirectoryMemberUsernames = append(teams[teamIndex].DirectoryMemberUsernames, username)
				break
			}
		}
	}
	if err := directoryMembershipRows.Err(); err != nil {
		return domaindirectory.Snapshot{}, fmt.Errorf("iterate GitLab directory memberships: %w", err)
	}
	return domaindirectory.Snapshot{Teams: teams, Members: members, SourceRevision: revision, SyncedAt: syncedAt.UTC()}, nil
}

func (r *Repository) Preferences(ctx context.Context, userID string) (appdirectory.Preferences, error) {
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
		return appdirectory.Preferences{}, domaindirectory.ErrPreferencesNotFound
	}
	if err != nil {
		return appdirectory.Preferences{}, fmt.Errorf("load preferences: %w", err)
	}
	return appdirectory.Preferences{DefaultTeamKey: defaultTeamKey, ConfirmedAt: confirmedAt, DirectoryTeamKeys: directoryTeamKeys}, nil
}

func (r *Repository) SetPreferences(ctx context.Context, userID, teamKey string, confirmedAt time.Time) (appdirectory.Preferences, error) {
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
		return err
	})
	if err != nil {
		return appdirectory.Preferences{}, fmt.Errorf("set preferences transaction: %w", err)
	}
	return r.Preferences(ctx, userID)
}

func (r *Repository) Board(ctx context.Context) (appboard.Snapshot, error) {
	db := postgres.Executor(ctx, r.pool)
	var syncedAt time.Time
	if err := db.QueryRow(ctx, `SELECT last_success_at FROM sync_snapshots WHERE resource = 'board'`).Scan(&syncedAt); errors.Is(err, pgx.ErrNoRows) {
		return appboard.Snapshot{}, domainboard.ErrSnapshotNotFound
	} else if err != nil {
		return appboard.Snapshot{}, fmt.Errorf("load board revision: %w", err)
	}

	listRows, err := db.Query(ctx, `
		SELECT key, display_name, gitlab_label, position, closed, color
		FROM board_lists
		ORDER BY position, key
	`)
	if err != nil {
		return appboard.Snapshot{}, fmt.Errorf("list board lists: %w", err)
	}
	defer listRows.Close()
	lists := make([]domainboard.List, 0)
	for listRows.Next() {
		var list domainboard.List
		if err := listRows.Scan(&list.Key, &list.Name, &list.GitLabLabel, &list.Position, &list.Closed, &list.Color); err != nil {
			return appboard.Snapshot{}, fmt.Errorf("scan board list: %w", err)
		}
		lists = append(lists, list)
	}
	if err := listRows.Err(); err != nil {
		return appboard.Snapshot{}, fmt.Errorf("iterate board lists: %w", err)
	}

	cardRows, err := db.Query(ctx, selectCards+` ORDER BY board_list.position, card.position, card.issue_iid`)
	if err != nil {
		return appboard.Snapshot{}, fmt.Errorf("list board cards: %w", err)
	}
	defer cardRows.Close()
	cards := make([]domainboard.Card, 0)
	for cardRows.Next() {
		card, err := scanCard(cardRows)
		if err != nil {
			return appboard.Snapshot{}, fmt.Errorf("scan board card: %w", err)
		}
		cards = append(cards, card)
	}
	if err := cardRows.Err(); err != nil {
		return appboard.Snapshot{}, fmt.Errorf("iterate board cards: %w", err)
	}
	return appboard.Snapshot{Lists: lists, Cards: cards, SyncedAt: syncedAt.UTC()}, nil
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

func (r *Repository) ByOperation(ctx context.Context, operationID string) (appboard.Result, error) {
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
		return appboard.Result{}, domainboard.ErrOperationNotFound
	}
	if err != nil {
		return appboard.Result{}, fmt.Errorf("get durable operation: %w", err)
	}
	operation.IssueIID = issueIID
	if lastError != nil {
		operation.LastError = *lastError
	}
	if issueIID == nil {
		return appboard.Result{Operation: operation}, nil
	}
	card, err := r.Card(ctx, *issueIID)
	if err != nil {
		return appboard.Result{}, err
	}
	return appboard.Result{Card: card, Operation: operation}, nil
}

func (r *Repository) CreateCard(ctx context.Context, mutation appboard.Mutation) (appboard.Result, error) {
	payload, err := json.Marshal(mutation.Payload)
	if err != nil {
		return appboard.Result{}, fmt.Errorf("encode create card operation: %w", err)
	}
	var result appboard.Result
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
			    (title, list_key, position, team_key, assignee_gitlab_user_id, due_date,
			     labels, sync_state, pending_operation_id, created_at, updated_at)
			VALUES ($1, $2, 0, $3, $4, $5, COALESCE($6::text[], '{}'), $7, $8, $9, $9)
			RETURNING issue_iid
		`, mutation.Card.Title, mutation.Card.ListKey, mutation.Card.TeamKey,
			mutation.Card.AssigneeGitLabUserID, nullableDate(mutation.Card.DueDate), mutation.Card.Labels,
			mutation.Card.SyncState, uuid.MustParse(mutation.Card.PendingOperationID), mutation.Card.UpdatedAt,
		).Scan(&issueIID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE durable_operations SET issue_iid = $1 WHERE id = $2`, issueIID, uuid.MustParse(mutation.Operation.ID)); err != nil {
			return err
		}
		mutation.Card.IssueIID = issueIID
		mutation.Operation.IssueIID = &issueIID
		result = appboard.Result{Card: mutation.Card, Operation: mutation.Operation}
		return nil
	})
	if operationConflict(err) {
		return appboard.Result{}, domainboard.ErrOperationConflict
	}
	if err != nil {
		return appboard.Result{}, fmt.Errorf("create optimistic card transaction: %w", err)
	}
	return result, nil
}

func (r *Repository) UpdateCard(ctx context.Context, mutation appboard.Mutation) (appboard.Result, error) {
	payload, err := json.Marshal(mutation.Payload)
	if err != nil {
		return appboard.Result{}, fmt.Errorf("encode card operation: %w", err)
	}
	err = pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO durable_operations
			    (id, kind, issue_iid, requested_by_user_id, payload, state, attempts, available_at, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, 0, $7, $7, $7)
		`, uuid.MustParse(mutation.Operation.ID), mutation.Operation.Kind, mutation.Card.IssueIID,
			uuid.MustParse(mutation.RequestedByUserID), payload, mutation.Operation.State, mutation.Operation.CreatedAt); err != nil {
			return err
		}
		command, err := tx.Exec(ctx, `
			UPDATE issue_cache
			SET title = $2, list_key = $3, position = $4, team_key = $5,
			    assignee_gitlab_user_id = $6, due_date = $7, labels = COALESCE($8::text[], '{}'),
			    sync_state = $9, sync_error = NULL, pending_operation_id = $10, updated_at = $11
			WHERE issue_iid = $1
		`, mutation.Card.IssueIID, mutation.Card.Title, mutation.Card.ListKey, mutation.Card.Position,
			mutation.Card.TeamKey, mutation.Card.AssigneeGitLabUserID, nullableDate(mutation.Card.DueDate),
			mutation.Card.Labels, mutation.Card.SyncState, uuid.MustParse(mutation.Card.PendingOperationID), mutation.Card.UpdatedAt)
		if err != nil {
			return err
		}
		if command.RowsAffected() == 0 {
			return domainboard.ErrCardNotFound
		}
		return nil
	})
	if operationConflict(err) {
		return appboard.Result{}, domainboard.ErrOperationConflict
	}
	if err != nil {
		return appboard.Result{}, fmt.Errorf("update optimistic card transaction: %w", err)
	}
	return appboard.Result{Card: mutation.Card, Operation: mutation.Operation}, nil
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
		if issueIID != nil {
			if _, err := tx.Exec(ctx, `
				UPDATE issue_cache
				SET sync_state = 'pending', sync_error = NULL, pending_operation_id = $2, updated_at = now()
				WHERE issue_iid = $1
			`, *issueIID, uuid.MustParse(operationID)); err != nil {
				return err
			}
		}
		return nil
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
	SELECT card.issue_iid, card.gitlab_issue_id, card.title, card.web_url,
	       card.list_key, card.position, card.team_key, card.assignee_gitlab_user_id,
	       card.due_date, card.labels, card.sync_state, card.sync_error,
	       card.pending_operation_id, card.updated_at
	FROM issue_cache card
	JOIN board_lists board_list ON board_list.key = card.list_key
`

type rowScanner interface {
	Scan(...any) error
}

func scanCard(row rowScanner) (domainboard.Card, error) {
	var card domainboard.Card
	var webURL, syncError *string
	var dueDate pgtype.Date
	var pendingOperationID *uuid.UUID
	err := row.Scan(
		&card.IssueIID, &card.GitLabIssueID, &card.Title, &webURL,
		&card.ListKey, &card.Position, &card.TeamKey, &card.AssigneeGitLabUserID,
		&dueDate, &card.Labels, &card.SyncState, &syncError,
		&pendingOperationID, &card.UpdatedAt,
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
	if syncError != nil {
		card.SyncError = *syncError
	}
	if pendingOperationID != nil {
		card.PendingOperationID = pendingOperationID.String()
	}
	return card, nil
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
