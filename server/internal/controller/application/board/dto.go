package board

type CreateInput struct {
	OperationID          string
	ActorUserID          string
	Title                string
	TeamKey              string
	AssigneeGitLabUserID *int64
	DueDate              *string
}

type UpdateTeamInput struct {
	OperationID string
	ActorUserID string
	IssueIID    int64
	TeamKey     string
}

type UpdateAssigneeInput struct {
	OperationID          string
	ActorUserID          string
	IssueIID             int64
	AssigneeGitLabUserID *int64
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
