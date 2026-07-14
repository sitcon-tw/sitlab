package board

type CreateInput struct {
	OperationID           string
	ActorUserID           string
	Title                 string
	Description           string
	TeamKey               string
	AssigneeGitLabUserIDs []int64
	DueDate               *string
}

type UpdateDetailsInput struct {
	OperationID string
	ActorUserID string
	IssueIID    int64
	Title       string
	Description string
}

type UpdateTeamInput struct {
	OperationID string
	ActorUserID string
	IssueIID    int64
	TeamKey     string
}

type UpdateAssigneeInput struct {
	OperationID           string
	ActorUserID           string
	IssueIID              int64
	AssigneeGitLabUserIDs []int64
}

type UpdateDueDateInput struct {
	OperationID string
	ActorUserID string
	IssueIID    int64
	DueDate     *string
}

type MoveInput struct {
	OperationID string
	ActorUserID string
	IssueIID    int64
	ListKey     string
	Position    int32
}
