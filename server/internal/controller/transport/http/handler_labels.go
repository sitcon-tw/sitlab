package httpserver

import (
	"net/http"
	"strconv"

	"example.com/project-template/internal/controller/application/apperror"
	appactivity "example.com/project-template/internal/controller/application/cardactivity"
	"github.com/go-chi/chi/v5"
)

// Project labels are project-scoped rather than card-scoped, so they live on
// their own route and in their own handler file.
//
// There is deliberately no application-level access-level gate: these run as
// the acting user, so GitLab's own project role is the authorization. A member
// without permission receives GitLab's 403, which surfaces as ErrGitLabForbidden
// and then as apperror.Forbidden.

func (h handler) listProjectLabels(w http.ResponseWriter, r *http.Request) {
	labels, err := h.activity.Labels(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	response := make([]projectLabelResponse, 0, len(labels))
	for _, label := range labels {
		response = append(response, mapProjectLabel(label))
	}
	writeJSON(w, http.StatusOK, map[string]any{"labels": response})
}

func (h handler) createProjectLabel(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string  `json:"name"`
		Color       string  `json:"color"`
		Description *string `json:"description"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, r, err)
		return
	}
	label, err := h.activity.CreateLabel(r.Context(), appactivity.CreateLabelInput{
		ActorUserID: actorID(r), Name: body.Name, Color: body.Color, Description: body.Description,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, mapProjectLabel(label))
}

func (h handler) updateProjectLabel(w http.ResponseWriter, r *http.Request) {
	id, err := labelID(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	var body struct {
		Name        string  `json:"name"`
		Color       string  `json:"color"`
		Description *string `json:"description"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, r, err)
		return
	}
	label, err := h.activity.UpdateLabel(r.Context(), appactivity.UpdateLabelInput{
		ActorUserID: actorID(r), LabelID: id, Name: body.Name, Color: body.Color, Description: body.Description,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, mapProjectLabel(label))
}

func (h handler) deleteProjectLabel(w http.ResponseWriter, r *http.Request) {
	id, err := labelID(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if err := h.activity.DeleteLabel(r.Context(), appactivity.DeleteLabelInput{ActorUserID: actorID(r), LabelID: id}); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func labelID(r *http.Request) (int64, error) {
	value, err := strconv.ParseInt(chi.URLParam(r, "labelId"), 10, 64)
	if err != nil {
		return 0, apperror.Invalid("VALIDATION_FAILED", "label id is invalid", apperror.Field{Name: "path.labelId", Code: "INVALID_FORMAT", Message: "must be an integer"})
	}
	return value, nil
}
