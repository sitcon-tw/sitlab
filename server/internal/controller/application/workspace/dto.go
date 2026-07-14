package workspace

type CreateInput struct {
	ActorUserID string
	Name        string
}

type UpdateInput struct {
	ActorUserID string
	WorkspaceID string
	Name        string
}

type AddMemberInput struct {
	ActorUserID string
	WorkspaceID string
	Email       string
	Role        string
}

type UpdateMemberInput struct {
	ActorUserID string
	WorkspaceID string
	UserID      string
	Role        string
}
