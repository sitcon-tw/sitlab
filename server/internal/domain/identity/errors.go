package identity

import "errors"

var (
	ErrUserNotFound          = errors.New("user not found")
	ErrEmailInUse            = errors.New("email already in use")
	ErrSessionNotFound       = errors.New("session not found")
	ErrOAuthStateNotFound    = errors.New("oauth state not found")
	ErrGitLabUnavailable     = errors.New("GitLab is unavailable")
	ErrProjectMemberRequired = errors.New("active project membership is required")
)
