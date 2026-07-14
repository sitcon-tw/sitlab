package oauth

import (
	"context"

	"example.com/project-template/internal/domain/identity"
)

type Repository interface {
	StoreOAuthState(context.Context, identity.OAuthState) error
	ConsumeOAuthState(context.Context, []byte) (identity.OAuthState, error)
	UpsertUser(context.Context, identity.User) (identity.User, error)
	GetUserByID(context.Context, string) (identity.User, error)
	CreateSession(context.Context, identity.Session) (identity.Session, error)
	GetSessionByTokenHash(context.Context, []byte) (identity.Session, error)
	SetSessionCSRFHash(context.Context, string, []byte) error
	TouchSession(context.Context, string, identity.Session) error
	DeleteSessionByTokenHash(context.Context, []byte) error
	DeleteExpiredSession(context.Context, string) error
}

type Transactor interface {
	WithinTx(context.Context, func(context.Context) error) error
}

type Tokens interface {
	New() (raw string, digest []byte, err error)
	Digest(raw string) []byte
	Matches(raw string, digest []byte) bool
}

type Cipher interface {
	Seal(string) ([]byte, error)
	Open([]byte) (string, error)
}

type GitLab interface {
	AuthorizationURL(state, codeChallenge string) string
	ExchangeIdentity(context.Context, string, string) (GitLabIdentity, error)
}
