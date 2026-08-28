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
	appactivity "example.com/project-template/internal/controller/application/cardactivity"
	apprelation "example.com/project-template/internal/controller/application/cardrelation"
	appdirectory "example.com/project-template/internal/controller/application/directory"
	appoauth "example.com/project-template/internal/controller/application/oauth"
	appsync "example.com/project-template/internal/controller/application/sync"
	"example.com/project-template/internal/domain/board"
	"example.com/project-template/internal/domain/directory"
	"example.com/project-template/internal/domain/identity"
	"go.uber.org/zap"
)

const httpUserID = "30000000-0000-0000-0000-000000000001"

var renewedExpiry = time.Date(2026, time.July, 28, 8, 0, 0, 0, time.UTC)

type authFake struct{}

func (authFake) Start(context.Context) (appoauth.StartResult, error) {
	return appoauth.StartResult{
		AuthorizationURL: "https://gitlab.example/oauth/authorize",
		StateToken:       "browser-bound-state",
	}, nil
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
func (boardFake) UpdateLabels(context.Context, appboard.UpdateLabelsInput) (appboard.Result, error) {
	return mutationResult(board.OperationUpdateLabels), nil
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
		Revision:    "1",
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
func (syncFake) EnqueueWebhook(context.Context, board.WebhookDelivery) (bool, error) {
	return false, nil
}
func (syncFake) Delta(context.Context, appsync.DeltaQuery) (board.SyncDelta, error) {
	return board.SyncDelta{}, nil
}

type cardActivityFake struct{}

func (cardActivityFake) Labels(context.Context) ([]appactivity.ProjectLabel, error) {
	description := "Server work"
	return []appactivity.ProjectLabel{{ID: 7, Name: "Backend", Color: "#1D76DB", TextColor: "#FFFFFF", Description: &description}}, nil
}
func (cardActivityFake) CreateLabel(_ context.Context, input appactivity.CreateLabelInput) (appactivity.ProjectLabel, error) {
	return appactivity.ProjectLabel{ID: 99, Name: input.Name, Color: input.Color, TextColor: "#FFFFFF", Description: input.Description}, nil
}

func (cardActivityFake) UpdateLabel(_ context.Context, input appactivity.UpdateLabelInput) (appactivity.ProjectLabel, error) {
	return appactivity.ProjectLabel{ID: input.LabelID, Name: input.Name, Color: input.Color, TextColor: "#FFFFFF", Description: input.Description}, nil
}

func (cardActivityFake) DeleteLabel(context.Context, appactivity.DeleteLabelInput) error { return nil }
func (cardActivityFake) Comments(context.Context, string, int64) ([]appactivity.Comment, error) {
	return []appactivity.Comment{{
		ID: 7, Body: "changed status", System: true,
		Author:    appactivity.CommentAuthor{GitLabUserID: 101, Username: "alice", DisplayName: "Alice", ProfileURL: "https://gitlab.example/alice"},
		CreatedAt: time.Date(2026, 7, 29, 8, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 7, 29, 8, 0, 0, 0, time.UTC),
	}}, nil
}
func (cardActivityFake) CreateComment(_ context.Context, input appactivity.CreateCommentInput) (appactivity.Comment, error) {
	return appactivity.Comment{
		ID: 8, Body: input.Body,
		Author:    appactivity.CommentAuthor{GitLabUserID: 101, Username: "alice", DisplayName: "Alice", ProfileURL: "https://gitlab.example/alice"},
		CreatedAt: time.Date(2026, 7, 29, 9, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 7, 29, 9, 0, 0, 0, time.UTC),
	}, nil
}

type cardRelationFake struct{}

func (cardRelationFake) ChildItems(context.Context, string, int64, apprelation.PageQuery) (apprelation.ChildPage, error) {
	return apprelation.ChildPage{Items: []apprelation.WorkItem{relationshipHTTPItem()}, TotalCount: 1}, nil
}

func (cardRelationFake) LinkedItems(context.Context, string, int64, apprelation.PageQuery) (apprelation.LinkedPage, error) {
	return apprelation.LinkedPage{
		Items: []apprelation.LinkedItem{{WorkItem: relationshipHTTPItem(), LinkType: apprelation.LinkTypeBlocks}}, TotalCount: 1,
	}, nil
}

func (cardRelationFake) Search(context.Context, apprelation.SearchInput) ([]apprelation.WorkItem, error) {
	return []apprelation.WorkItem{relationshipHTTPItem()}, nil
}

func (cardRelationFake) CreateChild(context.Context, apprelation.CreateChildInput) (apprelation.WorkItem, error) {
	return relationshipHTTPItem(), nil
}

func (cardRelationFake) AttachChild(context.Context, apprelation.ChildRelationInput) error {
	return nil
}
func (cardRelationFake) DetachChild(context.Context, apprelation.ChildRelationInput) error {
	return nil
}
func (cardRelationFake) AddLinks(context.Context, apprelation.LinkInput) error            { return nil }
func (cardRelationFake) RemoveLink(context.Context, apprelation.ChildRelationInput) error { return nil }

func relationshipHTTPItem() apprelation.WorkItem {
	return apprelation.WorkItem{
		GitLabWorkItemID: 9201, IID: 201, Type: apprelation.WorkItemTypeTask,
		Title: "Add metrics", State: apprelation.WorkItemStateOpen, WebURL: "https://gitlab.example/work_items/201",
		Status: &apprelation.Status{Name: "To do", Category: "to_do"},
		Assignees: []apprelation.Assignee{{
			GitLabUserID: 101, Username: "alice", DisplayName: "Alice", ProfileURL: "https://gitlab.example/alice",
		}},
		Labels: []apprelation.Label{{Name: "Backend", Color: "#1D76DB", TextColor: "#FFFFFF"}},
	}
}

func testRouter(readiness func(context.Context) error, webDir string) http.Handler {
	return NewRouter(Dependencies{
		Log: zap.NewNop(), Auth: authFake{}, Bootstrap: bootstrapFake{},
		Directory: directoryFake{}, Board: boardFake{}, CardActivity: cardActivityFake{}, CardRelations: cardRelationFake{}, Sync: syncFake{},
		Cookie: CookieConfig{
			Name: "test_session", TTL: 14 * 24 * time.Hour, OAuthStateTTL: 10 * time.Minute,
		},
		AllowedOrigins: []string{"https://app.example.com"}, Readiness: readiness,
		APIName: "SITCON Board API", APIVersion: "9.8.7", WebDir: webDir,
	})
}

func TestGitLabOAuthIsPublicAndSetsFourteenDaySession(t *testing.T) {
	router := testRouter(nil, "")
	start := perform(router, http.MethodGet, "/api/v1/auth/gitlab", "", false)
	if start.Code != http.StatusFound || start.Header().Get("Location") != "https://gitlab.example/oauth/authorize" {
		t.Fatalf("start = %d %s", start.Code, start.Header().Get("Location"))
	}
	stateCookie := cookieByName(start.Result().Cookies(), "test_session_oauth_state")
	if stateCookie == nil || stateCookie.Value != "browser-bound-state" || !stateCookie.HttpOnly ||
		stateCookie.SameSite != http.SameSiteLaxMode || stateCookie.MaxAge != 10*60 {
		t.Fatalf("OAuth state cookie = %#v", stateCookie)
	}
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet,
		"/api/v1/auth/gitlab/callback?code=code&state=browser-bound-state", nil)
	request.AddCookie(stateCookie)
	callback := httptest.NewRecorder()
	router.ServeHTTP(callback, request)
	cookies := callback.Result().Cookies()
	sessionCookie := cookieByName(cookies, "test_session")
	clearedState := cookieByName(cookies, "test_session_oauth_state")
	if callback.Code != http.StatusFound || sessionCookie == nil || sessionCookie.MaxAge != 14*24*60*60 || !sessionCookie.HttpOnly ||
		clearedState == nil || clearedState.MaxAge != -1 {
		t.Fatalf("callback = %d cookies=%#v", callback.Code, cookies)
	}
}

func TestGitLabOAuthCallbackRejectsStateFromAnotherBrowser(t *testing.T) {
	response := perform(testRouter(nil, ""), http.MethodGet,
		"/api/v1/auth/gitlab/callback?code=code&state=attacker-state", "", false)
	if response.Code != http.StatusUnauthorized || !strings.Contains(response.Body.String(), `"code":"AUTH_OAUTH_FAILED"`) ||
		cookieByName(response.Result().Cookies(), "test_session") != nil {
		t.Fatalf("callback = %d body=%s cookies=%#v", response.Code, response.Body.String(), response.Result().Cookies())
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
	response := perform(testRouter(nil, ""), http.MethodPost, "/api/v1/cards", `{"operationId":"10000000-0000-0000-0000-000000000001","title":"修正流程","description":"詳細規劃","teamKey":"development","listKey":"inbox","assigneeGitLabUserIds":[101],"labels":["Backend"],"startDate":null,"dueDate":"2026-07-21"}`, true)
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

func TestCardLabelsAndCommentsMatchContract(t *testing.T) {
	labels := perform(testRouter(nil, ""), http.MethodGet, "/api/v1/labels", "", true)
	if labels.Code != http.StatusOK || !strings.Contains(labels.Body.String(), `"id":7`) || !strings.Contains(labels.Body.String(), `"name":"Backend"`) || !strings.Contains(labels.Body.String(), `"textColor":"#FFFFFF"`) {
		t.Fatalf("labels = %d %s", labels.Code, labels.Body.String())
	}
	updated := perform(testRouter(nil, ""), http.MethodPut, "/api/v1/cards/127/labels", `{"operationId":"10000000-0000-0000-0000-000000000001","labels":["Backend"]}`, true)
	if updated.Code != http.StatusAccepted || !strings.Contains(updated.Body.String(), `"kind":"update_labels"`) {
		t.Fatalf("update labels = %d %s", updated.Code, updated.Body.String())
	}
	comments := perform(testRouter(nil, ""), http.MethodGet, "/api/v1/cards/127/comments", "", true)
	if comments.Code != http.StatusOK || !strings.Contains(comments.Body.String(), `"system":true`) {
		t.Fatalf("comments = %d %s", comments.Code, comments.Body.String())
	}
	created := perform(testRouter(nil, ""), http.MethodPost, "/api/v1/cards/127/comments", `{"body":"please review"}`, true)
	if created.Code != http.StatusCreated || !strings.Contains(created.Body.String(), `"body":"please review"`) {
		t.Fatalf("create comment = %d %s", created.Code, created.Body.String())
	}
}

func TestCardRelationshipsMatchContract(t *testing.T) {
	children := perform(testRouter(nil, ""), http.MethodGet, "/api/v1/cards/127/child-items?limit=50", "", true)
	if children.Code != http.StatusOK || !strings.Contains(children.Body.String(), `"gitLabWorkItemId":9201`) || !strings.Contains(children.Body.String(), `"totalCount":1`) {
		t.Fatalf("children = %d %s", children.Code, children.Body.String())
	}
	links := perform(testRouter(nil, ""), http.MethodGet, "/api/v1/cards/127/linked-items", "", true)
	if links.Code != http.StatusOK || !strings.Contains(links.Body.String(), `"linkType":"blocks"`) {
		t.Fatalf("links = %d %s", links.Code, links.Body.String())
	}
	candidates := perform(testRouter(nil, ""), http.MethodGet, "/api/v1/cards/127/relationship-candidates?kind=child&query=%23201", "", true)
	if candidates.Code != http.StatusOK || !strings.Contains(candidates.Body.String(), `"iid":201`) {
		t.Fatalf("candidates = %d %s", candidates.Code, candidates.Body.String())
	}
	created := perform(testRouter(nil, ""), http.MethodPost, "/api/v1/cards/127/child-items", `{"title":"Add metrics"}`, true)
	if created.Code != http.StatusCreated || !strings.Contains(created.Body.String(), `"type":"task"`) {
		t.Fatalf("create child = %d %s", created.Code, created.Body.String())
	}
	for _, mutation := range []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodPut, path: "/api/v1/cards/127/child-items/9201"},
		{method: http.MethodDelete, path: "/api/v1/cards/127/child-items/9201"},
		{method: http.MethodPost, path: "/api/v1/cards/127/linked-items", body: `{"workItemIds":[9201,9202],"linkType":"blocks"}`},
		{method: http.MethodDelete, path: "/api/v1/cards/127/linked-items/9201"},
	} {
		response := perform(testRouter(nil, ""), mutation.method, mutation.path, mutation.body, true)
		if response.Code != http.StatusNoContent {
			t.Fatalf("%s %s = %d %s", mutation.method, mutation.path, response.Code, response.Body.String())
		}
	}
	malformed := perform(testRouter(nil, ""), http.MethodDelete, "/api/v1/cards/127/linked-items/nope", "", true)
	if malformed.Code != http.StatusUnprocessableEntity {
		t.Fatalf("malformed work item id = %d %s", malformed.Code, malformed.Body.String())
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

func cookieByName(cookies []*http.Cookie, name string) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

func TestProjectLabelManagementMatchesContract(t *testing.T) {
	created := perform(testRouter(nil, ""), http.MethodPost, "/api/v1/labels", `{"name":"Priority::High","color":"#D73A4A","description":null}`, true)
	if created.Code != http.StatusCreated || !strings.Contains(created.Body.String(), `"name":"Priority::High"`) {
		t.Fatalf("create label = %d %s", created.Code, created.Body.String())
	}
	updated := perform(testRouter(nil, ""), http.MethodPut, "/api/v1/labels/7", `{"name":"Priority::Urgent","color":"#B60205","description":null}`, true)
	if updated.Code != http.StatusOK || !strings.Contains(updated.Body.String(), `"id":7`) {
		t.Fatalf("update label = %d %s", updated.Code, updated.Body.String())
	}
	deleted := perform(testRouter(nil, ""), http.MethodDelete, "/api/v1/labels/7", "", true)
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete label = %d %s", deleted.Code, deleted.Body.String())
	}
	// A non-integer id is rejected before the service runs.
	malformed := perform(testRouter(nil, ""), http.MethodDelete, "/api/v1/labels/abc", "", true)
	if malformed.Code != http.StatusUnprocessableEntity {
		t.Fatalf("delete label with a bad id = %d %s", malformed.Code, malformed.Body.String())
	}
}

func TestProjectLabelWritesRequireSessionAndCSRF(t *testing.T) {
	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/v1/labels", `{"name":"Backend","color":"#1D76DB","description":null}`},
		{http.MethodPut, "/api/v1/labels/7", `{"name":"Backend","color":"#1D76DB","description":null}`},
		{http.MethodDelete, "/api/v1/labels/7", ""},
	}
	for _, testCase := range cases {
		unauthenticated := perform(testRouter(nil, ""), testCase.method, testCase.path, testCase.body, false)
		if unauthenticated.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s unauthenticated = %d %s", testCase.method, testCase.path, unauthenticated.Code, unauthenticated.Body.String())
		}

		// A real session with the wrong CSRF token must still be rejected.
		request := httptest.NewRequestWithContext(context.Background(), testCase.method, testCase.path, strings.NewReader(testCase.body))
		request.Header.Set("Content-Type", "application/json")
		request.AddCookie(&http.Cookie{Name: "test_session", Value: "session"})
		request.Header.Set("X-CSRF-Token", "stale-csrf")
		request.Header.Set("Origin", "https://app.example.com")
		response := httptest.NewRecorder()
		testRouter(nil, "").ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("%s %s with a stale CSRF token = %d %s", testCase.method, testCase.path, response.Code, response.Body.String())
		}
	}
}
