package auth

import (
	"strings"

	"example.com/project-template/internal/domain/identity"
)

type RegisterInput struct {
	Email       string
	Password    string
	DisplayName string
}

type LoginInput struct {
	Email    string
	Password string
}

type Authenticated struct {
	User         identity.User
	SessionToken string
}

func normalizeEmail(value string) string { return strings.ToLower(strings.TrimSpace(value)) }
