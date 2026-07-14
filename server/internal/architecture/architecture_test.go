package architecture_test

import (
	"bytes"
	"context"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

const modulePath = "example.com/project-template/"

func TestDomainDoesNotImportOuterLayers(t *testing.T) {
	assertNoImports(t, "./internal/domain/...", []string{"internal/controller/", "internal/platform/", "cmd/"})
}

func TestApplicationDoesNotImportAdapters(t *testing.T) {
	assertNoImports(t, "./internal/controller/application/...", []string{"internal/controller/app", "internal/controller/config", "internal/controller/infrastructure/", "internal/controller/logger", "internal/controller/transport/", "cmd/"})
}

func TestPostgresDoesNotImportApplicationOrTransport(t *testing.T) {
	assertNoImports(t, "./internal/controller/infrastructure/postgres/...", []string{"internal/controller/application/", "internal/controller/transport/", "internal/controller/app"})
}

func TestTransportDoesNotImportInfrastructure(t *testing.T) {
	assertNoImports(t, "./internal/controller/transport/...", []string{"internal/controller/infrastructure/", "internal/controller/app"})
}

func assertNoImports(t *testing.T, pattern string, forbidden []string) {
	t.Helper()
	for _, line := range goList(t, pattern) {
		pkg, imports, ok := strings.Cut(line, "|")
		if !ok {
			t.Fatalf("unexpected go list output %q", line)
		}
		for _, imported := range strings.Fields(imports) {
			relative, local := strings.CutPrefix(imported, modulePath)
			if !local {
				continue
			}
			for _, prefix := range forbidden {
				if forbiddenImport(relative, prefix) {
					t.Fatalf("%s imports forbidden package %s", pkg, imported)
				}
			}
		}
	}
}

func forbiddenImport(relative, forbidden string) bool {
	if strings.HasSuffix(forbidden, "/") {
		return strings.HasPrefix(relative, forbidden)
	}
	return relative == forbidden || strings.HasPrefix(relative, forbidden+"/")
}

func goList(t *testing.T, pattern string) []string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test path")
	}
	root := strings.TrimSuffix(filename, "/internal/architecture/architecture_test.go")
	cmd := exec.CommandContext(context.Background(), "go", "list", "-f", `{{.ImportPath}}|{{join .Imports " "}}`, pattern)
	cmd.Dir = root
	cmd.Env = append(cmd.Environ(), "GOTOOLCHAIN=local", "GOCACHE="+t.TempDir())
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list %s: %v\n%s", pattern, err, stderr.String())
	}
	return strings.Split(strings.TrimSpace(string(output)), "\n")
}
