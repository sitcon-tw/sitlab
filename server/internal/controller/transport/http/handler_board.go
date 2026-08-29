package httpserver

import (
	"net/http"
	"strconv"

	"example.com/project-template/internal/controller/application/apperror"
	appboard "example.com/project-template/internal/controller/application/board"
	appactivity "example.com/project-template/internal/controller/application/cardactivity"
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
		"teams": teams, "members": members, "milestones": mapDirectoryMilestones(snapshot.Milestones),
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
		OperationID           string   `json:"operationId"`
		Title                 string   `json:"title"`
		Description           string   `json:"description"`
		TeamKey               string   `json:"teamKey"`
		ListKey               string   `json:"listKey"`
		AssigneeGitLabUserIDs []int64  `json:"assigneeGitLabUserIds"`
		Labels                []string `json:"labels"`
		StartDate             *string  `json:"startDate"`
		DueDate               *string  `json:"dueDate"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, r, err)
		return
	}
	result, err := h.board.Create(r.Context(), appboard.CreateInput{
		OperationID: body.OperationID, ActorUserID: actorID(r), Title: body.Title,
		Description: body.Description, TeamKey: body.TeamKey, ListKey: body.ListKey,
		AssigneeGitLabUserIDs: body.AssigneeGitLabUserIDs, Labels: body.Labels, StartDate: body.StartDate, DueDate: body.DueDate,
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

func (h handler) updateCardLabels(w http.ResponseWriter, r *http.Request) {
	var body struct {
		OperationID string   `json:"operationId"`
		Labels      []string `json:"labels"`
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
	result, err := h.board.UpdateLabels(r.Context(), appboard.UpdateLabelsInput{
		OperationID: body.OperationID, ActorUserID: actorID(r), IssueIID: issueIID, Labels: body.Labels,
	})
	h.writeMutation(w, r, result, err)
}

func (h handler) listCardComments(w http.ResponseWriter, r *http.Request) {
	issueIID, err := issueIID(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	comments, err := h.activity.Comments(r.Context(), actorID(r), issueIID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	response := make([]cardCommentResponse, 0, len(comments))
	for _, comment := range comments {
		response = append(response, mapCardComment(comment))
	}
	writeJSON(w, http.StatusOK, map[string]any{"comments": response})
}

func (h handler) createCardComment(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Body string `json:"body"`
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
	result, err := h.activity.CreateComment(r.Context(), appactivity.CreateCommentInput{
		ActorUserID: actorID(r), IssueIID: issueIID, Body: body.Body,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	if result.QuickActionsApplied {
		h.sync.RequestRefresh()
	}
	status := http.StatusAccepted
	var comment *cardCommentResponse
	if result.Comment != nil {
		mapped := mapCardComment(*result.Comment)
		comment = &mapped
		status = http.StatusCreated
	}
	writeJSON(w, status, map[string]any{
		"comment": comment, "quickActionsApplied": result.QuickActionsApplied, "summary": stringSlice(result.Summary),
	})
}

func (h handler) listQuickActions(w http.ResponseWriter, r *http.Request) {
	var issueIID int64
	if value := r.URL.Query().Get("issueIid"); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil || parsed <= 0 {
			writeError(w, r, apperror.Invalid("VALIDATION_FAILED", "issueIid must be a positive integer"))
			return
		}
		issueIID = parsed
	}
	commands, err := h.activity.QuickActions(r.Context(), actorID(r), issueIID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	items := make([]map[string]any, 0, len(commands))
	for _, command := range commands {
		items = append(items, map[string]any{
			"name": command.Name, "aliases": stringSlice(command.Aliases), "params": stringSlice(command.Params),
			"description": optionalString(command.Description), "warning": optionalString(command.Warning), "icon": optionalString(command.Icon),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"commands": items})
}

func stringSlice(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func (h handler) listQuickActionSuggestions(w http.ResponseWriter, r *http.Request) {
	var issueIID int64
	if value := r.URL.Query().Get("issueIid"); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil || parsed <= 0 {
			writeError(w, r, apperror.Invalid("VALIDATION_FAILED", "issueIid must be a positive integer"))
			return
		}
		issueIID = parsed
	}
	items, err := h.activity.QuickActionSuggestions(r.Context(), actorID(r), r.URL.Query().Get("kind"), r.URL.Query().Get("query"), issueIID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	suggestions := make([]map[string]any, 0, len(items))
	for _, item := range items {
		suggestions = append(suggestions, map[string]any{
			"id": item.ID, "kind": item.Kind, "value": item.Value, "label": item.Label,
			"detail": optionalString(item.Detail), "avatarUrl": optionalString(item.AvatarURL), "color": optionalString(item.Color),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"suggestions": suggestions})
}

func (h handler) moveCard(w http.ResponseWriter, r *http.Request) {
	var body struct {
		OperationID string `json:"operationId"`
		ListKey     string `json:"listKey"`
		Position    *int32 `json:"position"`
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
