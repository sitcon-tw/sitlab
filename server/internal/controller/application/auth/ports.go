package auth

import (
	"context"

	"example.com/project-template/internal/domain/identity"
)

type Repository interface {
	CreateUser(context.Context, identity.User) (identity.User, error)
	GetUserByEmail(context.Context, string) (identity.User, error)
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

type PasswordHasher interface {
	Hash(string) (string, error)
	Compare(string, string) error
}

type Tokens interface {
	New() (raw string, digest []byte, err error)
	Digest(raw string) []byte
	Matches(raw string, digest []byte) bool
}
