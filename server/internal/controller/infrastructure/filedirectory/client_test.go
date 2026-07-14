package filedirectory

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

const testDirectoryYAML = "version: 1\nteams:\n  - key: development\n    name: 開發組\n    title_prefix: '[開發組]'\n    gitlab_label: '組別::開發'\n    active: true\n    members: [alice]\n"

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
	if err != nil || fileRevision != revision || len(file.Teams) != 1 || file.Teams[0].Members[0] != "alice" {
		t.Fatalf("DirectoryFile() = %#v, %q, %v", file, fileRevision, err)
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
