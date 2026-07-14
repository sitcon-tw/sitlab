package task

import (
	"errors"
	"strings"
	"time"
)

type Status string

const (
	StatusTodo       Status = "todo"
	StatusInProgress Status = "in_progress"
	StatusDone       Status = "done"
)

var (
	ErrNotFound          = errors.New("task not found")
	ErrAssigneeNotMember = errors.New("task assignee is not a workspace member")
)

type Task struct {
	ID              string
	WorkspaceID     string
	Title           string
	Description     string
	Status          Status
	AssigneeUserID  *string
	CreatedByUserID string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func ParseStatus(value string) (Status, bool) {
	status := Status(strings.TrimSpace(value))
	switch status {
	case StatusTodo, StatusInProgress, StatusDone:
		return status, true
	default:
		return "", false
	}
}

func NormalizeTitle(value string) string { return strings.TrimSpace(value) }

func ValidTitle(value string) bool {
	n := len([]rune(value))
	return n >= 1 && n <= 160
}

func ValidDescription(value string) bool { return len([]rune(value)) <= 4000 }
