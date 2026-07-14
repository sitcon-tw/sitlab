package gitlab

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"example.com/project-template/internal/domain/identity"
)

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
