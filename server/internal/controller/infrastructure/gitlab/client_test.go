package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"example.com/project-template/internal/controller/application/cardactivity"
	"example.com/project-template/internal/controller/application/sync"
	"example.com/project-template/internal/domain/board"
	"example.com/project-template/internal/domain/identity"
)

func TestSnapshotEndpointsParseMembersAndIssues(t *testing.T) {
	t.Parallel()
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodGet && request.Header.Get("PRIVATE-TOKEN") != "project-token" {
			t.Errorf("PRIVATE-TOKEN = %q", request.Header.Get("PRIVATE-TOKEN"))
		}
		switch {
		case strings.Contains(request.URL.Path, "/members/all"):
			return response(http.StatusOK, `[{"id":101,"username":"alice","name":"Alice","web_url":"https://gitlab.example/alice","access_level":40,"state":"active"}]`), nil
		case request.URL.Path == "/api/graphql":
			if contentType := request.Header.Get("Content-Type"); contentType != "application/json" {
				t.Errorf("GraphQL Content-Type = %q", contentType)
			}
			if accept := request.Header.Get("Accept"); accept != "application/json" {
				t.Errorf("GraphQL Accept = %q", accept)
			}
			var payload struct {
				Query     string         `json:"query"`
				Variables map[string]any `json:"variables"`
			}
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(payload.Query, "WorkItemMetadata") {
				return response(http.StatusOK, `{"data":{"project":{"labels":{"pageInfo":{"hasNextPage":false},"nodes":[{"id":"gid://gitlab/ProjectLabel/1","title":"Team::開發組"}]},"workItemTypes":{"nodes":[{"id":"gid://gitlab/WorkItems::Type/1","name":"Issue","widgetDefinitions":[{"allowedStatuses":[{"id":"status-1","name":"Waiting"},{"id":"status-2","name":"Inbox"},{"id":"status-3","name":"To do"},{"id":"status-4","name":"Doing"},{"id":"status-5","name":"Review"},{"id":"status-6","name":"Done"}]}]}]}}}}`), nil
			}
			if strings.Contains(payload.Query, "WorkItemLifecycle") {
				return response(http.StatusOK, `{"data":{"project":{"workItemTypes":{"nodes":[{"name":"Issue","widgetDefinitions":[{"allowedStatuses":[{"id":"status-1","name":"Waiting"},{"id":"status-2","name":"Inbox"},{"id":"status-3","name":"To do"},{"id":"status-4","name":"Doing"},{"id":"status-5","name":"Review"},{"id":"status-6","name":"Done"}]}]}]}}}}`), nil
			}
			if strings.Contains(payload.Query, "CreateWorkItem") {
				if request.Header.Get("Authorization") != "Bearer actor-token" {
					t.Errorf("Authorization = %q", request.Header.Get("Authorization"))
				}
				input := payload.Variables["input"].(map[string]any)
				if input["title"] != "[開發組] 新卡" || input["statusWidget"].(map[string]any)["name"] != "To do" {
					t.Errorf("work item input = %#v", input)
				}
				return response(http.StatusOK, `{"data":{"workItemCreate":{"errors":[],"workItem":`+testWorkItemJSON("2", "20", "[開發組] 新卡")+`}}}`), nil
			}
			if strings.Contains(payload.Query, "query WorkItems") || strings.Contains(payload.Query, "query WorkItem") {
				return response(http.StatusOK, `{"data":{"project":{"workItems":{"pageInfo":{"hasNextPage":false,"endCursor":null},"nodes":[`+testWorkItemJSON("1", "10", "[開發組] 修正流程")+`]}}}}`), nil
			}
			return response(http.StatusBadRequest, `{}`), nil
		default:
			return response(http.StatusNotFound, `{}`), nil
		}
	})
	client, err := New(&http.Client{Transport: transport}, Config{
		BaseURL: "https://gitlab.example", ProjectPath: "sitcon-tw/2027",
		AccessToken: "project-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	members, err := client.ProjectMembers(context.Background())
	if err != nil || len(members) != 1 || members[0].GitLabUserID != 101 {
		t.Fatalf("ProjectMembers() = %#v, %v", members, err)
	}
	issues, err := client.Issues(context.Background(), sync.IssueFilter{})
	if err != nil || len(issues) != 1 || len(issues[0].AssigneeGitLabUserIDs) != 2 || issues[0].Description != "工作拆解" || issues[0].StartDate != "2026-07-17" {
		t.Fatalf("Issues() = %#v, %v", issues, err)
	}
	issue, err := client.Issue(context.Background(), 1)
	if err != nil || issue.IssueIID != 1 || issue.Description != "工作拆解" {
		t.Fatalf("Issue() = %#v, %v", issue, err)
	}
	created, err := client.ApplyIssue(context.Background(), sync.IssueMutation{
		Create: true, Title: "[開發組] 新卡", Description: "詳細規劃", StartDate: "2026-07-17", DueDate: "2026-07-21",
		Labels: []string{"Team::開發組"}, AssigneeGitLabUserIDs: []int64{101, 202}, GitLabStatusName: "To do",
	}, "actor-token")
	if err != nil || created.IssueIID != 2 || created.StartDate != "2026-07-17" {
		t.Fatalf("ApplyIssue() = %#v, %v", created, err)
	}
}

func TestMissingIssueMapsToCardNotFound(t *testing.T) {
	t.Parallel()
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusOK, `{"data":{"project":{"workItems":{"nodes":[]}}}}`), nil
	})
	client, _ := New(&http.Client{Transport: transport}, Config{
		BaseURL: "https://gitlab.example", ProjectPath: "sitcon-tw/2027", AccessToken: "project-token",
	})
	_, err := client.Issue(context.Background(), 404)
	if !errors.Is(err, board.ErrCardNotFound) {
		t.Fatalf("Issue() error = %v", err)
	}
}

func TestInvisibleProjectIsAnErrorRatherThanAnEmptyBoard(t *testing.T) {
	t.Parallel()
	lifecycle := `{"data":{"project":{"workItemTypes":{"nodes":[{"name":"Issue","widgetDefinitions":[{"allowedStatuses":[{"id":"status-1","name":"Waiting"},{"id":"status-2","name":"Inbox"},{"id":"status-3","name":"To do"},{"id":"status-4","name":"Doing"},{"id":"status-5","name":"Review"},{"id":"status-6","name":"Done"}]}]}]}}}}`
	// GitLab answers a project-rooted query with a null project and no errors array
	// when the token cannot see the project. The lifecycle probe is served normally
	// here so the assertion does not depend on it happening to fail first.
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var payload struct {
			Query string `json:"query"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(payload.Query, "WorkItemLifecycle") {
			return response(http.StatusOK, lifecycle), nil
		}
		return response(http.StatusOK, `{"data":{"project":null}}`), nil
	})
	client, _ := New(&http.Client{Transport: transport}, Config{
		BaseURL: "https://gitlab.example", ProjectPath: "sitcon-tw/2027", AccessToken: "project-token",
	})

	issues, err := client.Issues(context.Background(), sync.IssueFilter{})
	if err == nil {
		t.Fatalf("Issues() = %#v, want an error so the snapshot merge never prunes a live board", issues)
	}
	if !strings.Contains(err.Error(), "sitcon-tw/2027") {
		t.Errorf("Issues() error = %v, want it to name the project", err)
	}
	if _, err := client.Issue(context.Background(), 1); err == nil {
		t.Fatal("Issue() = nil error, want an error")
	} else if errors.Is(err, board.ErrCardNotFound) {
		t.Errorf("Issue() error = %v, want a visibility error rather than a missing card", err)
	}
}

func TestLifecycleStatusesAreCachedAndDroppedOnAFailedValidation(t *testing.T) {
	t.Parallel()
	full := `{"data":{"project":{"workItemTypes":{"nodes":[{"name":"Issue","widgetDefinitions":[{"allowedStatuses":[{"id":"status-1","name":"Waiting"},{"id":"status-2","name":"Inbox"},{"id":"status-3","name":"To do"},{"id":"status-4","name":"Doing"},{"id":"status-5","name":"Review"},{"id":"status-6","name":"Done"}]}]}]}}}}`
	partial := `{"data":{"project":{"workItemTypes":{"nodes":[{"name":"Issue","widgetDefinitions":[{"allowedStatuses":[{"id":"status-1","name":"Waiting"}]}]}]}}}}`
	emptyBoard := `{"data":{"project":{"workItems":{"pageInfo":{"hasNextPage":false,"endCursor":null},"nodes":[]}}}}`

	lifecycleBody := full
	lifecycleCalls := 0
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var payload struct {
			Query string `json:"query"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(payload.Query, "WorkItemLifecycle") {
			lifecycleCalls++
			return response(http.StatusOK, lifecycleBody), nil
		}
		return response(http.StatusOK, emptyBoard), nil
	})
	client, _ := New(&http.Client{Transport: transport}, Config{
		BaseURL: "https://gitlab.example", ProjectPath: "sitcon-tw/2027", AccessToken: "project-token",
	})
	clock := time.Unix(1_750_000_000, 0).UTC()
	client.now = func() time.Time { return clock }

	for range 3 {
		if _, err := client.Issues(context.Background(), sync.IssueFilter{}); err != nil {
			t.Fatalf("Issues() error = %v", err)
		}
	}
	if lifecycleCalls != 1 {
		t.Fatalf("lifecycle requests = %d, want 1 across three polls inside the TTL", lifecycleCalls)
	}

	clock = clock.Add(lifecycleTTL)
	if _, err := client.Issues(context.Background(), sync.IssueFilter{}); err != nil {
		t.Fatalf("Issues() error = %v", err)
	}
	if lifecycleCalls != 2 {
		t.Fatalf("lifecycle requests = %d, want a refetch once the TTL elapsed", lifecycleCalls)
	}

	// A lifecycle missing a required status must not stay cached, so an administrator
	// who fixes it is picked up by the next poll instead of after the TTL.
	lifecycleBody = partial
	client.forgetLifecycle()
	if _, err := client.Issues(context.Background(), sync.IssueFilter{}); err == nil {
		t.Fatal("Issues() = nil error for a lifecycle missing required statuses")
	}
	before := lifecycleCalls
	lifecycleBody = full
	if _, err := client.Issues(context.Background(), sync.IssueFilter{}); err != nil {
		t.Fatalf("Issues() error = %v", err)
	}
	if lifecycleCalls != before+1 {
		t.Fatalf("lifecycle requests = %d, want %d: the failed lifecycle stayed cached", lifecycleCalls, before+1)
	}
}

func TestIssueReadsSendTheFilterGitLabExpects(t *testing.T) {
	t.Parallel()
	lifecycle := `{"data":{"project":{"workItemTypes":{"nodes":[{"name":"Issue","widgetDefinitions":[{"allowedStatuses":[{"id":"status-1","name":"Waiting"},{"id":"status-2","name":"Inbox"},{"id":"status-3","name":"To do"},{"id":"status-4","name":"Doing"},{"id":"status-5","name":"Review"},{"id":"status-6","name":"Done"}]}]}]}}}}`
	type sentRequest struct {
		query     string
		variables map[string]any
	}
	var seen []sentRequest
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var payload struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(payload.Query, "WorkItemLifecycle") {
			return response(http.StatusOK, lifecycle), nil
		}
		seen = append(seen, sentRequest{query: payload.Query, variables: payload.Variables})
		return response(http.StatusOK, `{"data":{"project":{"workItems":{"pageInfo":{"hasNextPage":false,"endCursor":null},"nodes":[]}}}}`), nil
	})
	client, _ := New(&http.Client{Transport: transport}, Config{
		BaseURL: "https://gitlab.example", ProjectPath: "sitcon-tw/2027", AccessToken: "project-token",
	})

	since := time.Date(2026, time.August, 18, 8, 30, 0, 0, time.UTC)
	if _, err := client.Issues(context.Background(), sync.IssueFilter{UpdatedAfter: &since, Order: sync.IssueOrderUpdatedAsc}); err != nil {
		t.Fatalf("Issues() error = %v", err)
	}
	if len(seen) != 1 {
		t.Fatalf("requests = %d, want 1", len(seen))
	}
	if !strings.Contains(seen[0].query, "sort: UPDATED_ASC") {
		t.Errorf("delta query did not sort UPDATED_ASC: %s", seen[0].query)
	}
	if got := seen[0].variables["updatedAfter"]; got != "2026-08-18T08:30:00Z" {
		t.Errorf("updatedAfter variable = %v, want RFC3339 2026-08-18T08:30:00Z", got)
	}

	seen = nil
	if _, err := client.IssueDigests(context.Background()); err != nil {
		t.Fatalf("IssueDigests() error = %v", err)
	}
	if len(seen) != 1 {
		t.Fatalf("digest requests = %d, want 1", len(seen))
	}
	// Widgets are what make a full-project read expensive; a presence sweep must not
	// pay for them.
	if strings.Contains(seen[0].query, "widgets") {
		t.Errorf("digest query selected widgets: %s", seen[0].query)
	}
	if !strings.Contains(seen[0].query, "sort: CREATED_ASC") {
		t.Errorf("digest query did not sort CREATED_ASC: %s", seen[0].query)
	}
	if _, sent := seen[0].variables["updatedAfter"]; sent {
		t.Error("digest query sent a lower bound; it must enumerate the whole project")
	}

	seen = nil
	if _, err := client.Issues(context.Background(), sync.IssueFilter{IIDs: []int64{7, 9}}); err != nil {
		t.Fatalf("Issues(by iid) error = %v", err)
	}
	if got := toStrings(t, seen[0].variables["iids"]); !slices.Equal(got, []string{"7", "9"}) {
		t.Errorf("iids variable = %v, want [7 9]", got)
	}

	// An explicitly empty set means "no issues", not "every issue".
	seen = nil
	if issues, err := client.Issues(context.Background(), sync.IssueFilter{IIDs: []int64{}}); err != nil || len(issues) != 0 {
		t.Fatalf("Issues(no iids) = %#v, %v", issues, err)
	}
	if len(seen) != 0 {
		t.Fatalf("an empty iid set still hit GitLab %d times", len(seen))
	}
}

func toStrings(t *testing.T, value any) []string {
	t.Helper()
	raw, ok := value.([]any)
	if !ok {
		t.Fatalf("value %#v is not a list", value)
	}
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		text, ok := item.(string)
		if !ok {
			t.Fatalf("item %#v is not a string", item)
		}
		result = append(result, text)
	}
	return result
}

func TestProjectLabelsAndCommentsUseExpectedCredentialsAndPagination(t *testing.T) {
	t.Parallel()
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case strings.HasSuffix(request.URL.Path, "/labels"):
			if request.Header.Get("PRIVATE-TOKEN") != "project-token" {
				t.Errorf("labels PRIVATE-TOKEN = %q", request.Header.Get("PRIVATE-TOKEN"))
			}
			return response(http.StatusOK, `[{"id":7,"name":"Backend","color":"#1D76DB","text_color":"#FFFFFF","description":"Server work"}]`), nil
		case request.Method == http.MethodGet && strings.HasSuffix(request.URL.Path, "/notes"):
			assertBearer(t, request)
			if request.URL.Query().Get("page") == "1" {
				result := response(http.StatusOK, `[{"id":2,"body":"changed status","system":true,"created_at":"2026-07-29T09:00:00Z","updated_at":"2026-07-29T09:00:00Z","author":{"id":101,"username":"alice","name":"Alice","web_url":"https://gitlab.example/alice"}}]`)
				result.Header.Set("X-Next-Page", "2")
				return result, nil
			}
			return response(http.StatusOK, `[{"id":3,"body":"please review","system":false,"created_at":"2026-07-29T10:00:00Z","updated_at":"2026-07-29T10:00:00Z","author":{"id":102,"username":"bob","name":"Bob","web_url":"https://gitlab.example/bob"}}]`), nil
		case request.Method == http.MethodPost && strings.HasSuffix(request.URL.Path, "/notes"):
			assertBearer(t, request)
			var payload map[string]string
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil || payload["body"] != "new comment" {
				t.Fatalf("comment payload = %#v, %v", payload, err)
			}
			return response(http.StatusCreated, `{"id":4,"body":"new comment","system":false,"created_at":"2026-07-29T11:00:00Z","updated_at":"2026-07-29T11:00:00Z","author":{"id":101,"username":"alice","name":"Alice","web_url":"https://gitlab.example/alice"}}`), nil
		default:
			return response(http.StatusNotFound, `{}`), nil
		}
	})
	client, _ := New(&http.Client{Transport: transport}, Config{
		BaseURL: "https://gitlab.example", ProjectPath: "sitcon-tw/2027", AccessToken: "project-token",
	})
	labels, err := client.ProjectLabels(context.Background())
	if err != nil || len(labels) != 1 || labels[0].ID != 7 || labels[0].Name != "Backend" || labels[0].TextColor != "#FFFFFF" {
		t.Fatalf("ProjectLabels() = %#v, %v", labels, err)
	}
	comments, err := client.Comments(context.Background(), 42, "token")
	if err != nil || len(comments) != 2 || !comments[0].System || comments[1].Author.DisplayName != "Bob" {
		t.Fatalf("Comments() = %#v, %v", comments, err)
	}
	created, err := client.CreateComment(context.Background(), 42, "new comment", "token")
	if err != nil || created.ID != 4 || created.Body != "new comment" {
		t.Fatalf("CreateComment() = %#v, %v", created, err)
	}
}

func TestCommentForbiddenMapsToActivityError(t *testing.T) {
	t.Parallel()
	client, _ := New(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusForbidden, `{}`), nil
	})}, Config{BaseURL: "https://gitlab.example", ProjectPath: "sitcon-tw/2027"})
	_, err := client.Comments(context.Background(), 42, "token")
	if !errors.Is(err, cardactivity.ErrGitLabForbidden) {
		t.Fatalf("Comments() error = %v", err)
	}
}

func TestOAuthAndProjectMembership(t *testing.T) {
	t.Parallel()
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/oauth/token":
			if err := request.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if request.FormValue("code_verifier") != "verifier" {
				t.Errorf("code_verifier = %q", request.FormValue("code_verifier"))
			}
			return response(http.StatusOK, `{"access_token":"token","refresh_token":"refresh","expires_in":7200,"created_at":10000}`), nil
		case "/api/v4/user":
			assertBearer(t, request)
			return response(http.StatusOK, `{"id":123,"username":"yorukot","name":"Yorukot","avatar_url":"https://img.example/avatar.png","web_url":"https://gitlab.com/yorukot"}`), nil
		case "/api/v4/projects/sitcon-tw/2027/members/all/123":
			if request.URL.EscapedPath() != "/api/v4/projects/sitcon-tw%2F2027/members/all/123" {
				t.Errorf("escaped project path = %q", request.URL.EscapedPath())
			}
			assertBearer(t, request)
			return response(http.StatusOK, `{"access_level":40,"state":"active"}`), nil
		default:
			return response(http.StatusNotFound, `{}`), nil
		}
	})
	client, err := New(&http.Client{Transport: transport}, Config{
		BaseURL: "https://gitlab.example", ClientID: "client", ClientSecret: "secret",
		RedirectURI: "https://board.example/callback", ProjectPath: "sitcon-tw/2027",
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.ExchangeIdentity(context.Background(), "code", "verifier")
	if err != nil {
		t.Fatalf("ExchangeIdentity() error = %v", err)
	}
	if result.GitLabUserID != 123 || result.AccessLevel != 40 || result.Username != "yorukot" {
		t.Fatalf("ExchangeIdentity() = %#v", result)
	}
	authorize, err := url.Parse(client.AuthorizationURL("state", "challenge"))
	if err != nil {
		t.Fatal(err)
	}
	if authorize.Query().Get("state") != "state" || authorize.Query().Get("code_challenge_method") != "S256" || authorize.Query().Get("scope") != "api" {
		t.Fatalf("AuthorizationURL() = %s", authorize)
	}
}

func TestMissingProjectMemberIsForbidden(t *testing.T) {
	t.Parallel()
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.Path == "/oauth/token":
			return response(http.StatusOK, `{"access_token":"token","refresh_token":"refresh","expires_in":7200}`), nil
		case request.URL.Path == "/api/v4/user":
			return response(http.StatusOK, `{"id":123,"username":"outside","name":"Outside"}`), nil
		case strings.Contains(request.URL.Path, "/members/all/"):
			return response(http.StatusNotFound, `{}`), nil
		default:
			return response(http.StatusNotFound, `{}`), nil
		}
	})
	client, _ := New(&http.Client{Transport: transport}, Config{BaseURL: "https://gitlab.example", ProjectPath: "sitcon-tw/2027"})
	_, err := client.ExchangeIdentity(context.Background(), "code", "verifier")
	if !errors.Is(err, identity.ErrProjectMemberRequired) {
		t.Fatalf("ExchangeIdentity() error = %v", err)
	}
}

func TestRefreshTokenRotatesOAuthCredential(t *testing.T) {
	t.Parallel()
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if err := request.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if request.FormValue("grant_type") != "refresh_token" || request.FormValue("refresh_token") != "old-refresh" {
			t.Fatalf("refresh form = %#v", request.Form)
		}
		return response(http.StatusOK, `{"access_token":"new-access","refresh_token":"new-refresh","expires_in":7200,"created_at":10000}`), nil
	})
	client, _ := New(&http.Client{Transport: transport}, Config{BaseURL: "https://gitlab.example", ClientID: "client", ClientSecret: "secret"})

	tokens, err := client.RefreshToken(context.Background(), "old-refresh")
	if err != nil || tokens.AccessToken != "new-access" || tokens.RefreshToken != "new-refresh" || !tokens.ExpiresAt.Equal(time.Unix(17_200, 0).UTC()) {
		t.Fatalf("RefreshToken() = %#v, %v", tokens, err)
	}
}

func assertBearer(t *testing.T, request *http.Request) {
	t.Helper()
	if request.Header.Get("Authorization") != "Bearer token" {
		t.Errorf("Authorization = %q", request.Header.Get("Authorization"))
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func testWorkItemJSON(iid, id, title string) string {
	value, _ := json.Marshal(map[string]any{
		"id":        "gid://gitlab/WorkItem/" + id,
		"iid":       iid,
		"title":     title,
		"state":     "OPEN",
		"webUrl":    "https://gitlab.example/issues/" + iid,
		"createdAt": "2026-07-13T08:00:00Z",
		"updatedAt": "2026-07-14T08:00:00Z",
		"widgets": []any{
			map[string]any{"type": "DESCRIPTION", "description": "工作拆解"},
			map[string]any{"type": "STATUS", "status": map[string]any{"id": "gid://gitlab/WorkItems::Statuses::Custom::Status/1513", "name": "To do", "category": "to_do", "color": "#ed9121"}},
			map[string]any{"type": "LABELS", "labels": map[string]any{"nodes": []any{map[string]any{"id": "gid://gitlab/ProjectLabel/1", "title": "Team::開發組"}}}},
			map[string]any{"type": "ASSIGNEES", "assignees": map[string]any{"nodes": []any{map[string]any{"id": "gid://gitlab/User/101"}, map[string]any{"id": "gid://gitlab/User/202"}}}},
			map[string]any{"type": "START_AND_DUE_DATE", "startDate": "2026-07-17", "dueDate": "2026-07-21"},
		},
	})
	return string(value)
}

func TestProjectLabelWritesUseRESTAndTheActorToken(t *testing.T) {
	t.Parallel()
	var seen struct {
		method  string
		path    string
		payload map[string]any
	}
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		// Label writes must carry the actor's bearer token, never the service
		// PRIVATE-TOKEN: GitLab's own project role is the authorization.
		if request.Header.Get("Authorization") != "Bearer actor-token" {
			t.Errorf("label write Authorization = %q", request.Header.Get("Authorization"))
		}
		if request.Header.Get("PRIVATE-TOKEN") != "" {
			t.Errorf("label write sent PRIVATE-TOKEN = %q", request.Header.Get("PRIVATE-TOKEN"))
		}
		seen.method, seen.path = request.Method, request.URL.Path
		if request.Body != nil && request.Method != http.MethodDelete {
			_ = json.NewDecoder(request.Body).Decode(&seen.payload)
		}
		if request.Method == http.MethodDelete {
			return response(http.StatusNoContent, ""), nil
		}
		return response(http.StatusOK, `{"id":10,"name":"feature","color":"#5843AD","text_color":"#FFFFFF","description":null}`), nil
	})
	client, _ := New(&http.Client{Transport: transport}, Config{
		BaseURL: "https://gitlab.example", ProjectPath: "sitcon-tw/2027", AccessToken: "project-token",
	})

	created, err := client.CreateProjectLabel(context.Background(), cardactivity.ProjectLabelWrite{Name: "feature", Color: "#5843AD"}, "actor-token")
	if err != nil || created.ID != 10 || created.TextColor != "#FFFFFF" {
		t.Fatalf("CreateProjectLabel() = %#v, %v", created, err)
	}
	if seen.method != http.MethodPost || !strings.HasSuffix(seen.path, "/labels") || seen.payload["name"] != "feature" {
		t.Fatalf("create call = %s %s %#v", seen.method, seen.path, seen.payload)
	}

	description := "renamed"
	if _, err := client.UpdateProjectLabel(context.Background(), 10, cardactivity.ProjectLabelWrite{Name: "epic", Color: "#5843AD", Description: &description}, "actor-token"); err != nil {
		t.Fatalf("UpdateProjectLabel() error = %v", err)
	}
	// GitLab renames through new_name, not name.
	if seen.method != http.MethodPut || !strings.HasSuffix(seen.path, "/labels/10") || seen.payload["new_name"] != "epic" || seen.payload["description"] != "renamed" {
		t.Fatalf("update call = %s %s %#v", seen.method, seen.path, seen.payload)
	}

	if err := client.DeleteProjectLabel(context.Background(), 10, "actor-token"); err != nil {
		t.Fatalf("DeleteProjectLabel() error = %v", err)
	}
	if seen.method != http.MethodDelete || !strings.HasSuffix(seen.path, "/labels/10") {
		t.Fatalf("delete call = %s %s", seen.method, seen.path)
	}
}

func TestProjectLabelWritesMapGitLabStatuses(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		status int
		want   error
	}{
		{http.StatusConflict, cardactivity.ErrLabelConflict},
		{http.StatusBadRequest, cardactivity.ErrLabelRejected},
		{http.StatusForbidden, cardactivity.ErrGitLabForbidden},
		{http.StatusNotFound, board.ErrCardNotFound},
	} {
		transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
			return response(testCase.status, `{}`), nil
		})
		client, _ := New(&http.Client{Transport: transport}, Config{BaseURL: "https://gitlab.example", ProjectPath: "sitcon-tw/2027"})
		_, err := client.CreateProjectLabel(context.Background(), cardactivity.ProjectLabelWrite{Name: "feature", Color: "#5843AD"}, "actor-token")
		if !errors.Is(err, testCase.want) {
			t.Fatalf("CreateProjectLabel() on %d = %v, want %v", testCase.status, err, testCase.want)
		}
	}
}

// A rename or delete landing while a card operation is pending would otherwise
// fail that operation permanently, because the worker replays the stale label
// names from the optimistic row and retryOperation re-reads the same names.
func TestApplyIssueDropsAVanishedLabelButNotAVanishedTeamLabel(t *testing.T) {
	t.Parallel()
	newClient := func(sentLabels *[]any) *Client {
		transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
			var payload struct {
				Query     string         `json:"query"`
				Variables map[string]any `json:"variables"`
			}
			_ = json.NewDecoder(request.Body).Decode(&payload)
			if strings.Contains(payload.Query, "WorkItemMetadata") {
				return response(http.StatusOK, `{"data":{"project":{"labels":{"pageInfo":{"hasNextPage":false},"nodes":[{"id":"gid://gitlab/ProjectLabel/1","title":"Team::開發組"}]},"workItemTypes":{"nodes":[{"id":"gid://gitlab/WorkItems::Type/1","name":"Issue","widgetDefinitions":[{"allowedStatuses":[{"id":"status-3","name":"To do"}]}]}]}}}}`), nil
			}
			if strings.Contains(payload.Query, "UpdateWorkItem") {
				input := payload.Variables["input"].(map[string]any)
				if widget, ok := input["labelsWidget"].(map[string]any); ok {
					if add, ok := widget["addLabelIds"].([]any); ok {
						*sentLabels = add
					}
				}
				return response(http.StatusOK, `{"data":{"workItemUpdate":{"errors":[],"workItem":`+testWorkItemJSON("1", "10", "[開發組] 修正流程")+`}}}`), nil
			}
			return response(http.StatusOK, `{"data":{"project":{"workItems":{"pageInfo":{"hasNextPage":false,"endCursor":null},"nodes":[`+testWorkItemJSON("1", "10", "[開發組] 修正流程")+`]}}}}`), nil
		})
		client, _ := New(&http.Client{Transport: transport}, Config{BaseURL: "https://gitlab.example", ProjectPath: "sitcon-tw/2027"})
		return client
	}

	var sent []any
	mutation := sync.IssueMutation{
		IssueIID: 10, Title: "[開發組] 修正流程", GitLabStatusName: "To do",
		Labels: []string{"Team::開發組", "Deleted::Label"},
	}
	if _, err := newClient(&sent).ApplyIssue(context.Background(), mutation, "actor-token"); err != nil {
		t.Fatalf("ApplyIssue() with a vanished ordinary label = %v, want it dropped", err)
	}

	missingTeam := sync.IssueMutation{
		IssueIID: 10, Title: "[新組] 修正流程", GitLabStatusName: "To do",
		Labels: []string{"Team::新組"},
	}
	if _, err := newClient(&sent).ApplyIssue(context.Background(), missingTeam, "actor-token"); err == nil {
		t.Fatal("ApplyIssue() with a vanished team label = nil, want an error")
	}
}
