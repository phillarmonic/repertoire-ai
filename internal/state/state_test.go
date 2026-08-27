package state

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestManifestRoundTripAndValidation(t *testing.T) {
	t.Parallel()
	manifest := NewManifest()
	manifest.Catalog = &CatalogDefinition{
		Name: "example",
		Skills: map[string]SkillEntry{
			"code-reviewer": {
				Path:     "skills/code-reviewer",
				Variants: map[string]string{"codex": "platforms/codex"},
				Instructions: map[string][]ArtifactEntry{
					"codex": {{
						ID: "guidance", Source: "platforms/codex/agents.md",
						Destination: "AGENTS.md", Mode: ArtifactModeMarkdownSection,
					}},
				},
				Artifacts: map[string][]ArtifactEntry{
					"codex": {{
						ID: "hooks", Source: "platforms/codex/hooks.json",
						Destination: ".codex/hooks.json", Mode: ArtifactModeJSONMerge,
					}},
				},
			},
		},
	}
	manifest.Catalogs["private"] = CatalogRegistration{Source: "git@example.test:org/skills.git", Ref: "main"}
	manifest.Skills["github.com/phillarmonic/ai-skills/zensical"] = BootstrapSkill{Scope: BootstrapScopeGlobal}
	manifest.Skills["shared-helpers"] = BootstrapSkill{
		Catalog: "private",
		Scope:   BootstrapScopeProject,
		Targets: []string{"agents"},
		Hooks:   true,
	}
	manifest.Requirements["code-reviewer"] = Requirement{Catalog: "example", Targets: []string{"codex"}}

	first, err := manifest.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(first), "tool: https://github.com/phillarmonic/repertoire-ai\n") {
		t.Fatalf("manifest marker missing:\n%s", first)
	}
	path := filepath.Join(t.TempDir(), "repertoire.yaml")
	if writeErr := WriteFileAtomic(path, first, 0o644); writeErr != nil {
		t.Fatalf("write: %v", writeErr)
	}
	loaded, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	second, err := loaded.Marshal()
	if err != nil {
		t.Fatalf("marshal loaded: %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("manifest is not deterministic:\n%s\n%s", first, second)
	}

	loaded.Catalog.Skills["escape"] = SkillEntry{Path: "../escape"}
	if _, err := loaded.Marshal(); err == nil {
		t.Fatal("expected escaping path to fail")
	}
}

func TestLoadManifestAcceptsLegacyFileWithoutToolMarker(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "repertoire.yaml")
	content := `schema: 1
catalog:
  name: example
  skills:
    demo:
      path: skills/demo
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest, err := LoadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Tool != ManifestTool {
		t.Fatalf("legacy manifest default tool marker = %q", manifest.Tool)
	}
}

func TestLoadManifestDefaultsSkillScopes(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "repertoire.yaml")
	content := `schema: 1
skills:
  demo:
    targets: [codex]
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest, err := LoadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Skills["demo"].Scope != BootstrapScopeGlobal {
		t.Fatalf("default skill scope = %q", manifest.Skills["demo"].Scope)
	}
}

func TestLoadManifestRejectsInvalidSkillDeclarations(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "scope", body: "schema: 1\nskills:\n  demo:\n    scope: elsewhere\n", want: "scope must be"},
		{name: "target", body: "schema: 1\nskills:\n  demo:\n    targets: [unknown]\n", want: "unknown target"},
		{name: "credentials", body: "schema: 1\ncatalogs:\n  private:\n    source: https://token@example.test/skills.git\n", want: "embedded credentials"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "repertoire.yaml")
			if err := os.WriteFile(path, []byte(test.body), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadManifest(path); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestCatalogRejectsUnsafeVariantAndArtifactDeclarations(t *testing.T) {
	t.Parallel()
	base := func() Manifest {
		manifest := NewManifest()
		manifest.Catalog = &CatalogDefinition{
			Name: "example",
			Skills: map[string]SkillEntry{
				"graphify": {Path: "skills/graphify"},
			},
		}
		return manifest
	}
	variant := base()
	entry := variant.Catalog.Skills["graphify"]
	entry.Variants = map[string]string{"codex": "../escape"}
	variant.Catalog.Skills["graphify"] = entry
	if err := variant.Validate(); err == nil || !strings.Contains(err.Error(), "contained relative path") {
		t.Fatalf("unsafe variant error = %v", err)
	}

	artifact := base()
	entry = artifact.Catalog.Skills["graphify"]
	entry.Artifacts = map[string][]ArtifactEntry{
		"codex": {{
			ID: "hooks", Source: "hooks.json", Destination: "../../escape", Mode: ArtifactModeJSONMerge,
		}},
	}
	artifact.Catalog.Skills["graphify"] = entry
	if err := artifact.Validate(); err == nil || !strings.Contains(err.Error(), "contained relative path") {
		t.Fatalf("unsafe artifact error = %v", err)
	}

	instruction := base()
	entry = instruction.Catalog.Skills["graphify"]
	entry.Instructions = map[string][]ArtifactEntry{
		"codex": {{
			ID: "guidance", Source: "../escape", Destination: "AGENTS.md", Mode: ArtifactModeMarkdownSection,
		}},
	}
	instruction.Catalog.Skills["graphify"] = entry
	if err := instruction.Validate(); err == nil || !strings.Contains(err.Error(), "contained relative path") {
		t.Fatalf("unsafe instruction error = %v", err)
	}

	duplicate := base()
	entry = duplicate.Catalog.Skills["graphify"]
	entry.Instructions = map[string][]ArtifactEntry{
		"codex": {{
			ID: "managed", Source: "agents.md", Destination: "AGENTS.md", Mode: ArtifactModeMarkdownSection,
		}},
	}
	entry.Artifacts = map[string][]ArtifactEntry{
		"codex": {{
			ID: "managed", Source: "hooks.json", Destination: ".codex/hooks.json", Mode: ArtifactModeJSONMerge,
		}},
	}
	duplicate.Catalog.Skills["graphify"] = entry
	if err := duplicate.Validate(); err == nil || !strings.Contains(err.Error(), "across instruction and artifact") {
		t.Fatalf("duplicate managed id error = %v", err)
	}
}

func TestCatalogSkillNamesAllowQualifiedSegments(t *testing.T) {
	t.Parallel()
	manifest := NewManifest()
	manifest.Catalog = &CatalogDefinition{
		Name: "example",
		Skills: map[string]SkillEntry{
			"phillarmonkey/code": {Path: "skills/code"},
		},
	}
	if err := manifest.Validate(); err != nil {
		t.Fatalf("qualified catalog skill name should validate: %v", err)
	}

	manifest.Catalog.Skills["Bad_Vendor/code"] = SkillEntry{Path: "skills/bad"}
	if err := manifest.Validate(); err == nil {
		t.Fatal("expected invalid qualified catalog skill name to fail")
	}
}

func TestLockMarshalDeterministic(t *testing.T) {
	t.Parallel()
	lock := NewLock()
	lock.Skills["z"] = LockSkill{Catalog: "z", Commit: "1", Digest: "a"}
	lock.Skills["a"] = LockSkill{Catalog: "a", Commit: "2", Digest: "b"}
	first, err := lock.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	second, err := lock.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) || !strings.HasSuffix(string(first), "\n") {
		t.Fatal("lock encoding is not deterministic")
	}
}

func TestLockOriginsRemainBackwardCompatible(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "repertoire.lock.json")
	legacy := `{
  "schema": 1,
  "skills": {
    "declared": {"declared": true},
    "loose": {"declared": false}
  }
}
`
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	lock, err := LoadLock(path)
	if err != nil {
		t.Fatal(err)
	}
	if lock.Skills["declared"].EffectiveOrigin() != LockOriginDeclared {
		t.Fatalf("legacy declared origin = %q", lock.Skills["declared"].EffectiveOrigin())
	}
	if lock.Skills["loose"].EffectiveOrigin() != LockOriginAdHoc {
		t.Fatalf("legacy loose origin = %q", lock.Skills["loose"].EffectiveOrigin())
	}
	lock.Skills["bootstrapped"] = LockSkill{Origin: LockOriginBootstrap}
	content, err := lock.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `"origin": "bootstrap"`) {
		t.Fatalf("bootstrap origin missing from lock:\n%s", content)
	}
}

func TestResolveScope(t *testing.T) {
	t.Parallel()
	project := t.TempDir()
	if err := exec.Command("git", "-C", project, "init", "-q").Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	nested := filepath.Join(project, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(t.TempDir(), "config")
	scope, err := ResolveScope(ScopeOptions{Directory: nested, ConfigDir: config})
	if err != nil {
		t.Fatal(err)
	}
	if !scope.Global || scope.Root != config {
		t.Fatalf("unexpected default global scope: %+v", scope)
	}

	projectRoot, err := filepath.EvalSymlinks(project)
	if err != nil {
		t.Fatal(err)
	}
	scope, err = ResolveScope(ScopeOptions{Project: true, Directory: nested})
	if err != nil {
		t.Fatal(err)
	}
	if scope.Global || scope.Root != projectRoot {
		t.Fatalf("unexpected project scope: %+v", scope)
	}

	scope, err = ResolveScope(ScopeOptions{Global: true, Directory: nested, ConfigDir: config})
	if err != nil {
		t.Fatal(err)
	}
	if !scope.Global || scope.Root != config {
		t.Fatalf("unexpected global scope: %+v", scope)
	}
	if _, err := ResolveScope(ScopeOptions{Global: true, Project: true}); err == nil {
		t.Fatal("expected conflicting flags to fail")
	}
	if _, err := ResolveScope(ScopeOptions{Project: true, Directory: t.TempDir()}); err == nil {
		t.Fatal("expected --project outside a Git worktree to fail")
	}
}

func TestAtomicWritePreservesExistingFileOnValidationFailure(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "repertoire.yaml")
	if err := os.WriteFile(path, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	invalid := NewManifest()
	invalid.Schema = 99
	if err := SaveManifest(path, invalid); err == nil {
		t.Fatal("expected validation failure")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "original" {
		t.Fatalf("existing content changed: %q", content)
	}
}
