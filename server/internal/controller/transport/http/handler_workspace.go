package httpserver

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	appworkspace "example.com/project-template/internal/controller/application/workspace"
)

func (h handler) createWorkspace(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, r, err)
		return
	}
	item, err := h.workspaces.Create(r.Context(), appworkspace.CreateInput{ActorUserID: actorID(r), Name: body.Name})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"workspace": mapWorkspace(item)})
}

func (h handler) listWorkspaces(w http.ResponseWriter, r *http.Request) {
	items, err := h.workspaces.List(r.Context(), actorID(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	result := make([]workspaceResponse, 0, len(items))
	for _, item := range items {
		result = append(result, mapWorkspace(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"workspaces": result})
}

func (h handler) getWorkspace(w http.ResponseWriter, r *http.Request) {
	item, err := h.workspaces.Get(r.Context(), workspaceID(r), actorID(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workspace": mapWorkspace(item)})
}

func (h handler) updateWorkspace(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, r, err)
		return
	}
	item, err := h.workspaces.Update(r.Context(), appworkspace.UpdateInput{ActorUserID: actorID(r), WorkspaceID: workspaceID(r), Name: body.Name})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workspace": mapWorkspace(item)})
}

func (h handler) deleteWorkspace(w http.ResponseWriter, r *http.Request) {
	if err := h.workspaces.Delete(r.Context(), workspaceID(r), actorID(r)); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h handler) listMembers(w http.ResponseWriter, r *http.Request) {
	items, err := h.workspaces.ListMembers(r.Context(), workspaceID(r), actorID(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	result := make([]memberResponse, 0, len(items))
	for _, item := range items {
		result = append(result, mapMember(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": result})
}

func (h handler) addMember(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, r, err)
		return
	}
	item, err := h.workspaces.AddMember(r.Context(), appworkspace.AddMemberInput{ActorUserID: actorID(r), WorkspaceID: workspaceID(r), Email: body.Email, Role: body.Role})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"member": mapMember(item)})
}

func (h handler) updateMember(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Role string `json:"role"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, r, err)
		return
	}
	item, err := h.workspaces.UpdateMember(r.Context(), appworkspace.UpdateMemberInput{ActorUserID: actorID(r), WorkspaceID: workspaceID(r), UserID: chi.URLParam(r, "userId"), Role: body.Role})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"member": mapMember(item)})
}

func (h handler) removeMember(w http.ResponseWriter, r *http.Request) {
	if err := h.workspaces.RemoveMember(r.Context(), workspaceID(r), chi.URLParam(r, "userId"), actorID(r)); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
