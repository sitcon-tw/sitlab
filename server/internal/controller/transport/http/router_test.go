package httpserver

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"example.com/project-template/internal/controller/application/apperror"
	appboard "example.com/project-template/internal/controller/application/board"
	appbootstrap "example.com/project-template/internal/controller/application/bootstrap"
	appdirectory "example.com/project-template/internal/controller/application/directory"
	appoauth "example.com/project-template/internal/controller/application/oauth"
	"example.com/project-template/internal/domain/board"
	"example.com/project-template/internal/domain/directory"
	"example.com/project-template/internal/domain/identity"
	"go.uber.org/zap"
)

const httpUserID = "30000000-0000-0000-0000-000000000001"

var renewedExpiry = time.Date(2026, time.July, 28, 8, 0, 0, 0, time.UTC)

type authFake struct{}

func (authFake) Start(context.Context) (appoauth.StartResult, error) {
	return appoauth.StartResult{AuthorizationURL: "https://gitlab.example/oauth/authorize"}, nil
}
func (authFake) Complete(context.Context, appoauth.CompleteInput) (appoauth.Authenticated, error) {
	return appoauth.Authenticated{SessionToken: "new-session", RedirectPath: "/"}, nil
}
func (authFake) VerifySession(context.Context, string) (identity.SessionClaims, error) {
	return identity.SessionClaims{SessionID: "session-id", UserID: httpUserID, ExpiresAt: renewedExpiry}, nil
}
func (authFake) VerifyCSRFToken(_ context.Context, _, csrf string) (identity.SessionClaims, error) {
	if csrf != "valid-csrf" {
		return identity.SessionClaims{}, apperror.Forbidden("AUTH_INVALID_CSRF", "csrf token is invalid")
	}
	return identity.SessionClaims{SessionID: "session-id", UserID: httpUserID, ExpiresAt: renewedExpiry}, nil
}
func (authFake) IssueCSRF(context.Context, identity.SessionClaims) (string, error) {
	return "valid-csrf", nil
}
func (authFake) Logout(context.Context, string) error { return nil }
func (authFake) Me(context.Context, string) (identity.User, error) {
	return identity.User{
		ID: httpUserID, GitLabUserID: 101, Username: "alice", DisplayName: "Alice",
		ProfileURL: "https://gitlab.example/alice", AccessLevel: 40,
	}, nil
}

type directoryFake struct{}

func (directoryFake) Snapshot(context.Context) (directory.Snapshot, error) {
	return directory.Snapshot{
		Teams:          []directory.Team{{Key: "development", Name: "開發組", Active: true}},
		Members:        []directory.Member{{GitLabUserID: 101, Username: "alice", DisplayName: "Alice", State: directory.MemberActive}},
		SourceRevision: "revision-1", SyncedAt: time.Unix(1, 0),
	}, nil
}
func (directoryFake) Preferences(context.Context, string) (appdirectory.Preferences, error) {
	key := "development"
	return appdirectory.Preferences{DefaultTeamKey: &key}, nil
}
func (directoryFake) Update(_ context.Context, _ string, key string) (appdirectory.Preferences, error) {
	return appdirectory.Preferences{DefaultTeamKey: &key}, nil
}

type boardFake struct{}

func mutationResult(kind board.OperationKind) appboard.Result {
	return appboard.Result{
		Card:      board.Card{IssueIID: -1, Title: "修正流程", TeamKey: "development", ListKey: "todo", SyncState: board.OperationPending},
		Operation: board.Operation{ID: "10000000-0000-0000-0000-000000000001", Kind: kind, State: board.OperationPending},
	}
}
func (boardFake) Create(context.Context, appboard.CreateInput) (appboard.Result, error) {
	return mutationResult(board.OperationCreateCard), nil
}
func (boardFake) UpdateDetails(context.Context, appboard.UpdateDetailsInput) (appboard.Result, error) {
	return mutationResult(board.OperationUpdateDetails), nil
}
func (boardFake) UpdateTeam(context.Context, appboard.UpdateTeamInput) (appboard.Result, error) {
	return mutationResult(board.OperationUpdateTeam), nil
}
func (boardFake) UpdateAssignee(context.Context, appboard.UpdateAssigneeInput) (appboard.Result, error) {
	return mutationResult(board.OperationUpdateAssignee), nil
}
func (boardFake) UpdateStartDate(context.Context, appboard.UpdateStartDateInput) (appboard.Result, error) {
	return mutationResult(board.OperationUpdateStartDate), nil
}
func (boardFake) UpdateDueDate(context.Context, appboard.UpdateDueDateInput) (appboard.Result, error) {
	return mutationResult(board.OperationUpdateDueDate), nil
}
func (boardFake) Move(context.Context, appboard.MoveInput) (appboard.Result, error) {
	return mutationResult(board.OperationMoveCard), nil
}
func (boardFake) Retry(context.Context, string) (board.Operation, error) {
	return mutationResult(board.OperationMoveCard).Operation, nil
}

type bootstrapFake struct{}

func (bootstrapFake) Get(context.Context, identity.SessionClaims) (appbootstrap.Result, error) {
	key := "development"
	return appbootstrap.Result{
		Me:          identity.User{ID: httpUserID, GitLabUserID: 101, Username: "alice", DisplayName: "Alice", ProfileURL: "https://gitlab.example/alice", AccessLevel: 40},
		CSRFToken:   "valid-csrf",
		Directory:   directory.Snapshot{Teams: []directory.Team{{Key: key, Name: "開發組", Active: true}}},
		Board:       appboard.Snapshot{Lists: []board.List{{Key: "todo", Name: "待處理"}}, SyncedAt: time.Unix(1, 0)},
		Preferences: appdirectory.Preferences{DefaultTeamKey: &key},
		Sync:        appbootstrap.SyncStatus{State: "synced", LastSuccessAt: time.Unix(1, 0)},
	}, nil
}

type bootstrapFailureFake struct{}

func (bootstrapFailureFake) Get(context.Context, identity.SessionClaims) (appbootstrap.Result, error) {
	return appbootstrap.Result{}, errors.New("snapshot unavailable")
}

type syncFake struct{}

func (syncFake) RequestRefresh() time.Time { return time.Unix(2, 0) }

func testRouter(readiness func(context.Context) error, webDir string) http.Handler {
	return NewRouter(Dependencies{
		Log: zap.NewNop(), Auth: authFake{}, Bootstrap: bootstrapFake{},
		Directory: directoryFake{}, Board: boardFake{}, Sync: syncFake{},
		Cookie:         CookieConfig{Name: "test_session", TTL: 14 * 24 * time.Hour},
		AllowedOrigins: []string{"https://app.example.com"}, Readiness: readiness,
		APIName: "SITCON Board API", APIVersion: "9.8.7", WebDir: webDir,
	})
}

func TestGitLabOAuthIsPublicAndSetsFourteenDaySession(t *testing.T) {
	start := perform(testRouter(nil, ""), http.MethodGet, "/api/v1/auth/gitlab", "", false)
	if start.Code != http.StatusFound || start.Header().Get("Location") != "https://gitlab.example/oauth/authorize" {
		t.Fatalf("start = %d %s", start.Code, start.Header().Get("Location"))
	}
	callback := perform(testRouter(nil, ""), http.MethodGet, "/api/v1/auth/gitlab/callback?code=code&state=state", "", false)
	cookies := callback.Result().Cookies()
	if callback.Code != http.StatusFound || len(cookies) != 1 || cookies[0].MaxAge != 14*24*60*60 || !cookies[0].HttpOnly {
		t.Fatalf("callback = %d cookies=%#v", callback.Code, cookies)
	}
}

func TestAuthenticatedRequestRenewsCookieAndReturnsBootstrap(t *testing.T) {
	response := perform(testRouter(nil, ""), http.MethodGet, "/api/v1/bootstrap", "", true)
	cookies := response.Result().Cookies()
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"csrfToken":"valid-csrf"`) {
		t.Fatalf("response = %d %s", response.Code, response.Body.String())
	}
	if len(cookies) != 1 || !cookies[0].Expires.Equal(renewedExpiry) {
		t.Fatalf("rolling cookie = %#v", cookies)
	}
}

func TestCardMutationUsesAcceptedContractAndCSRF(t *testing.T) {
	response := perform(testRouter(nil, ""), http.MethodPost, "/api/v1/cards", `{"operationId":"10000000-0000-0000-0000-000000000001","title":"修正流程","description":"詳細規劃","teamKey":"development","assigneeGitLabUserIds":[101],"dueDate":"2026-07-21"}`, true)
	if response.Code != http.StatusAccepted || !strings.Contains(response.Body.String(), `"card"`) || !strings.Contains(response.Body.String(), `"operation"`) {
		t.Fatalf("response = %d %s", response.Code, response.Body.String())
	}
	forbidden := perform(testRouter(nil, ""), http.MethodPost, "/api/v1/cards", `{}`, false)
	if forbidden.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated mutation = %d", forbidden.Code)
	}
}

func TestUpdateCardStartDateUsesAcceptedContract(t *testing.T) {
	response := perform(testRouter(nil, ""), http.MethodPut, "/api/v1/cards/127/start-date", `{"operationId":"10000000-0000-0000-0000-000000000001","startDate":"2026-07-18"}`, true)
	if response.Code != http.StatusAccepted || !strings.Contains(response.Body.String(), `"kind":"update_start_date"`) {
		t.Fatalf("response = %d %s", response.Code, response.Body.String())
	}
}

func TestProductionHTMLInjectsBootstrapWithoutLoadingFetch(t *testing.T) {
	webDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(webDir, "index.html"), []byte("<html><head></head><body><div id=\"root\"></div></body></html>"), 0o600); err != nil {
		t.Fatal(err)
	}
	response := perform(testRouter(nil, webDir), http.MethodGet, "/", "", true)
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, `id="__SITCON_BOOTSTRAP__"`) || !strings.Contains(body, `"teams"`) {
		t.Fatalf("html = %d %s", response.Code, body)
	}
}

func TestProductionHTMLRenewsValidSessionWhenBootstrapIsUnavailable(t *testing.T) {
	webDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(webDir, "index.html"), []byte("<html><head></head><body></body></html>"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := spaHandler(webDir, authFake{}, bootstrapFailureFake{}, CookieConfig{Name: "test_session", TTL: 14 * 24 * time.Hour})
	response := perform(handler, http.MethodGet, "/", "", true)
	cookies := response.Result().Cookies()
	if response.Code != http.StatusOK || len(cookies) != 1 || !cookies[0].Expires.Equal(renewedExpiry) {
		t.Fatalf("rolling HTML session = %d %#v", response.Code, cookies)
	}
}

func TestReadinessRequiresSnapshots(t *testing.T) {
	router := testRouter(func(context.Context) error { return errors.New("snapshots missing") }, "")
	response := perform(router, http.MethodGet, "/api/v1/health/ready", "", false)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("readiness = %d %s", response.Code, response.Body.String())
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
