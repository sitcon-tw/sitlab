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

	OperationCreateCard      OperationKind = "create_card"
	OperationUpdateDetails   OperationKind = "update_details"
	OperationUpdateTeam      OperationKind = "update_team"
	OperationUpdateAssignee  OperationKind = "update_assignee"
	OperationUpdateStartDate OperationKind = "update_start_date"
	OperationUpdateDueDate   OperationKind = "update_due_date"
	OperationUpdateLabels    OperationKind = "update_labels"
	OperationMoveCard        OperationKind = "move_card"
)

// ErrCheckpointTooOld means a client cannot be caught up incrementally and has to
// start again from a full bootstrap: its checkpoint has been pruned, is further ahead
// than the server (a restored backup), or is so far behind that replaying costs more
// than resending everything.
var ErrCheckpointTooOld = errors.New("sync checkpoint is too old")

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
	Key              string
	Name             string
	GitLabStatusName string
	Position         int32
	Closed           bool
	Color            string
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
	StartDate             string
	DueDate               string
	Labels                []string
	GitLabStatusName      string
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

// BoardObservation is one full-board read of GitLab, handed to the cache as a unit.
//
// StartedAt and SyncedAt are deliberately separate and come from different clocks.
// StartedAt is PostgreSQL's clock read before the first GitLab page, and pruning
// compares it against issue_cache.gitlab_observed_at so both sides of that decision
// sit in the same clock domain -- a card a webhook reconciled while this read was in
// flight is stamped later than StartedAt and so survives. SyncedAt is the app's wall
// clock and only stamps updated_at.
type BoardObservation struct {
	Cards []Card
	// Retained are issues GitLab reported but that this read did not carry a card for:
	// either it could not be placed on a lane, or a presence read confirmed it exists
	// without paying for its content. They are neither written nor pruned.
	Retained []int64
	// Removed are issues this read positively determined are no longer board cards.
	// An incremental read has to say so explicitly: omitting an issue there means
	// "unchanged", not "gone", so an issue that lost its Team:: label would otherwise
	// linger until the next full sweep.
	Removed []int64
	// Complete marks a read that enumerated the whole project. Only a complete read
	// may prune, because a partial one cannot tell "deleted" from "not mentioned".
	Complete bool
	// Watermark advances the incremental cursor to the newest GitLab timestamp this
	// read observed. Nil leaves the cursor alone, which is what a sweep wants: it
	// enumerates the project without establishing a new incremental boundary.
	Watermark *time.Time
	StartedAt time.Time
	SyncedAt  time.Time
}

// SyncAction is one recorded change, replayed to clients that are behind.
type SyncAction struct {
	SyncID    string
	Seq       int32
	Entity    string
	EntityID  string
	Operation string
	// ActorGitLabUserID identifies who caused the change, where a person did.
	ActorGitLabUserID *int64
	// Payload is the domain value as JSON. Transport maps it to the wire shape; this
	// type stays free of HTTP concerns.
	Payload    []byte
	OccurredAt time.Time
}

// SyncDelta is one page of the change log.
type SyncDelta struct {
	Checkpoint string
	Actions    []SyncAction
	HasMore    bool
}

// SyncCursor is where the incremental board read left off.
type SyncCursor struct {
	// Watermark is GitLab's own newest updated_at that a successful read has observed.
	// Nil means nothing has been read yet and the next read must be a full sweep.
	Watermark *time.Time
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
	StartDate             string
	DueDate               string
	State                 string
	GitLabStatusName      string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// IssueDigest is the cheap half of a GitLab issue: enough to tell whether the cache is
// current without paying for description, labels, assignees and status widgets. A
// presence sweep reads these for the whole project and refetches only what drifted.
type IssueDigest struct {
	IssueIID  int64
	UpdatedAt time.Time
}

type IssueMutation struct {
	Create                bool
	IssueIID              int64
	Title                 string
	Description           string
	Labels                []string
	AssigneeGitLabUserIDs []int64
	StartDate             string
	DueDate               string
	GitLabStatusName      string
}

type PendingOperation struct {
	Operation         Operation
	Card              Card
	RequestedByUserID string
}

type SyncStatus struct {
	State         string
	LastSuccessAt time.Time
	Message       string
}

type WebhookDelivery struct {
	ID          string
	Scope       string
	EventKind   string
	EventName   string
	IssueIID    *int64
	State       string
	Attempts    int32
	LastError   string
	AvailableAt time.Time
	ReceivedAt  time.Time
	UpdatedAt   time.Time
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
