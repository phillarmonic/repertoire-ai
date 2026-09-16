package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phillarmonic/repertoire-ai/internal/catalog"
	installer "github.com/phillarmonic/repertoire-ai/internal/install"
	"github.com/phillarmonic/repertoire-ai/internal/state"
)

func TestWriteSkillShowJSONIsStableObject(t *testing.T) {
	t.Parallel()
	view := sampleShowView()
	var output bytes.Buffer
	if err := writeSkillShow(&output, view, skillListFormatJSON); err != nil {
		t.Fatal(err)
	}
	var decoded skillShowView
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, output.String())
	}
	if decoded.Name != "demo" || decoded.Catalog != "local" || decoded.Origin != "declared" {
		t.Fatalf("JSON object = %+v", decoded)
	}
	if len(decoded.Targets) != 2 || decoded.Targets[0].Status != showCopyIntact || decoded.Targets[1].Status != showCopyModified {
		t.Fatalf("JSON targets = %+v", decoded.Targets)
	}
}

func TestWriteSkillShowTableListsIntegrity(t *testing.T) {
	t.Parallel()
	view := sampleShowView()
	var output bytes.Buffer
	if err := writeSkillShow(&output, view, skillListFormatTable); err != nil {
		t.Fatal(err)
	}
	result := output.String()
	for _, expected := range []string{
		"SKILL", "demo", "CATALOG", "local", "SOURCE", "/catalogs/local",
		"COMMIT", "abc123", "DIGEST", "deadbeef", "ORIGIN", "declared",
		"LOOSE", "true", "CACHE", "present",
		"agents", "/project/.agents/skills/demo", "intact",
		"codex", "/project/.codex/skills/demo", "modified",
	} {
		if !strings.Contains(result, expected) {
			t.Fatalf("table missing %q:\n%s", expected, result)
		}
	}
}

func TestBuildSkillShowViewReportsPerTargetIntegrity(t *testing.T) {
	t.Parallel()
	project := t.TempDir()
	catalogRoot := t.TempDir()
	skillRoot := filepath.Join(catalogRoot, "skills", "demo")
	if err := os.MkdirAll(skillRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	content := []byte("---\nname: demo\ndescription: Test skill\n---\nbody\n")
	if err := os.WriteFile(filepath.Join(skillRoot, "SKILL.md"), content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(catalogRoot, "repertoire.yaml"), []byte("schema: 1\ncatalog:\n  name: local\n  skills:\n    demo:\n      path: skills/demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	digest, err := installer.Digest(skillRoot)
	if err != nil {
		t.Fatal(err)
	}
	agentsPath := filepath.Join(project, ".agents", "skills", "demo")
	codexPath := filepath.Join(project, ".codex", "skills", "demo")
	copySkillDir(t, skillRoot, agentsPath)
	copySkillDir(t, skillRoot, codexPath)
	if writeErr := os.WriteFile(filepath.Join(codexPath, "SKILL.md"), append(content, []byte("edit\n")...), 0o644); writeErr != nil {
		t.Fatal(writeErr)
	}

	manifest := state.NewManifest()
	manifest.Catalogs["local"] = state.CatalogRegistration{Source: catalogRoot}
	lock := state.NewLock()
	lock.Skills["demo"] = state.LockSkill{
		Catalog: "local", Source: catalogRoot, Commit: "abc123", Digest: digest,
		Origin: state.LockOriginDeclared, Declared: true,
		Targets:       []string{"agents", "codex"},
		Locations:     []string{agentsPath, codexPath},
		TargetDigests: map[string]string{"agents": digest, "codex": digest},
	}
	manager, err := catalog.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	view, err := buildSkillShowView(state.Scope{Root: project}, manifest, lock, "demo", manager)
	if err != nil {
		t.Fatal(err)
	}
	if view.Origin != "declared" || view.CatalogCache != showCachePresent || view.Loose {
		t.Fatalf("view = %+v", view)
	}
	if len(view.Targets) != 2 {
		t.Fatalf("targets = %+v", view.Targets)
	}
	if view.Targets[0].Name != "agents" || view.Targets[0].Status != showCopyIntact {
		t.Fatalf("agents target = %+v", view.Targets[0])
	}
	if view.Targets[1].Name != "codex" || view.Targets[1].Status != showCopyModified {
		t.Fatalf("codex target = %+v", view.Targets[1])
	}

	_, err = buildSkillShowView(state.Scope{Root: project}, manifest, lock, "missing", manager)
	if err == nil || !strings.Contains(err.Error(), `skill "missing" is not managed in this scope`) {
		t.Fatalf("missing skill error = %v", err)
	}
}

func TestAnnotateShowCatalogNotesAbsentCache(t *testing.T) {
	t.Parallel()
	view := skillShowView{Catalog: "remote"}
	manifest := state.NewManifest()
	manifest.Catalogs["remote"] = state.CatalogRegistration{Source: "https://example.invalid/skills.git"}
	manager, err := catalog.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	annotateShowCatalog(&view, manifest, manager)
	if view.CatalogCache != showCacheAbsent {
		t.Fatalf("cache = %q, want absent", view.CatalogCache)
	}
}

func sampleShowView() skillShowView {
	return skillShowView{
		Name: "demo", Catalog: "local", Source: "/catalogs/local",
		Ref: "main", Commit: "abc123", Digest: "deadbeef",
		Origin: "declared", Hooks: true, Loose: true, CatalogCache: showCachePresent,
		Targets: []skillShowTarget{
			{Name: "agents", Path: "/project/.agents/skills/demo", Status: showCopyIntact},
			{Name: "codex", Path: "/project/.codex/skills/demo", Status: showCopyModified},
		},
	}
}

func copySkillDir(t *testing.T, source, destination string) {
	t.Helper()
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		content, err := os.ReadFile(filepath.Join(source, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(destination, entry.Name()), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
