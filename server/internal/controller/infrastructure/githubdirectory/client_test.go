package githubdirectory

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestDirectoryRevisionCachesGitHubContentsResponse(t *testing.T) {
	t.Parallel()
	directoryYAML := "version: 1\nteams:\n  - key: development\n    name: 開發組\n    title_prefix: '[開發組]'\n    gitlab_label: '組別::開發'\n    active: true\n    members: [alice]\n"
	requests := 0
	client, err := New(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		if request.URL.EscapedPath() != "/repos/sitcon-tw/sitlab/contents/.sitcon/board-directory.yml" {
			t.Errorf("path = %q", request.URL.EscapedPath())
		}
		if request.URL.Query().Get("ref") != "main" {
			t.Errorf("ref = %q", request.URL.Query().Get("ref"))
		}
		if request.Header.Get("Authorization") != "Bearer github-token" {
			t.Errorf("Authorization = %q", request.Header.Get("Authorization"))
		}
		if request.Header.Get("Accept") != "application/vnd.github+json" {
			t.Errorf("Accept = %q", request.Header.Get("Accept"))
		}
		body := `{"sha":"blob-sha","encoding":"base64","content":"` + base64.StdEncoding.EncodeToString([]byte(directoryYAML)) + `"}`
		return response(http.StatusOK, body), nil
	})}, Config{
		BaseURL: "https://api.github.test", Owner: "sitcon-tw", Repository: "sitlab",
		Path: ".sitcon/board-directory.yml", Ref: "main", Token: "github-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	revision, err := client.DirectoryRevision(context.Background())
	if err != nil || revision != "blob-sha" {
		t.Fatalf("DirectoryRevision() = %q, %v", revision, err)
	}
	file, revision, err := client.DirectoryFile(context.Background())
	if err != nil || revision != "blob-sha" || len(file.Teams) != 1 || file.Teams[0].Members[0] != "alice" {
		t.Fatalf("DirectoryFile() = %#v, %q, %v", file, revision, err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
}

func TestDirectoryFileReportsGitHubStatus(t *testing.T) {
	t.Parallel()
	client, err := New(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusNotFound, `{}`), nil
	})}, Config{
		BaseURL: "https://api.github.test", Owner: "sitcon-tw", Repository: "sitlab",
		Path: ".sitcon/board-directory.yml", Ref: "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = client.DirectoryFile(context.Background())
	if err == nil || !strings.Contains(err.Error(), "GitHub returned HTTP 404") {
		t.Fatalf("DirectoryFile() error = %v", err)
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
