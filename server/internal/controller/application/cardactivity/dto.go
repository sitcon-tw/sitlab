package cardactivity

import "time"

type ProjectLabel struct {
	Name        string
	Color       string
	TextColor   string
	Description *string
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
