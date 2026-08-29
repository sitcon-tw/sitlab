package oauth

import (
	"context"
	"time"

	"example.com/project-template/internal/domain/identity"
)

type Repository interface {
	StoreOAuthState(context.Context, identity.OAuthState) error
	ConsumeOAuthState(context.Context, []byte) (identity.OAuthState, error)
	UpsertUser(context.Context, identity.User) (identity.User, error)
	UpsertOAuthCredential(context.Context, identity.OAuthCredential) error
	ReplaceOAuthCredential(context.Context, identity.OAuthCredential, time.Time) error
	OAuthCredentialForUpdate(context.Context, string) (identity.OAuthCredential, error)
	GetUserByID(context.Context, string) (identity.User, error)
	CreateSession(context.Context, identity.Session) (identity.Session, error)
	GetSessionByTokenHash(context.Context, []byte) (identity.Session, error)
	TouchSession(context.Context, string, identity.Session) error
	DeleteSessionByTokenHash(context.Context, []byte) (string, bool, error)
	DeleteExpiredSession(context.Context, string) error
	HasActiveSessions(context.Context, string, time.Time) (bool, error)
	QueueAndDeleteOAuthCredential(context.Context, string, time.Time) error
	QueueOrphanedOAuthCredentials(context.Context, time.Time) error
	ClaimOAuthRevocation(context.Context, time.Time, time.Time) (identity.OAuthTokenRevocation, error)
	CompleteOAuthRevocation(context.Context, string) error
	RetryOAuthRevocation(context.Context, string, string, time.Time, time.Time) error
	OAuthRevocationQueueStats(context.Context, time.Time) (int64, float64, error)
}

type Transactor interface {
	WithinTx(context.Context, func(context.Context) error) error
}

type Tokens interface {
	New() (raw string, digest []byte, err error)
	Digest(raw string) []byte
	Matches(raw string, digest []byte) bool
	Derive(purpose, value string) string
	MatchesDerived(raw, purpose, value string) bool
}

type Cipher interface {
	Seal(string, string, string) ([]byte, error)
	Open(string, string, []byte) (string, bool, error)
}

type GitLab interface {
	AuthorizationURL(state, codeChallenge string) string
	ExchangeIdentity(context.Context, string, string) (GitLabIdentity, error)
	RefreshToken(context.Context, string) (OAuthTokens, error)
	RevokeToken(context.Context, string) error
}

type Observer interface {
	OAuthRefresh(string)
	OAuthRevocation(string)
	SetOAuthRevocationQueue(int64, float64)
}
