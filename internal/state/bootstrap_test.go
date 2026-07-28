package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadBootstrapManifestDefaultsAndRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), BootstrapFileName)
	content := `schema: 1
catalogs:
  company:
    source: git@example.test:company/skills.git
    ref: main
skills:
  reviewer:
    catalog: company
    targets: [codex, claude, cline, cursor, gemini, junie, kimi, kiro, opencode, openclaw, roo, windsurf]
  github.com/phillarmonic/ai-skills/zensical:
    scope: global
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest, err := LoadBootstrapManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Skills["reviewer"].Scope != BootstrapScopeGlobal {
		t.Fatalf("default scope = %q", manifest.Skills["reviewer"].Scope)
	}
	first, err := manifest.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(first), "tool: https://github.com/phillarmonic/repertoire-ai\n") {
		t.Fatalf("bootstrap manifest marker missing:\n%s", first)
	}
	loadedPath := filepath.Join(t.TempDir(), BootstrapFileName)
	if err := os.WriteFile(loadedPath, first, 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadBootstrapManifest(loadedPath)
	if err != nil {
		t.Fatal(err)
	}
	second, err := loaded.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("bootstrap manifest is not deterministic:\n%s\n%s", first, second)
	}
}

func TestLoadBootstrapManifestAcceptsLegacyFileWithoutToolMarker(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), BootstrapFileName)
	content := `schema: 1
skills:
  demo:
    scope: global
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest, err := LoadBootstrapManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Tool != ManifestTool {
		t.Fatalf("legacy bootstrap manifest default tool marker = %q", manifest.Tool)
	}
}

func TestBootstrapManifestValidation(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "missing", body: "", want: "does not exist"},
		{name: "empty", body: "schema: 1\nskills: {}\n", want: "at least one skill"},
		{name: "scope", body: "schema: 1\nskills:\n  demo:\n    scope: elsewhere\n", want: "scope must be"},
		{name: "target", body: "schema: 1\nskills:\n  demo:\n    scope: project\n    targets: [unknown]\n", want: "unknown target"},
		{name: "catalog", body: "schema: 1\nskills:\n  demo:\n    scope: project\n    catalog: missing\n", want: "unknown catalog"},
		{name: "credentials", body: "schema: 1\ncatalogs:\n  private:\n    source: https://token@example.test/skills.git\nskills:\n  demo:\n    scope: project\n    catalog: private\n", want: "embedded credentials"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), BootstrapFileName)
			if test.body != "" {
				if err := os.WriteFile(path, []byte(test.body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			_, err := LoadBootstrapManifest(path)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
	}
}
