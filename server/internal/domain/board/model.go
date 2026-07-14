package board

import (
	"errors"
	"strings"
	"time"
)

type OperationState string
type OperationKind string

const (
	OperationPending    OperationState = "pending"
	OperationProcessing OperationState = "processing"
	OperationSynced     OperationState = "synced"
	OperationFailed     OperationState = "failed"

	OperationCreateCard     OperationKind = "create_card"
	OperationUpdateTeam     OperationKind = "update_team"
	OperationUpdateAssignee OperationKind = "update_assignee"
	OperationUpdateDueDate  OperationKind = "update_due_date"
	OperationMoveCard       OperationKind = "move_card"
)

var (
	ErrCardNotFound        = errors.New("card not found")
	ErrTeamNotFound        = errors.New("team not found")
	ErrMemberNotAssignable = errors.New("member is not assignable")
	ErrListNotFound        = errors.New("board list not found")
	ErrInvalidTitle        = errors.New("invalid card title")
)

type List struct {
	Key         string
	Name        string
	GitLabLabel string
	Position    int32
	Closed      bool
	Color       string
}

type Card struct {
	IssueIID             int64
	GitLabIssueID        *int64
	Title                string
	WebURL               string
	ListKey              string
	Position             int32
	TeamKey              string
	AssigneeGitLabUserID *int64
	DueDate              string
	Labels               []string
	SyncState            OperationState
	SyncError            string
	PendingOperationID   string
	UpdatedAt            time.Time
}

type Operation struct {
	ID        string
	Kind      OperationKind
	IssueIID  *int64
	State     OperationState
	Attempts  int32
	LastError string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type AssignmentDirectory interface {
	TeamExists(teamKey string) bool
	IsAssignable(gitLabUserID int64) bool
	IsMemberOf(gitLabUserID int64, teamKey string) bool
}

func NormalizeTitle(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func ValidTitle(value string) bool {
	length := len([]rune(NormalizeTitle(value)))
	return length >= 1 && length <= 255
}

func ComposeGitLabTitle(prefix, title string) string {
	prefix = strings.TrimSpace(prefix)
	title = NormalizeTitle(title)
	if prefix == "" || strings.HasPrefix(title, prefix) {
		return title
	}
	return prefix + " " + title
}

func DefaultAssignee(selectedTeamKey, primaryTeamKey string, currentGitLabUserID int64) *int64 {
	if selectedTeamKey == "" || selectedTeamKey != primaryTeamKey || currentGitLabUserID <= 0 {
		return nil
	}
	value := currentGitLabUserID
	return &value
}

func ReconcileAssignee(directory AssignmentDirectory, teamKey string, current *int64) (*int64, bool, error) {
	if !directory.TeamExists(teamKey) {
		return current, false, ErrTeamNotFound
	}
	if current == nil {
		return nil, false, nil
	}
	if directory.IsAssignable(*current) && directory.IsMemberOf(*current, teamKey) {
		return current, false, nil
	}
	return nil, true, nil
}

func ValidateAssignee(directory AssignmentDirectory, gitLabUserID *int64) error {
	if gitLabUserID == nil {
		return nil
	}
	if !directory.IsAssignable(*gitLabUserID) {
		return ErrMemberNotAssignable
	}
	return nil
}

func DefaultDueDate(now time.Time) string {
	taipei := time.FixedZone("Asia/Taipei", 8*60*60)
	return now.In(taipei).AddDate(0, 0, 7).Format(time.DateOnly)
}
