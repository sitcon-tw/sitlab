package httpserver

import (
	"net/http"
	"strconv"

	"example.com/project-template/internal/controller/application/apperror"
	appboard "example.com/project-template/internal/controller/application/board"
	"github.com/go-chi/chi/v5"
)

func (h handler) bootstrapState(w http.ResponseWriter, r *http.Request) {
	result, err := h.bootstrap.Get(r.Context(), claimsFromContext(r.Context()))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, mapBootstrap(result))
}

func (h handler) directoryState(w http.ResponseWriter, r *http.Request) {
	snapshot, err := h.directory.Snapshot(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	teams := make([]teamResponse, 0, len(snapshot.Teams))
	for _, team := range snapshot.Teams {
		teams = append(teams, mapTeam(team))
	}
	members := make([]directoryMemberResponse, 0, len(snapshot.Members))
	for _, member := range snapshot.Members {
		members = append(members, mapDirectoryMember(member))
	}
	writeJSON(w, http.StatusOK, map[string]any{"directory": map[string]any{
		"teams": teams, "members": members,
		"sourceRevision": snapshot.SourceRevision, "syncedAt": snapshot.SyncedAt,
	}})
}

func (h handler) updatePreferences(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DefaultTeamKey string `json:"defaultTeamKey"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, r, err)
		return
	}
	result, err := h.directory.Update(r.Context(), actorID(r), body.DefaultTeamKey)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"preferences": mapPreferences(result)})
}

func (h handler) createCard(w http.ResponseWriter, r *http.Request) {
	var body struct {
		OperationID           string  `json:"operationId"`
		Title                 string  `json:"title"`
		Description           string  `json:"description"`
		TeamKey               string  `json:"teamKey"`
		AssigneeGitLabUserIDs []int64 `json:"assigneeGitLabUserIds"`
		StartDate             *string `json:"startDate"`
		DueDate               *string `json:"dueDate"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, r, err)
		return
	}
	result, err := h.board.Create(r.Context(), appboard.CreateInput{
		OperationID: body.OperationID, ActorUserID: actorID(r), Title: body.Title,
		Description: body.Description, TeamKey: body.TeamKey,
		AssigneeGitLabUserIDs: body.AssigneeGitLabUserIDs, StartDate: body.StartDate, DueDate: body.DueDate,
	})
	h.writeMutation(w, r, result, err)
}

func (h handler) updateCardDetails(w http.ResponseWriter, r *http.Request) {
	var body struct {
		OperationID string `json:"operationId"`
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, r, err)
		return
	}
	issueIID, err := issueIID(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	result, err := h.board.UpdateDetails(r.Context(), appboard.UpdateDetailsInput{
		OperationID: body.OperationID, ActorUserID: actorID(r), IssueIID: issueIID,
		Title: body.Title, Description: body.Description,
	})
	h.writeMutation(w, r, result, err)
}

func (h handler) updateCardTeam(w http.ResponseWriter, r *http.Request) {
	var body struct {
		OperationID string `json:"operationId"`
		TeamKey     string `json:"teamKey"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, r, err)
		return
	}
	issueIID, err := issueIID(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	result, err := h.board.UpdateTeam(r.Context(), appboard.UpdateTeamInput{
		OperationID: body.OperationID, ActorUserID: actorID(r), IssueIID: issueIID, TeamKey: body.TeamKey,
	})
	h.writeMutation(w, r, result, err)
}

func (h handler) updateCardAssignee(w http.ResponseWriter, r *http.Request) {
	var body struct {
		OperationID           string  `json:"operationId"`
		AssigneeGitLabUserIDs []int64 `json:"assigneeGitLabUserIds"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, r, err)
		return
	}
	issueIID, err := issueIID(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	result, err := h.board.UpdateAssignee(r.Context(), appboard.UpdateAssigneeInput{
		OperationID: body.OperationID, ActorUserID: actorID(r), IssueIID: issueIID,
		AssigneeGitLabUserIDs: body.AssigneeGitLabUserIDs,
	})
	h.writeMutation(w, r, result, err)
}

func (h handler) updateCardDueDate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		OperationID string  `json:"operationId"`
		DueDate     *string `json:"dueDate"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, r, err)
		return
	}
	issueIID, err := issueIID(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	result, err := h.board.UpdateDueDate(r.Context(), appboard.UpdateDueDateInput{
		OperationID: body.OperationID, ActorUserID: actorID(r), IssueIID: issueIID, DueDate: body.DueDate,
	})
	h.writeMutation(w, r, result, err)
}

func (h handler) updateCardStartDate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		OperationID string  `json:"operationId"`
		StartDate   *string `json:"startDate"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, r, err)
		return
	}
	issueIID, err := issueIID(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	result, err := h.board.UpdateStartDate(r.Context(), appboard.UpdateStartDateInput{
		OperationID: body.OperationID, ActorUserID: actorID(r), IssueIID: issueIID, StartDate: body.StartDate,
	})
	h.writeMutation(w, r, result, err)
}

func (h handler) moveCard(w http.ResponseWriter, r *http.Request) {
	var body struct {
		OperationID string `json:"operationId"`
		ListKey     string `json:"listKey"`
		Position    int32  `json:"position"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, r, err)
		return
	}
	issueIID, err := issueIID(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	result, err := h.board.Move(r.Context(), appboard.MoveInput{
		OperationID: body.OperationID, ActorUserID: actorID(r), IssueIID: issueIID,
		ListKey: body.ListKey, Position: body.Position,
	})
	h.writeMutation(w, r, result, err)
}

func (h handler) retryOperation(w http.ResponseWriter, r *http.Request) {
	operation, err := h.board.Retry(r.Context(), chi.URLParam(r, "operationId"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"operation": mapOperation(operation)})
}

func (h handler) refreshSnapshots(w http.ResponseWriter, r *http.Request) {
	user, err := h.auth.Me(r.Context(), actorID(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	if user.AccessLevel < 40 {
		writeError(w, r, apperror.Forbidden("FORBIDDEN", "Maintainer access is required to refresh snapshots"))
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"acceptedAt": h.sync.RequestRefresh()})
}

func (h handler) writeMutation(w http.ResponseWriter, r *http.Request, result appboard.Result, err error) {
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"card": mapCard(result.Card), "operation": mapOperation(result.Operation),
	})
}

func issueIID(r *http.Request) (int64, error) {
	value, err := strconv.ParseInt(chi.URLParam(r, "issueIid"), 10, 64)
	if err != nil {
		return 0, apperror.Invalid("VALIDATION_FAILED", "issue IID is invalid", apperror.Field{Name: "path.issueIid", Code: "INVALID_FORMAT", Message: "must be an integer"})
	}
	return value, nil
}
