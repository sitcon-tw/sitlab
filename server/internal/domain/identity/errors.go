package identity

import "errors"

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrEmailInUse      = errors.New("email already in use")
	ErrSessionNotFound = errors.New("session not found")
)
