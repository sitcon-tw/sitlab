package cardactivity

import "time"

type ProjectLabel struct {
	ID          int64
	Name        string
	Color       string
	TextColor   string
	Description *string
}

// ProjectLabelWrite is the mutable half of a label. TextColor is computed by
// GitLab and never sent.
type ProjectLabelWrite struct {
	Name        string
	Color       string
	Description *string
}

type CreateLabelInput struct {
	ActorUserID string
	Name        string
	Color       string
	Description *string
}

type UpdateLabelInput struct {
	ActorUserID string
	LabelID     int64
	Name        string
	Color       string
	Description *string
}

type DeleteLabelInput struct {
	ActorUserID string
	LabelID     int64
}

type CommentAuthor struct {
	GitLabUserID int64
	Username     string
	DisplayName  string
	AvatarURL    string
	ProfileURL   string
}

type Comment struct {
	ID        int64
	Body      string
	Author    CommentAuthor
	System    bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type CreateCommentInput struct {
	ActorUserID string
	IssueIID    int64
	Body        string
}
