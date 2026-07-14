package identity

import "time"

type User struct {
	ID           string
	Email        string
	PasswordHash string
	DisplayName  string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Session struct {
	ID                string
	UserID            string
	TokenHash         []byte
	CSRFTokenHash     []byte
	IdleExpiresAt     time.Time
	AbsoluteExpiresAt time.Time
	CreatedAt         time.Time
	LastSeenAt        time.Time
}

type SessionClaims struct {
	SessionID string
	UserID    string
	ExpiresAt time.Time
}
