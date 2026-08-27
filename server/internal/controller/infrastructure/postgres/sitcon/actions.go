package sitcon

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	domainboard "example.com/project-template/internal/domain/board"
	domaindirectory "example.com/project-template/internal/domain/directory"
)

// actionBatch collects everything one transaction changed, then writes it to the sync
// log and announces it, as that transaction's final act.
//
// Payloads carry domain values, not HTTP DTOs: this package may not import transport,
// and the mapping to the wire shape already exists there for the bootstrap payload.
//
// Actions are full-row snapshots rather than field diffs. Reconnects, Last-Event-ID
// overlap, extra browser tabs and post-drag catch-up all re-deliver the same action,
// and a snapshot applied twice is a no-op where a diff applied twice can corrupt. It
// also means the newest snapshot for an entity subsumes every earlier one, so a client
// returning after days can collapse a long replay to one action per entity.
type actionBatch struct {
	actorUserID string
	at          time.Time
	entries     []actionEntry
}

type actionEntry struct {
	entity     string
	entityID   string
	op         string
	audienceID string
	payload    []byte
	err        error
}

func newActionBatch(actorUserID string, at time.Time) *actionBatch {
	return &actionBatch{actorUserID: actorUserID, at: at}
}

func (b *actionBatch) upsert(entity, entityID string, value any) {
	payload, err := json.Marshal(value)
	b.entries = append(b.entries, actionEntry{entity: entity, entityID: entityID, op: "upsert", payload: payload, err: err})
}

func (b *actionBatch) remove(entity, entityID string) {
	b.entries = append(b.entries, actionEntry{entity: entity, entityID: entityID, op: "delete"})
}

func (b *actionBatch) card(card domainboard.Card) {
	b.upsert("card", formatIID(card.IssueIID), card)
}

func (b *actionBatch) deleteCard(issueIID int64) {
	b.remove("card", formatIID(issueIID))
}

// laneOrder records a lane's ordering as one action rather than a snapshot per card.
// A single drag renumbers every card in up to two lanes, so per-card snapshots would
// put tens of kilobytes on the wire for one drop.
func (b *actionBatch) laneOrder(listKey string, order []int64) {
	b.upsert("card_order", listKey, map[string]any{"listKey": listKey, "issueIids": order})
}

func (b *actionBatch) list(list domainboard.List) {
	b.upsert("list", list.Key, list)
}

func (b *actionBatch) team(team domaindirectory.Team) {
	b.upsert("team", team.Key, team)
}

func (b *actionBatch) deleteTeam(key string) {
	b.remove("team", key)
}

func (b *actionBatch) member(member domaindirectory.Member) {
	b.upsert("member", formatIID(member.GitLabUserID), member)
}

func (b *actionBatch) deleteMember(gitLabUserID int64) {
	b.remove("member", formatIID(gitLabUserID))
}

// preferences is audience-scoped: it is the one payload that belongs to a single user.
func (b *actionBatch) preferences(userID string, preferences domaindirectory.Preferences) {
	payload, err := json.Marshal(preferences)
	b.entries = append(b.entries, actionEntry{
		entity: "preference", entityID: userID, op: "upsert",
		audienceID: userID, payload: payload, err: err,
	})
}

func (b *actionBatch) syncStatus(status domainboard.SyncStatus) {
	b.upsert("sync_status", "board", status)
}

// flush replaces bumpBootstrapRevision at every call site. It must stay the final
// statement of its transaction: the revision UPDATE holds a row lock until commit, and
// that lock is exactly what makes sync ids gapless and commit-ordered.
func (b *actionBatch) flush(ctx context.Context, tx pgx.Tx) (string, error) {
	revision, err := bumpBootstrapRevision(ctx, tx, b.at)
	if err != nil {
		return "", err
	}
	if len(b.entries) == 0 {
		return revision, nil
	}
	entities := make([]string, 0, len(b.entries))
	entityIDs := make([]string, 0, len(b.entries))
	ops := make([]string, 0, len(b.entries))
	audiences := make([]*string, 0, len(b.entries))
	payloads := make([]*string, 0, len(b.entries))
	for _, entry := range b.entries {
		if entry.err != nil {
			return "", fmt.Errorf("encode %s sync action: %w", entry.entity, entry.err)
		}
		entities = append(entities, entry.entity)
		entityIDs = append(entityIDs, entry.entityID)
		ops = append(ops, entry.op)
		audiences = append(audiences, optionalText(entry.audienceID))
		if entry.op == "delete" {
			payloads = append(payloads, nil)
			continue
		}
		payload := string(entry.payload)
		payloads = append(payloads, &payload)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO sync_actions
		    (sync_id, seq, entity, entity_id, op, audience_user_id, payload, actor_user_id, created_at)
		SELECT $1::bigint, entry.ordinality - 1, entry.entity, entry.entity_id, entry.op,
		       entry.audience::uuid, entry.payload::jsonb, $7::uuid, $8
		FROM unnest($2::text[], $3::text[], $4::text[], $5::text[], $6::text[])
		     WITH ORDINALITY AS entry(entity, entity_id, op, audience, payload, ordinality)
	`, revision, entities, entityIDs, ops, audiences, payloads, optionalText(b.actorUserID), b.at); err != nil {
		return "", fmt.Errorf("record sync actions: %w", err)
	}
	return revision, nil
}

func optionalText(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func formatIID(value int64) string {
	return fmt.Sprintf("%d", value)
}
