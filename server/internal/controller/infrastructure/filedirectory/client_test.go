package filedirectory

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"example.com/project-template/internal/domain/directory"
)

// The milestone dates are deliberately unquoted: yaml.v3 tags such scalars as
// timestamps, and this fixture proves they still decode into plain strings.
const testDirectoryYAML = "version: 1\nteams:\n  - key: development\n    name: 開發組\n    title_prefix: '[開發組]'\n    gitlab_label: '組別::開發'\n    active: true\n    members: [alice]\n    leaders: [alice]\nmilestones:\n  - name: 一籌\n    date: 2026-08-29\n    kind: organizing\n  - name: 年會\n    date: 2027-03-13\n    kind: conference\n"

func TestDirectoryRevisionCachesFileContents(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "board-directory.yml")
	if err := os.WriteFile(path, []byte(testDirectoryYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	client, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	revision, err := client.DirectoryRevision(context.Background())
	if err != nil || revision == "" {
		t.Fatalf("DirectoryRevision() = %q, %v", revision, err)
	}
	if err := os.WriteFile(path, []byte("invalid: ["), 0o600); err != nil {
		t.Fatal(err)
	}
	file, fileRevision, err := client.DirectoryFile(context.Background())
	if err != nil || fileRevision != revision || len(file.Teams) != 1 || file.Teams[0].Members[0] != "alice" || file.Teams[0].Leaders[0] != "alice" {
		t.Fatalf("DirectoryFile() = %#v, %q, %v", file, fileRevision, err)
	}
	wantMilestones := []directory.MilestoneConfig{
		{Name: "一籌", Date: "2026-08-29", Kind: directory.MilestoneOrganizing},
		{Name: "年會", Date: "2027-03-13", Kind: directory.MilestoneConference},
	}
	if !reflect.DeepEqual(file.Milestones, wantMilestones) {
		t.Fatalf("milestones = %#v, want %#v", file.Milestones, wantMilestones)
	}
}

func TestDirectoryFileReportsMissingFile(t *testing.T) {
	t.Parallel()
	client, err := New(filepath.Join(t.TempDir(), "missing.yml"))
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = client.DirectoryFile(context.Background())
	if err == nil {
		t.Fatal("DirectoryFile() error = nil")
	}
}
