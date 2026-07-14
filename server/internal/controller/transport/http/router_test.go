package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"example.com/project-template/internal/controller/application/apperror"
	appauth "example.com/project-template/internal/controller/application/auth"
	apptask "example.com/project-template/internal/controller/application/task"
	appworkspace "example.com/project-template/internal/controller/application/workspace"
	"example.com/project-template/internal/domain/identity"
	domaintask "example.com/project-template/internal/domain/task"
	domainworkspace "example.com/project-template/internal/domain/workspace"
)

const (
	httpUserID      = "30000000-0000-0000-0000-000000000001"
	httpWorkspaceID = "30000000-0000-0000-0000-000000000002"
	httpTaskID      = "30000000-0000-0000-0000-000000000003"
)

type authFake struct{}

func (authFake) Register(_ context.Context, input appauth.RegisterInput) (appauth.Authenticated, error) {
	return appauth.Authenticated{User: identity.User{ID: httpUserID, Email: input.Email, DisplayName: input.DisplayName, CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(2, 0)}, SessionToken: "new-session"}, nil
}
func (authFake) Login(context.Context, appauth.LoginInput) (appauth.Authenticated, error) {
	return appauth.Authenticated{}, nil
}
func (authFake) VerifySession(context.Context, string) (identity.SessionClaims, error) {
	return identity.SessionClaims{SessionID: "session-id", UserID: httpUserID}, nil
}
func (authFake) VerifyCSRFToken(_ context.Context, _, csrf string) (identity.SessionClaims, error) {
	if csrf != "valid-csrf" {
		return identity.SessionClaims{}, apperror.Forbidden("AUTH_INVALID_CSRF", "csrf token is invalid")
	}
	return identity.SessionClaims{SessionID: "session-id", UserID: httpUserID}, nil
}
func (authFake) IssueCSRF(context.Context, identity.SessionClaims) (string, error) {
	return "valid-csrf", nil
}
func (authFake) Logout(context.Context, string) error { return nil }
func (authFake) Me(context.Context, string) (identity.User, error) {
	return identity.User{ID: httpUserID}, nil
}

type workspaceFake struct{}

func (workspaceFake) Create(_ context.Context, input appworkspace.CreateInput) (domainworkspace.Workspace, error) {
	if strings.TrimSpace(input.Name) == "" {
		return domainworkspace.Workspace{}, apperror.Invalid("VALIDATION_FAILED", "workspace input is invalid", apperror.Field{Name: "name", Code: "INVALID_LENGTH", Message: "must not be empty"})
	}
	return domainworkspace.Workspace{ID: httpWorkspaceID, Name: input.Name, Role: domainworkspace.RoleOwner, CreatedByUserID: input.ActorUserID}, nil
}
func (workspaceFake) List(context.Context, string) ([]domainworkspace.Workspace, error) {
	return nil, nil
}
func (workspaceFake) Get(context.Context, string, string) (domainworkspace.Workspace, error) {
	return domainworkspace.Workspace{ID: httpWorkspaceID, Name: "Engineering", Role: domainworkspace.RoleOwner}, nil
}
func (workspaceFake) Update(_ context.Context, input appworkspace.UpdateInput) (domainworkspace.Workspace, error) {
	if strings.TrimSpace(input.Name) == "" {
		return domainworkspace.Workspace{}, apperror.Invalid("VALIDATION_FAILED", "workspace input is invalid", apperror.Field{Name: "name", Code: "INVALID_LENGTH", Message: "must not be empty"})
	}
	return domainworkspace.Workspace{ID: input.WorkspaceID, Name: input.Name, Role: domainworkspace.RoleOwner}, nil
}
func (workspaceFake) Delete(context.Context, string, string) error { return nil }
func (workspaceFake) ListMembers(context.Context, string, string) ([]domainworkspace.Member, error) {
	return nil, nil
}
func (workspaceFake) AddMember(context.Context, appworkspace.AddMemberInput) (domainworkspace.Member, error) {
	return domainworkspace.Member{UserID: httpUserID, Email: "member@example.com", Role: domainworkspace.RoleViewer, JoinedAt: time.Unix(3, 0)}, nil
}
func (workspaceFake) UpdateMember(context.Context, appworkspace.UpdateMemberInput) (domainworkspace.Member, error) {
	return domainworkspace.Member{UserID: httpUserID, Role: domainworkspace.RoleEditor}, nil
}
func (workspaceFake) RemoveMember(context.Context, string, string, string) error { return nil }

type taskFake struct{}

func (taskFake) Create(_ context.Context, input apptask.CreateInput) (domaintask.Task, error) {
	return domaintask.Task{ID: httpTaskID, WorkspaceID: input.WorkspaceID, Title: input.Title, Description: input.Description, Status: domaintask.StatusTodo, AssigneeUserID: input.AssigneeUserID, CreatedByUserID: input.ActorUserID}, nil
}
func (taskFake) List(context.Context, string, string, string) ([]domaintask.Task, error) {
	return nil, nil
}
func (taskFake) Get(context.Context, string, string, string) (domaintask.Task, error) {
	return domaintask.Task{ID: httpTaskID, WorkspaceID: httpWorkspaceID, Title: "Review", Status: domaintask.StatusTodo, CreatedByUserID: httpUserID}, nil
}
func (taskFake) Update(context.Context, apptask.UpdateInput) (domaintask.Task, error) {
	return domaintask.Task{ID: httpTaskID, WorkspaceID: httpWorkspaceID, Title: "Updated", Status: domaintask.StatusDone}, nil
}
func (taskFake) Delete(context.Context, string, string, string) error { return nil }

func testRouter(readiness func(context.Context) error) http.Handler {
	return NewRouter(Dependencies{
		Log: zap.NewNop(), Auth: authFake{}, Workspaces: workspaceFake{}, Tasks: taskFake{},
		Cookie: CookieConfig{Name: "test_session", TTL: time.Hour}, AllowedOrigins: []string{"https://app.example.com"},
		Readiness: readiness, APIName: "Project Template API", APIVersion: "9.8.7",
	})
}

func TestRegisterIsPublicAndSetsStrictPersistentCookie(t *testing.T) {
	response := perform(testRouter(nil), http.MethodPost, "/api/v1/auth/register", `{"email":"user@example.com","password":"long-enough-password","displayName":"User"}`, false)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var body map[string]json.RawMessage
	_ = json.Unmarshal(response.Body.Bytes(), &body)
	if _, ok := body["user"]; !ok {
		t.Fatalf("user wrapper missing: %s", response.Body.String())
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].SameSite != http.SameSiteStrictMode || cookies[0].MaxAge <= 0 || cookies[0].Expires.IsZero() {
		t.Fatalf("unexpected cookie: %#v", cookies)
	}
}

func TestMutationResponsesUseNamedWrappers(t *testing.T) {
	router := testRouter(nil)
	tests := []struct {
		path string
		body string
		key  string
	}{
		{"/api/v1/workspaces", `{"name":"Engineering"}`, "workspace"},
		{"/api/v1/workspaces/" + httpWorkspaceID + "/members", `{"email":"member@example.com","role":"viewer"}`, "member"},
		{"/api/v1/workspaces/" + httpWorkspaceID + "/tasks", `{"title":"Review","assigneeId":"` + httpUserID + `"}`, "task"},
	}
	for _, tt := range tests {
		response := perform(router, http.MethodPost, tt.path, tt.body, true)
		if response.Code != http.StatusCreated {
			t.Fatalf("%s status = %d, body = %s", tt.path, response.Code, response.Body.String())
		}
		var body map[string]json.RawMessage
		_ = json.Unmarshal(response.Body.Bytes(), &body)
		if _, ok := body[tt.key]; !ok {
			t.Fatalf("%s wrapper missing in %s", tt.key, response.Body.String())
		}
	}
}

func TestTaskResponseUsesCanonicalFields(t *testing.T) {
	response := perform(testRouter(nil), http.MethodGet, "/api/v1/workspaces/"+httpWorkspaceID+"/tasks/"+httpTaskID, "", true)
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, `"task"`) || strings.Contains(body, "assigneeUserId") || strings.Contains(body, "createdByUserId") {
		t.Fatalf("unexpected task response: status=%d body=%s", response.Code, body)
	}
}

func TestProblemStatusAndLocationContract(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		path     string
		body     string
		auth     bool
		expected int
		location string
	}{
		{"malformed", http.MethodPost, "/api/v1/auth/register", `{`, false, http.StatusBadRequest, `"location":"body"`},
		{"semantic", http.MethodPost, "/api/v1/workspaces", `{"name":""}`, true, http.StatusUnprocessableEntity, `"location":"body.name"`},
		{"method", http.MethodPut, "/api/v1/auth/login", `{}`, false, http.StatusMethodNotAllowed, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := perform(testRouter(nil), tt.method, tt.path, tt.body, tt.auth)
			if response.Code != tt.expected || (tt.location != "" && !strings.Contains(response.Body.String(), tt.location)) {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestReadinessAndAPIRoot(t *testing.T) {
	router := testRouter(func(context.Context) error { return errors.New("database down") })
	readiness := perform(router, http.MethodGet, "/api/v1/healthz", "", false)
	if readiness.Code != http.StatusServiceUnavailable {
		t.Fatalf("readiness status = %d, body = %s", readiness.Code, readiness.Body.String())
	}
	root := perform(router, http.MethodGet, "/api/v1/", "", false)
	if root.Code != http.StatusOK || !strings.Contains(root.Body.String(), `"name":"Project Template API"`) || !strings.Contains(root.Body.String(), `"version":"9.8.7"`) {
		t.Fatalf("root response = %d %s", root.Code, root.Body.String())
	}
}

func perform(handler http.Handler, method, path, body string, authenticated bool) *httptest.ResponseRecorder {
	request := httptest.NewRequestWithContext(context.Background(), method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if authenticated {
		request.AddCookie(&http.Cookie{Name: "test_session", Value: "session"})
		request.Header.Set("X-CSRF-Token", "valid-csrf")
		request.Header.Set("Origin", "https://app.example.com")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
