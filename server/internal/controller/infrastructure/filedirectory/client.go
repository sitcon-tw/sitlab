package filedirectory

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"sync"

	"example.com/project-template/internal/domain/directory"
	"gopkg.in/yaml.v3"
)

type Client struct {
	path string

	mu      sync.Mutex
	pending *snapshot
}

type snapshot struct {
	file     directory.File
	revision string
}

func New(path string) (*Client, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("board directory file path is required")
	}
	return &Client{path: strings.TrimSpace(path)}, nil
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
	if err := ctx.Err(); err != nil {
		return snapshot{}, err
	}
	content, err := os.ReadFile(c.path)
	if err != nil {
		return snapshot{}, fmt.Errorf("read board directory file: %w", err)
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
			Leaders     []string `yaml:"leaders"`
		} `yaml:"teams"`
		Milestones []struct {
			Name string `yaml:"name"`
			Date string `yaml:"date"`
			Kind string `yaml:"kind"`
		} `yaml:"milestones"`
	}
	if err := yaml.Unmarshal(content, &source); err != nil {
		return snapshot{}, fmt.Errorf("parse board directory YAML: %w", err)
	}
	file := directory.File{Version: source.Version, Teams: make([]directory.TeamConfig, 0, len(source.Teams))}
	for _, team := range source.Teams {
		file.Teams = append(file.Teams, directory.TeamConfig{
			Key: team.Key, Name: team.Name, TitlePrefix: team.TitlePrefix,
			GitLabLabel: team.GitLabLabel, Active: team.Active, Members: team.Members, Leaders: team.Leaders,
		})
	}
	file.Milestones = make([]directory.MilestoneConfig, 0, len(source.Milestones))
	for _, milestone := range source.Milestones {
		file.Milestones = append(file.Milestones, directory.MilestoneConfig{
			Name: milestone.Name, Date: milestone.Date, Kind: directory.MilestoneKind(milestone.Kind),
		})
	}
	// The format prefix makes every revision stored by an older build a mismatch, so
	// the first refresh after a deploy re-reads the file instead of trusting the
	// snapshot fast path. Without it, an old process that consumed a YAML change the
	// new build parses more fully (milestones, say) would pin the richer fields empty
	// until the file happened to change again. Bump it when load() learns new fields.
	revision := fmt.Sprintf("v2:%x", sha256.Sum256(content))
	return snapshot{file: file, revision: revision}, nil
}
