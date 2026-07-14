package task

type CreateInput struct {
	ActorUserID    string
	WorkspaceID    string
	Title          string
	Description    string
	Status         string
	AssigneeUserID *string
}

type UpdateInput struct {
	ActorUserID    string
	WorkspaceID    string
	TaskID         string
	Title          *string
	Description    *string
	Status         *string
	AssigneeUserID **string
}
