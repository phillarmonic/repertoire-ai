package install

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/phillarmonic/repertoire-ai/internal/catalog"
	"github.com/phillarmonic/repertoire-ai/internal/state"
)

func TestValidateDigestAndSafeInstall(t *testing.T) {
	t.Parallel()
	source := skillFixture(t, "demo")
	if err := ValidateSkill(source, "demo"); err != nil {
		t.Fatal(err)
	}
	digest, err := Digest(source)
	if err != nil {
		t.Fatal(err)
	}
	targetRoot := filepath.Join(t.TempDir(), "skills")
	resolved := ResolvedSkill{Name: "demo", Root: source, Digest: digest}
	locations, err := Skill(resolved, []Target{{Name: "codex", Root: targetRoot}}, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(locations) != 1 {
		t.Fatalf("unexpected locations: %v", locations)
	}
	installedSkill := filepath.Join(locations[0], "SKILL.md")
	preservedTime := time.Unix(1234, 0)
	if err := os.Chtimes(installedSkill, preservedTime, preservedTime); err != nil {
		t.Fatal(err)
	}
	previous := &state.LockSkill{Digest: digest}
	if _, err := Skill(resolved, []Target{{Name: "codex", Root: targetRoot}}, previous, false); err != nil {
		t.Fatalf("skip intact install: %v", err)
	}
	info, err := os.Stat(installedSkill)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(preservedTime) {
		t.Fatalf("intact target was rewritten: modtime = %s", info.ModTime())
	}

	if err := os.WriteFile(filepath.Join(locations[0], "local.txt"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Skill(resolved, []Target{{Name: "codex", Root: targetRoot}}, previous, false); err == nil {
		t.Fatal("expected modified target conflict")
	}
	if _, err := Skill(resolved, []Target{{Name: "codex", Root: targetRoot}}, previous, true); err != nil {
		t.Fatalf("force install: %v", err)
	}
}

func TestQualifiedCatalogSkillInstallsToFlatDirectory(t *testing.T) {
	t.Parallel()
	source := skillFixtureWithDirectory(t, "code", "phillarmonkey/code")
	if err := ValidateSkill(source, "phillarmonkey/code"); err != nil {
		t.Fatal(err)
	}
	digest, err := Digest(source)
	if err != nil {
		t.Fatal(err)
	}
	targetRoot := filepath.Join(t.TempDir(), "skills")
	resolved := ResolvedSkill{Name: "phillarmonkey/code", Root: source, Digest: digest}
	locations, err := Skill(resolved, []Target{{Name: "codex", Root: targetRoot}}, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(targetRoot, "phillarmonkey-code")
	if len(locations) != 1 || locations[0] != want {
		t.Fatalf("locations = %v, want %s", locations, want)
	}
	if _, err := os.Stat(filepath.Join(want, "SKILL.md")); err != nil {
		t.Fatalf("installed qualified skill: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetRoot, "phillarmonkey", "code")); !os.IsNotExist(err) {
		t.Fatalf("qualified skill should not install to a nested path: %v", err)
	}
	if err := Remove("phillarmonkey/code", []Target{{Name: "codex", Root: targetRoot}}, state.LockSkill{Digest: digest}, false); err != nil {
		t.Fatalf("remove qualified skill: %v", err)
	}
	if _, err := os.Stat(want); !os.IsNotExist(err) {
		t.Fatalf("qualified skill was not removed: %v", err)
	}
}

func TestTargetVariantInstallsUnderLogicalSkillName(t *testing.T) {
	t.Parallel()
	defaultRoot := skillFixture(t, "graphify")
	codexRoot := skillFixtureWithDirectory(t, "codex", "graphify")
	defaultDigest, err := Digest(defaultRoot)
	if err != nil {
		t.Fatal(err)
	}
	codexDigest, err := Digest(codexRoot)
	if err != nil {
		t.Fatal(err)
	}
	resolved := ResolvedSkill{
		Name: "graphify", Root: defaultRoot, Digest: defaultDigest,
		Variants: map[string]ResolvedVariant{"codex": {Root: codexRoot, Digest: codexDigest}},
	}
	targetRoot := filepath.Join(t.TempDir(), "skills")
	locations, digests, err := SkillWithDigests(
		resolved, []Target{{Name: "codex", Root: targetRoot}}, nil, false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(locations) != 1 || filepath.Base(locations[0]) != "graphify" {
		t.Fatalf("variant locations = %v", locations)
	}
	if digests["codex"] != codexDigest {
		t.Fatalf("variant digest = %q, want %q", digests["codex"], codexDigest)
	}
}

func TestEscapingSymlinkRejected(t *testing.T) {
	t.Parallel()
	source := skillFixture(t, "escape")
	if err := os.Symlink("../../outside", filepath.Join(source, "bad")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := Digest(source); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("expected escaping symlink error, got %v", err)
	}
}

func TestValidateSkillRejectsInvalidStubManifest(t *testing.T) {
	t.Parallel()
	source := skillFixture(t, "demo")
	content := "schema: 1\nstubs:\n  broken:\n    description: Broken\n    path: assets/missing\n    instructions: Use it.\n"
	if err := os.WriteFile(filepath.Join(source, "stubs.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSkill(source, "demo"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected missing stub asset error, got %v", err)
	}
}

func TestResolveTargets(t *testing.T) {
	t.Parallel()
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	targets, err := ResolveTargets(state.Scope{Root: project}, nil, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 || targets[0].Name != "codex" {
		t.Fatalf("unexpected detected targets: %+v", targets)
	}
	explicit, err := ResolveTargets(state.Scope{Root: project}, []string{"agents"}, t.TempDir())
	if err != nil || len(explicit) != 1 || explicit[0].Name != "agents" {
		t.Fatalf("unexpected explicit targets: %+v, %v", explicit, err)
	}
}

func TestResolveAllTargets(t *testing.T) {
	project := t.TempDir()
	targets, err := ResolveTargets(state.Scope{Root: project}, []string{"all"}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(targets))
	for _, target := range targets {
		names = append(names, target.Name)
	}
	if want := SupportedTargetNames(); !reflect.DeepEqual(names, want) {
		t.Fatalf("all targets = %v, want %v", names, want)
	}
}

func TestExplicitTargetRoots(t *testing.T) {
	project, home := t.TempDir(), t.TempDir()
	t.Setenv("CODEX_HOME", "")
	t.Setenv("KIMI_CODE_HOME", "")
	t.Setenv("OPENCLAW_STATE_DIR", "")
	t.Setenv("DSH_HOME", "")

	tests := []struct {
		name        string
		projectRoot string
		globalRoot  string
	}{
		{"aider", filepath.Join(project, ".aider"), filepath.Join(home, ".aider")},
		{"amp", filepath.Join(project, ".agents", "skills"), filepath.Join(home, ".config", "agents", "skills")},
		{"antigravity", filepath.Join(project, ".agents", "skills"), filepath.Join(home, ".gemini", "config", "skills")},
		{"antigravity-windows", filepath.Join(project, ".agents", "skills"), filepath.Join(home, ".gemini", "config", "skills")},
		{"claude", filepath.Join(project, ".claude", "skills"), filepath.Join(home, ".claude", "skills")},
		{"claw", filepath.Join(project, ".openclaw", "skills"), filepath.Join(home, ".openclaw", "skills")},
		{"cline", filepath.Join(project, ".cline", "skills"), filepath.Join(home, ".cline", "skills")},
		{"codebuddy", filepath.Join(project, ".codebuddy", "skills"), filepath.Join(home, ".codebuddy", "skills")},
		{"cursor", filepath.Join(project, ".cursor", "skills"), filepath.Join(home, ".cursor", "skills")},
		{"devin", filepath.Join(project, ".devin", "skills"), filepath.Join(home, ".config", "devin", "skills")},
		{"droid", filepath.Join(project, ".factory", "skills"), filepath.Join(home, ".factory", "skills")},
		{"dsh", filepath.Join(project, ".dsh", "skills"), filepath.Join(home, ".dsh", "skills")},
		{"gemini", filepath.Join(project, ".gemini", "skills"), filepath.Join(home, ".gemini", "skills")},
		{"hermes", filepath.Join(project, ".hermes", "skills"), filepath.Join(home, ".hermes", "skills")},
		{"junie", filepath.Join(project, ".junie", "skills"), filepath.Join(home, ".junie", "skills")},
		{"kilo", filepath.Join(project, ".config", "kilo", "skills"), filepath.Join(home, ".config", "kilo", "skills")},
		{"kimi", filepath.Join(project, ".kimi-code", "skills"), filepath.Join(home, ".kimi-code", "skills")},
		{"kiro", filepath.Join(project, ".kiro", "skills"), filepath.Join(home, ".kiro", "skills")},
		{"opencode", filepath.Join(project, ".opencode", "skills"), filepath.Join(home, ".config", "opencode", "skills")},
		{"openclaw", filepath.Join(project, "skills"), filepath.Join(home, ".openclaw", "skills")},
		{"pi", filepath.Join(project, ".pi", "agent", "skills"), filepath.Join(home, ".pi", "agent", "skills")},
		{"roo", filepath.Join(project, ".roo", "skills"), filepath.Join(home, ".roo", "skills")},
		{"trae", filepath.Join(project, ".trae", "skills"), filepath.Join(home, ".trae", "skills")},
		{"trae-cn", filepath.Join(project, ".trae-cn", "skills"), filepath.Join(home, ".trae-cn", "skills")},
		{"vscode", filepath.Join(project, ".github", "skills"), filepath.Join(home, ".copilot", "skills")},
		{"windows", filepath.Join(project, ".claude", "skills"), filepath.Join(home, ".claude", "skills")},
		{"windsurf", filepath.Join(project, ".windsurf", "skills"), filepath.Join(home, ".codeium", "windsurf", "skills")},
	}
	for _, test := range tests {
		projectTargets, err := ResolveTargets(state.Scope{Root: project}, []string{test.name}, home)
		if err != nil || len(projectTargets) != 1 || projectTargets[0].Root != test.projectRoot {
			t.Fatalf("%s project target = %+v, %v", test.name, projectTargets, err)
		}
		globalTargets, err := ResolveTargets(state.Scope{Global: true}, []string{test.name}, home)
		if err != nil || len(globalTargets) != 1 || globalTargets[0].Root != test.globalRoot {
			t.Fatalf("%s global target = %+v, %v", test.name, globalTargets, err)
		}
	}
}

func TestDetectsAdditionalProfessionalAgentTargets(t *testing.T) {
	t.Parallel()
	project := t.TempDir()
	names := []string{"cline", "gemini", "junie", "kiro", "roo", "windsurf"}
	for _, name := range names {
		root, err := targetRoot(state.Scope{Root: project}, t.TempDir(), name)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(root), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	targets, err := ResolveTargets(state.Scope{Root: project}, nil, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != len(names) {
		t.Fatalf("detected targets = %+v, want %v", targets, names)
	}
	for index, name := range names {
		if targets[index].Name != name {
			t.Fatalf("detected targets = %+v, want %v", targets, names)
		}
	}
}

func TestDetectsOpenCodeAndKimiProjectTargets(t *testing.T) {
	t.Parallel()
	project := t.TempDir()
	for _, directory := range []string{".opencode", ".kimi-code"} {
		if err := os.MkdirAll(filepath.Join(project, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	targets, err := ResolveTargets(state.Scope{Root: project}, nil, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 2 || targets[0].Name != "kimi" || targets[1].Name != "opencode" {
		t.Fatalf("unexpected detected targets: %+v", targets)
	}
}

func TestKimiCodeHomeOverride(t *testing.T) {
	kimiHome := t.TempDir()
	t.Setenv("KIMI_CODE_HOME", kimiHome)
	targets, err := ResolveTargets(state.Scope{Global: true}, []string{"kimi"}, t.TempDir())
	if err != nil || len(targets) != 1 || targets[0].Root != filepath.Join(kimiHome, "skills") {
		t.Fatalf("kimi override target = %+v, %v", targets, err)
	}
}

func TestDetectsCursorAndOpenClawProjectTargets(t *testing.T) {
	t.Parallel()
	project := t.TempDir()
	for _, directory := range []string{".cursor", "skills"} {
		if err := os.MkdirAll(filepath.Join(project, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	targets, err := ResolveTargets(state.Scope{Root: project}, nil, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 2 || targets[0].Name != "cursor" || targets[1].Name != "openclaw" {
		t.Fatalf("unexpected detected targets: %+v", targets)
	}
}

func TestOpenClawStateDirectoryOverride(t *testing.T) {
	stateDirectory := t.TempDir()
	t.Setenv("OPENCLAW_STATE_DIR", stateDirectory)
	targets, err := ResolveTargets(state.Scope{Global: true}, []string{"openclaw"}, t.TempDir())
	if err != nil || len(targets) != 1 || targets[0].Root != filepath.Join(stateDirectory, "skills") {
		t.Fatalf("openclaw override target = %+v, %v", targets, err)
	}
}

func TestDSHHomeOverride(t *testing.T) {
	dshHome := t.TempDir()
	t.Setenv("DSH_HOME", dshHome)
	targets, err := ResolveTargets(state.Scope{Global: true}, []string{"dsh"}, t.TempDir())
	if err != nil || len(targets) != 1 || targets[0].Root != filepath.Join(dshHome, "skills") {
		t.Fatalf("dsh override target = %+v, %v", targets, err)
	}
}

func TestResolverPrefersMainlineAndAcceptsQualification(t *testing.T) {
	t.Parallel()
	manifest := state.NewManifest()
	for _, name := range []string{"one", "two"} {
		root := t.TempDir()
		skill := filepath.Join(root, "skills", "demo")
		if err := os.MkdirAll(skill, 0o755); err != nil {
			t.Fatal(err)
		}
		content := "---\nname: demo\ndescription: Test\n---\n"
		if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		catalogManifest := state.NewManifest()
		catalogManifest.Catalog = &state.CatalogDefinition{
			Name: name, Skills: map[string]state.SkillEntry{"demo": {Path: "skills/demo"}},
		}
		if err := state.SaveManifest(filepath.Join(root, "repertoire.yaml"), catalogManifest); err != nil {
			t.Fatal(err)
		}
		manifest.Catalogs[name] = state.CatalogRegistration{Source: root}
	}
	manifest.Catalogs[catalog.BuiltinName] = manifest.Catalogs["one"]
	delete(manifest.Catalogs, "one")
	manifest.Catalogs["broken"] = state.CatalogRegistration{Source: filepath.Join(t.TempDir(), "missing")}
	manager, err := catalog.NewManager(filepath.Join(t.TempDir(), "cache"))
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := Resolve(manager, manifest, "demo", "", false)
	if err != nil || resolved.Catalog.Name != catalog.BuiltinName {
		t.Fatalf("mainline resolution: %+v, %v", resolved, err)
	}
	resolved, err = Resolve(manager, manifest, "demo", "two", false)
	if err != nil || resolved.Catalog.Name != "two" {
		t.Fatalf("qualified resolution: %+v, %v", resolved, err)
	}
	namespaced := catalog.SkillID(manifest.Catalogs["two"].Source, "demo")
	resolved, err = Resolve(manager, manifest, namespaced, "", false)
	if err != nil || resolved.Catalog.Name != "two" || resolved.Name != "demo" {
		t.Fatalf("namespaced resolution: %+v, %v", resolved, err)
	}
}

func TestResolverListsAmbiguousNonMainlineDefinitions(t *testing.T) {
	t.Parallel()
	manifest := state.NewManifest()
	for _, name := range []string{"one", "two"} {
		root := t.TempDir()
		skill := filepath.Join(root, "skills", "demo")
		if err := os.MkdirAll(skill, 0o755); err != nil {
			t.Fatal(err)
		}
		content := "---\nname: demo\ndescription: Test\n---\n"
		if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		catalogManifest := state.NewManifest()
		catalogManifest.Catalog = &state.CatalogDefinition{
			Name: name, Skills: map[string]state.SkillEntry{"demo": {Path: "skills/demo"}},
		}
		if err := state.SaveManifest(filepath.Join(root, "repertoire.yaml"), catalogManifest); err != nil {
			t.Fatal(err)
		}
		manifest.Catalogs[name] = state.CatalogRegistration{Source: root}
	}
	mainline := t.TempDir()
	otherSkill := filepath.Join(mainline, "skills", "other")
	if err := os.MkdirAll(otherSkill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(otherSkill, "SKILL.md"),
		[]byte("---\nname: other\ndescription: Test\n---\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	mainlineManifest := state.NewManifest()
	mainlineManifest.Catalog = &state.CatalogDefinition{
		Name:   catalog.BuiltinName,
		Skills: map[string]state.SkillEntry{"other": {Path: "skills/other"}},
	}
	if err := state.SaveManifest(filepath.Join(mainline, "repertoire.yaml"), mainlineManifest); err != nil {
		t.Fatal(err)
	}
	manifest.Catalogs[catalog.BuiltinName] = state.CatalogRegistration{Source: mainline}

	manager, err := catalog.NewManager(filepath.Join(t.TempDir(), "cache"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = Resolve(manager, manifest, "demo", "", false)
	if err == nil {
		t.Fatal("expected ambiguous resolution to fail")
	}
	for _, expected := range []string{
		`skill "demo" is defined in multiple catalogs:`,
		"  - one (" + manifest.Catalogs["one"].Source + ")",
		"  - two (" + manifest.Catalogs["two"].Source + ")",
		"specify a catalog with --catalog <name> or a namespaced skill id",
	} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("ambiguity error %q does not contain %q", err, expected)
		}
	}
}

func TestResolverAcceptsQualifiedCatalogSkillKeys(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	skill := filepath.Join(root, "skills", "code")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: phillarmonkey/code\ndescription: Test\n---\n"
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	catalogManifest := state.NewManifest()
	catalogManifest.Catalog = &state.CatalogDefinition{
		Name: "local", Skills: map[string]state.SkillEntry{"phillarmonkey/code": {Path: "skills/code"}},
	}
	if err := state.SaveManifest(filepath.Join(root, "repertoire.yaml"), catalogManifest); err != nil {
		t.Fatal(err)
	}
	manifest := state.NewManifest()
	manifest.Catalogs["local"] = state.CatalogRegistration{Source: root}
	manager, err := catalog.NewManager(filepath.Join(t.TempDir(), "cache"))
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := Resolve(manager, manifest, "phillarmonkey/code", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Name != "phillarmonkey/code" || resolved.Catalog.Name != "local" {
		t.Fatalf("resolved = %+v", resolved)
	}
}

func skillFixture(t *testing.T, name string) string {
	t.Helper()
	return skillFixtureWithDirectory(t, name, name)
}

func skillFixtureWithDirectory(t *testing.T, directoryName, skillName string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), directoryName)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: " + skillName + "\ndescription: Test skill\n---\n\n# Test\n"
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}
