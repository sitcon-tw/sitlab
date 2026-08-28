package httpserver

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"example.com/project-template/internal/controller/application/apperror"
	apprelation "example.com/project-template/internal/controller/application/cardrelation"
)

func (h handler) listChildItems(w http.ResponseWriter, r *http.Request) {
	issueIID, query, err := relationshipPageRequest(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	page, err := h.relations.ChildItems(r.Context(), actorID(r), issueIID, query)
	if err != nil {
		writeError(w, r, err)
		return
	}
	items := make([]workItemResponse, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, mapWorkItem(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items, "totalCount": page.TotalCount, "nextCursor": optionalString(page.NextCursor),
	})
}

func (h handler) createChildItem(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title string `json:"title"`
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
	item, err := h.relations.CreateChild(r.Context(), apprelation.CreateChildInput{
		ActorUserID: actorID(r), IssueIID: issueIID, Title: body.Title,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, mapWorkItem(item))
}

func (h handler) attachChildItem(w http.ResponseWriter, r *http.Request) {
	input, err := childRelationInput(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if err := h.relations.AttachChild(r.Context(), input); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h handler) detachChildItem(w http.ResponseWriter, r *http.Request) {
	input, err := childRelationInput(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if err := h.relations.DetachChild(r.Context(), input); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h handler) listLinkedItems(w http.ResponseWriter, r *http.Request) {
	issueIID, query, err := relationshipPageRequest(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	page, err := h.relations.LinkedItems(r.Context(), actorID(r), issueIID, query)
	if err != nil {
		writeError(w, r, err)
		return
	}
	items := make([]linkedWorkItemResponse, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, mapLinkedWorkItem(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items, "totalCount": page.TotalCount, "nextCursor": optionalString(page.NextCursor),
	})
}

func (h handler) createLinkedItems(w http.ResponseWriter, r *http.Request) {
	var body struct {
		WorkItemIDs []int64 `json:"workItemIds"`
		LinkType    string  `json:"linkType"`
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
	err = h.relations.AddLinks(r.Context(), apprelation.LinkInput{
		ActorUserID: actorID(r), IssueIID: issueIID,
		WorkItemIDs: body.WorkItemIDs, LinkType: apprelation.LinkType(body.LinkType),
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h handler) deleteLinkedItem(w http.ResponseWriter, r *http.Request) {
	input, err := childRelationInput(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if err := h.relations.RemoveLink(r.Context(), input); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h handler) searchRelationshipCandidates(w http.ResponseWriter, r *http.Request) {
	issueIID, err := issueIID(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	items, err := h.relations.Search(r.Context(), apprelation.SearchInput{
		ActorUserID: actorID(r), IssueIID: issueIID,
		Kind: apprelation.RelationshipKind(r.URL.Query().Get("kind")), Query: r.URL.Query().Get("query"),
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	response := make([]workItemResponse, 0, len(items))
	for _, item := range items {
		response = append(response, mapWorkItem(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": response})
}

func relationshipPageRequest(r *http.Request) (int64, apprelation.PageQuery, error) {
	issueIID, err := issueIID(r)
	if err != nil {
		return 0, apprelation.PageQuery{}, err
	}
	query := apprelation.PageQuery{Cursor: r.URL.Query().Get("cursor")}
	if value := r.URL.Query().Get("limit"); value != "" {
		limit, parseErr := strconv.ParseInt(value, 10, 32)
		if parseErr != nil {
			return 0, apprelation.PageQuery{}, apperror.Invalid("VALIDATION_FAILED", "page limit is invalid", apperror.Field{
				Name: "query.limit", Code: "INVALID_FORMAT", Message: "must be an integer",
			})
		}
		query.Limit = int32(limit)
	}
	return issueIID, query, nil
}

func childRelationInput(r *http.Request) (apprelation.ChildRelationInput, error) {
	issueIID, err := issueIID(r)
	if err != nil {
		return apprelation.ChildRelationInput{}, err
	}
	workItemID, err := strconv.ParseInt(chi.URLParam(r, "workItemId"), 10, 64)
	if err != nil {
		return apprelation.ChildRelationInput{}, apperror.Invalid("VALIDATION_FAILED", "work item ID is invalid", apperror.Field{
			Name: "path.workItemId", Code: "INVALID_FORMAT", Message: "must be an integer",
		})
	}
	return apprelation.ChildRelationInput{
		ActorUserID: actorID(r), IssueIID: issueIID, WorkItemID: workItemID,
	}, nil
}
