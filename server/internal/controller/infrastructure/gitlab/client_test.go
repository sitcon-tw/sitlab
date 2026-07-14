package gitlab

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"example.com/project-template/internal/domain/identity"
)

func TestSnapshotEndpointsParseDirectoryMembersAndIssues(t *testing.T) {
	t.Parallel()
	directoryYAML := "version: 1\nteams:\n  - key: development\n    name: 開發組\n    title_prefix: '[開發組]'\n    gitlab_label: '組別::開發'\n    active: true\n    members: [alice]\n"
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("PRIVATE-TOKEN") != "project-token" {
			t.Errorf("PRIVATE-TOKEN = %q", request.Header.Get("PRIVATE-TOKEN"))
		}
		switch {
		case request.Method == http.MethodHead && strings.Contains(request.URL.Path, "/repository/files/"):
			result := response(http.StatusOK, "")
			result.Header.Set("X-Gitlab-Last-Commit-Id", "revision-1")
			return result, nil
		case request.Method == http.MethodGet && strings.Contains(request.URL.Path, "/repository/files/"):
			return response(http.StatusOK, `{"content":"`+base64.StdEncoding.EncodeToString([]byte(directoryYAML))+`","last_commit_id":"revision-1"}`), nil
		case strings.Contains(request.URL.Path, "/members/all"):
			return response(http.StatusOK, `[{"id":101,"username":"alice","name":"Alice","web_url":"https://gitlab.example/alice","access_level":40,"state":"active"}]`), nil
		case strings.Contains(request.URL.Path, "/issues"):
			return response(http.StatusOK, `[{"id":10,"iid":1,"title":"[開發組] 修正流程","web_url":"https://gitlab.example/issues/1","labels":["組別::開發","Todo"],"due_date":"2026-07-21","state":"opened","updated_at":"2026-07-14T08:00:00Z","assignee":{"id":101}}]`), nil
		default:
			return response(http.StatusNotFound, `{}`), nil
		}
	})
	client, err := New(&http.Client{Transport: transport}, Config{
		BaseURL: "https://gitlab.example", ProjectPath: "sitcon-tw/2027",
		AccessToken: "project-token", Branch: "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	revision, err := client.DirectoryRevision(context.Background())
	if err != nil || revision != "revision-1" {
		t.Fatalf("DirectoryRevision() = %q, %v", revision, err)
	}
	file, revision, err := client.DirectoryFile(context.Background())
	if err != nil || revision != "revision-1" || len(file.Teams) != 1 || file.Teams[0].Members[0] != "alice" {
		t.Fatalf("DirectoryFile() = %#v, %q, %v", file, revision, err)
	}
	members, err := client.ProjectMembers(context.Background())
	if err != nil || len(members) != 1 || members[0].GitLabUserID != 101 {
		t.Fatalf("ProjectMembers() = %#v, %v", members, err)
	}
	issues, err := client.Issues(context.Background())
	if err != nil || len(issues) != 1 || issues[0].AssigneeGitLabUserID == nil || *issues[0].AssigneeGitLabUserID != 101 {
		t.Fatalf("Issues() = %#v, %v", issues, err)
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
			return response(http.StatusOK, `{"access_token":"token"}`), nil
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
	if authorize.Query().Get("state") != "state" || authorize.Query().Get("code_challenge_method") != "S256" {
		t.Fatalf("AuthorizationURL() = %s", authorize)
	}
}

func TestMissingProjectMemberIsForbidden(t *testing.T) {
	t.Parallel()
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.Path == "/oauth/token":
			return response(http.StatusOK, `{"access_token":"token"}`), nil
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
	if err != identity.ErrProjectMemberRequired {
		t.Fatalf("ExchangeIdentity() error = %v", err)
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
