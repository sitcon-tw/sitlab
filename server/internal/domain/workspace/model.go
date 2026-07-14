package workspace

import (
	"errors"
	"strings"
	"time"
)

type Role string

const (
	RoleOwner  Role = "owner"
	RoleEditor Role = "editor"
	RoleViewer Role = "viewer"
)

var (
	ErrNotFound       = errors.New("workspace not found")
	ErrMemberNotFound = errors.New("workspace member not found")
	ErrMemberExists   = errors.New("workspace member already exists")
)

type Workspace struct {
	ID              string
	Name            string
	Role            Role
	CreatedByUserID string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Member struct {
	WorkspaceID string
	UserID      string
	Email       string
	DisplayName string
	Role        Role
	JoinedAt    time.Time
}

func ParseRole(value string) (Role, bool) {
	role := Role(strings.TrimSpace(value))
	switch role {
	case RoleOwner, RoleEditor, RoleViewer:
		return role, true
	default:
		return "", false
	}
}

func (r Role) CanRead() bool {
	return r == RoleOwner || r == RoleEditor || r == RoleViewer
}

func (r Role) CanWriteTasks() bool {
	return r == RoleOwner || r == RoleEditor
}

func (r Role) CanManageWorkspace() bool { return r == RoleOwner }

func NormalizeName(value string) string { return strings.TrimSpace(value) }

func ValidName(value string) bool {
	n := len([]rune(value))
	return n >= 1 && n <= 80
}
