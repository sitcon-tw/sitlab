package httpserver

import (
	"encoding/json"
	"net/http"

	apptask "example.com/project-template/internal/controller/application/task"
)

type optionalNullableString struct {
	Set   bool
	Value *string
}

func (o *optionalNullableString) UnmarshalJSON(data []byte) error {
	o.Set = true
	if string(data) == "null" {
		o.Value = nil
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	o.Value = &value
	return nil
}

func (h handler) createTask(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title       string  `json:"title"`
		Description string  `json:"description"`
		Status      string  `json:"status"`
		AssigneeID  *string `json:"assigneeId"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, r, err)
		return
	}
	item, err := h.tasks.Create(r.Context(), apptask.CreateInput{ActorUserID: actorID(r), WorkspaceID: workspaceID(r), Title: body.Title, Description: body.Description, Status: body.Status, AssigneeUserID: body.AssigneeID})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"task": mapTask(item)})
}

func (h handler) listTasks(w http.ResponseWriter, r *http.Request) {
	items, err := h.tasks.List(r.Context(), workspaceID(r), actorID(r), r.URL.Query().Get("status"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	result := make([]taskResponse, 0, len(items))
	for _, item := range items {
		result = append(result, mapTask(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": result})
}

func (h handler) getTask(w http.ResponseWriter, r *http.Request) {
	item, err := h.tasks.Get(r.Context(), workspaceID(r), taskID(r), actorID(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"task": mapTask(item)})
}

func (h handler) updateTask(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title       *string                `json:"title"`
		Description *string                `json:"description"`
		Status      *string                `json:"status"`
		AssigneeID  optionalNullableString `json:"assigneeId"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, r, err)
		return
	}
	input := apptask.UpdateInput{ActorUserID: actorID(r), WorkspaceID: workspaceID(r), TaskID: taskID(r), Title: body.Title, Description: body.Description, Status: body.Status}
	if body.AssigneeID.Set {
		input.AssigneeUserID = &body.AssigneeID.Value
	}
	item, err := h.tasks.Update(r.Context(), input)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"task": mapTask(item)})
}

func (h handler) deleteTask(w http.ResponseWriter, r *http.Request) {
	if err := h.tasks.Delete(r.Context(), workspaceID(r), taskID(r), actorID(r)); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
