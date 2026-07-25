package oauth

import (
	"time"

	"example.com/project-template/internal/domain/identity"
)

type StartResult struct {
	AuthorizationURL string
}

type CompleteInput struct {
	Code  string
	State string
}

type Authenticated struct {
	User         identity.User
	SessionToken string
	RedirectPath string
}

type GitLabIdentity struct {
	GitLabUserID int64
	Username     string
	DisplayName  string
	AvatarURL    string
	ProfileURL   string
	AccessLevel  int32
	State        string
	Tokens       OAuthTokens
}

type OAuthTokens struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}
