package githubdirectory

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"example.com/project-template/internal/domain/directory"
	"gopkg.in/yaml.v3"
)

type Config struct {
	BaseURL    string
	Owner      string
	Repository string
	Path       string
	Ref        string
	Token      string
}

type Client struct {
	http   *http.Client
	config Config
	base   *url.URL

	mu      sync.Mutex
	pending *snapshot
}

type snapshot struct {
	file     directory.File
	revision string
}

func New(httpClient *http.Client, config Config) (*Client, error) {
	base, err := url.Parse(strings.TrimRight(config.BaseURL, "/"))
	if err != nil || base.Scheme == "" || base.Host == "" {
		return nil, fmt.Errorf("invalid GitHub API base URL")
	}
	if strings.TrimSpace(config.Owner) == "" || strings.TrimSpace(config.Repository) == "" || strings.TrimSpace(config.Path) == "" {
		return nil, fmt.Errorf("GitHub directory owner, repository, and path are required")
	}
	if strings.TrimSpace(config.Ref) == "" {
		config.Ref = "main"
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{http: httpClient, config: config, base: base}, nil
}

func (c *Client) DirectoryRevision(ctx context.Context) (string, error) {
	loaded, err := c.load(ctx)
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	c.pending = &loaded
	c.mu.Unlock()
	return loaded.revision, nil
}

func (c *Client) DirectoryFile(ctx context.Context) (directory.File, string, error) {
	c.mu.Lock()
	if c.pending != nil {
		loaded := *c.pending
		c.pending = nil
		c.mu.Unlock()
		return loaded.file, loaded.revision, nil
	}
	c.mu.Unlock()

	loaded, err := c.load(ctx)
	if err != nil {
		return directory.File{}, "", err
	}
	return loaded.file, loaded.revision, nil
}

func (c *Client) load(ctx context.Context) (snapshot, error) {
	requestURL := c.base.JoinPath(
		"repos",
		strings.TrimSpace(c.config.Owner),
		strings.TrimSpace(c.config.Repository),
		"contents",
		strings.TrimSpace(c.config.Path),
	)
	query := requestURL.Query()
	query.Set("ref", strings.TrimSpace(c.config.Ref))
	requestURL.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return snapshot{}, fmt.Errorf("create GitHub directory request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", "sitcon-board-controller")
	if token := strings.TrimSpace(c.config.Token); token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	response, err := c.http.Do(request)
	if err != nil {
		return snapshot{}, fmt.Errorf("load GitHub directory: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return snapshot{}, fmt.Errorf("GitHub returned HTTP %d", response.StatusCode)
	}

	var wire struct {
		SHA      string `json:"sha"`
		Encoding string `json:"encoding"`
		Content  string `json:"content"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&wire); err != nil {
		return snapshot{}, fmt.Errorf("decode GitHub directory response: %w", err)
	}
	if strings.TrimSpace(wire.SHA) == "" {
		return snapshot{}, fmt.Errorf("GitHub directory response omitted sha")
	}
	if wire.Encoding != "base64" {
		return snapshot{}, fmt.Errorf("unsupported GitHub directory encoding %q", wire.Encoding)
	}
	content, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(wire.Content, "\n", ""))
	if err != nil {
		return snapshot{}, fmt.Errorf("decode GitHub directory content: %w", err)
	}

	var source struct {
		Version int `yaml:"version"`
		Teams   []struct {
			Key         string   `yaml:"key"`
			Name        string   `yaml:"name"`
			TitlePrefix string   `yaml:"title_prefix"`
			GitLabLabel string   `yaml:"gitlab_label"`
			Active      bool     `yaml:"active"`
			Members     []string `yaml:"members"`
		} `yaml:"teams"`
	}
	if err := yaml.Unmarshal(content, &source); err != nil {
		return snapshot{}, fmt.Errorf("parse board directory YAML: %w", err)
	}
	file := directory.File{Version: source.Version, Teams: make([]directory.TeamConfig, 0, len(source.Teams))}
	for _, team := range source.Teams {
		file.Teams = append(file.Teams, directory.TeamConfig{
			Key: team.Key, Name: team.Name, TitlePrefix: team.TitlePrefix,
			GitLabLabel: team.GitLabLabel, Active: team.Active, Members: team.Members,
		})
	}
	return snapshot{file: file, revision: wire.SHA}, nil
}
