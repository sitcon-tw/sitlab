package httpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"example.com/project-template/internal/controller/application/apperror"
	appsync "example.com/project-template/internal/controller/application/sync"
	"example.com/project-template/internal/domain/board"
	"example.com/project-template/internal/domain/directory"
)

type cardOrderResponse struct {
	ListKey   string  `json:"listKey"`
	IssueIIDs []int64 `json:"issueIids"`
}

// syncActionResponse is the union the contract describes. Every payload field is
// optional on the wire and exactly one is set, chosen by entity.
type syncActionResponse struct {
	Entity            string                   `json:"entity"`
	SyncID            string                   `json:"syncId"`
	EntityID          string                   `json:"entityId"`
	Operation         string                   `json:"operation"`
	ActorGitLabUserID *int64                   `json:"actorGitLabUserId"`
	OccurredAt        time.Time                `json:"occurredAt"`
	Card              *cardResponse            `json:"card,omitempty"`
	Order             *cardOrderResponse       `json:"order,omitempty"`
	List              *boardListResponse       `json:"list,omitempty"`
	Team              *teamResponse            `json:"team,omitempty"`
	Member            *directoryMemberResponse `json:"member,omitempty"`
	Preferences       *preferencesResponse     `json:"preferences,omitempty"`
	Sync              *syncStatusResponse      `json:"sync,omitempty"`
}

type syncDeltaResponse struct {
	Checkpoint string               `json:"checkpoint"`
	Actions    []syncActionResponse `json:"actions"`
	HasMore    bool                 `json:"hasMore"`
}

// MarshalJSON keeps each union variant faithful to the contract. In particular, a
// delete must carry its required nullable payload field (for example, "card": null)
// rather than omitting it through a generic omitempty field.
func (response syncActionResponse) MarshalJSON() ([]byte, error) {
	wire := map[string]any{
		"entity": response.Entity, "syncId": response.SyncID,
		"entityId": response.EntityID, "operation": response.Operation,
		"actorGitLabUserId": response.ActorGitLabUserID, "occurredAt": response.OccurredAt,
	}
	switch response.Entity {
	case "card":
		wire["card"] = response.Card
	case "cardOrder":
		wire["order"] = response.Order
	case "list":
		wire["list"] = response.List
	case "team":
		wire["team"] = response.Team
	case "member":
		wire["member"] = response.Member
	case "preference":
		wire["preferences"] = response.Preferences
	case "syncStatus":
		wire["sync"] = response.Sync
	}
	return json.Marshal(wire)
}

// wireEntity translates the storage name to the contract's camelCase discriminant.
var wireEntity = map[string]string{
	"card":        "card",
	"card_order":  "cardOrder",
	"list":        "list",
	"team":        "team",
	"member":      "member",
	"preference":  "preference",
	"sync_status": "syncStatus",
}

func (h handler) syncDelta(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	since := r.URL.Query().Get("since")
	if since == "" {
		writeError(w, r, apperror.Malformed("since is required"))
		return
	}
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeError(w, r, apperror.Malformed("limit must be a positive integer"))
			return
		}
		limit = parsed
	}
	delta, err := h.sync.Delta(r.Context(), appsync.DeltaQuery{
		Since: since, AudienceUserID: claims.UserID, Limit: limit,
	})
	if errors.Is(err, board.ErrCheckpointTooOld) {
		writeError(w, r, apperror.Conflict("SYNC_CHECKPOINT_TOO_OLD", "the checkpoint can no longer be replayed; reload the board"))
		return
	}
	if err != nil {
		writeError(w, r, apperror.Unavailable("board changes are unavailable"))
		return
	}
	response, mapErr := mapSyncDelta(delta)
	if mapErr != nil {
		writeError(w, r, apperror.Unavailable("board changes are unavailable"))
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func mapSyncDelta(delta board.SyncDelta) (syncDeltaResponse, error) {
	actions := make([]syncActionResponse, 0, len(delta.Actions))
	for _, action := range delta.Actions {
		mapped, err := mapSyncAction(action)
		if err != nil {
			return syncDeltaResponse{}, err
		}
		actions = append(actions, mapped)
	}
	return syncDeltaResponse{Checkpoint: delta.Checkpoint, Actions: actions, HasMore: delta.HasMore}, nil
}

// mapSyncAction decodes the stored domain payload and renders the wire shape. The log
// holds domain values because the storage adapter may not reach into transport; this is
// where they become HTTP, reusing the same mappers the bootstrap payload goes through.
func mapSyncAction(action board.SyncAction) (syncActionResponse, error) {
	entity, known := wireEntity[action.Entity]
	if !known {
		return syncActionResponse{}, fmt.Errorf("unknown sync action entity %q", action.Entity)
	}
	response := syncActionResponse{
		Entity: entity, SyncID: action.SyncID, EntityID: action.EntityID,
		Operation: action.Operation, ActorGitLabUserID: action.ActorGitLabUserID,
		OccurredAt: action.OccurredAt.UTC(),
	}
	if action.Operation == "delete" {
		return response, nil
	}
	switch action.Entity {
	case "card":
		var card board.Card
		if err := json.Unmarshal(action.Payload, &card); err != nil {
			return syncActionResponse{}, err
		}
		mapped := mapCard(card)
		response.Card = &mapped
	case "card_order":
		var order struct {
			ListKey   string  `json:"listKey"`
			IssueIIDs []int64 `json:"issueIids"`
		}
		if err := json.Unmarshal(action.Payload, &order); err != nil {
			return syncActionResponse{}, err
		}
		response.Order = &cardOrderResponse{ListKey: order.ListKey, IssueIIDs: order.IssueIIDs}
	case "list":
		var list board.List
		if err := json.Unmarshal(action.Payload, &list); err != nil {
			return syncActionResponse{}, err
		}
		mapped := mapBoardList(list)
		response.List = &mapped
	case "team":
		var team directory.Team
		if err := json.Unmarshal(action.Payload, &team); err != nil {
			return syncActionResponse{}, err
		}
		mapped := mapTeam(team)
		response.Team = &mapped
	case "member":
		var member directory.Member
		if err := json.Unmarshal(action.Payload, &member); err != nil {
			return syncActionResponse{}, err
		}
		mapped := mapDirectoryMember(member)
		response.Member = &mapped
	case "preference":
		var preferences directory.Preferences
		if err := json.Unmarshal(action.Payload, &preferences); err != nil {
			return syncActionResponse{}, err
		}
		mapped := mapPreferences(preferences)
		response.Preferences = &mapped
	case "sync_status":
		var status board.SyncStatus
		if err := json.Unmarshal(action.Payload, &status); err != nil {
			return syncActionResponse{}, err
		}
		response.Sync = &syncStatusResponse{
			State: status.State, LastSuccessAt: status.LastSuccessAt, Message: optionalString(status.Message),
		}
	}
	return response, nil
}
