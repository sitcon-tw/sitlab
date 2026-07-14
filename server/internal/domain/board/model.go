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
	OperationUpdateDetails  OperationKind = "update_details"
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
	ErrSnapshotNotFound    = errors.New("board snapshot not found")
	ErrOperationNotFound   = errors.New("operation not found")
	ErrOperationConflict   = errors.New("operation id is already used")
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
	IssueIID              int64
	GitLabIssueID         *int64
	Title                 string
	Description           string
	WebURL                string
	ListKey               string
	Position              int32
	TeamKey               string
	AssigneeGitLabUserIDs []int64
	DueDate               string
	Labels                []string
	SyncState             OperationState
	SyncError             string
	PendingOperationID    string
	CreatedAt             time.Time
	UpdatedAt             time.Time
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

type Snapshot struct {
	Lists    []List
	Cards    []Card
	SyncedAt time.Time
}

type Mutation struct {
	Card              Card
	Operation         Operation
	RequestedByUserID string
	Payload           map[string]any
}

type Result struct {
	Card      Card
	Operation Operation
}

type CanonicalIssue struct {
	IssueIID              int64
	GitLabIssueID         int64
	Title                 string
	Description           string
	WebURL                string
	Labels                []string
	AssigneeGitLabUserIDs []int64
	DueDate               string
	State                 string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type IssueMutation struct {
	Create                bool
	IssueIID              int64
	Title                 string
	Description           string
	Labels                []string
	AssigneeGitLabUserIDs []int64
	DueDate               string
	Closed                bool
}

type PendingOperation struct {
	Operation Operation
	Card      Card
}

type SyncStatus struct {
	State         string
	LastSuccessAt time.Time
	Message       string
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

func DefaultAssignees(selectedTeamKey, primaryTeamKey string, currentGitLabUserID int64) []int64 {
	if selectedTeamKey == "" || selectedTeamKey != primaryTeamKey || currentGitLabUserID <= 0 {
		return nil
	}
	return []int64{currentGitLabUserID}
}

func ReconcileAssignees(directory AssignmentDirectory, teamKey string, current []int64) ([]int64, bool, error) {
	if !directory.TeamExists(teamKey) {
		return current, false, ErrTeamNotFound
	}
	reconciled := make([]int64, 0, len(current))
	seen := make(map[int64]struct{}, len(current))
	for _, gitLabUserID := range current {
		if _, exists := seen[gitLabUserID]; exists {
			continue
		}
		seen[gitLabUserID] = struct{}{}
		if directory.IsAssignable(gitLabUserID) && directory.IsMemberOf(gitLabUserID, teamKey) {
			reconciled = append(reconciled, gitLabUserID)
		}
	}
	return reconciled, len(reconciled) != len(current), nil
}

func NormalizeAssigneeIDs(values []int64) []int64 {
	result := make([]int64, 0, len(values))
	seen := make(map[int64]struct{}, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func ValidateAssignees(directory AssignmentDirectory, gitLabUserIDs []int64) error {
	for _, gitLabUserID := range gitLabUserIDs {
		if !directory.IsAssignable(gitLabUserID) {
			return ErrMemberNotAssignable
		}
	}
	return nil
}

func DefaultDueDate(now time.Time) string {
	taipei := time.FixedZone("Asia/Taipei", 8*60*60)
	return now.In(taipei).AddDate(0, 0, 7).Format(time.DateOnly)
}
