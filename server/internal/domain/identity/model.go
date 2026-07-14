package identity

import "time"

type User struct {
	ID           string
	Email        string
	PasswordHash string
	GitLabUserID int64
	Username     string
	DisplayName  string
	AvatarURL    string
	ProfileURL   string
	AccessLevel  int32
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type OAuthState struct {
	StateHash          []byte
	VerifierCiphertext []byte
	ReturnPath         string
	ExpiresAt          time.Time
	CreatedAt          time.Time
}

type Session struct {
	ID                string
	UserID            string
	TokenHash         []byte
	CSRFTokenHash     []byte
	ExpiresAt         time.Time
	LastUsedAt        time.Time
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
