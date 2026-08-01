package doctor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	installer "github.com/phillarmonic/repertoire-ai/internal/install"
	"github.com/phillarmonic/repertoire-ai/internal/state"
)

func doctorEnvironment(t *testing.T, resolved installer.ResolvedSkill) *Env {
	t.Helper()
	project := t.TempDir()
	global := t.TempDir()
	return &Env{
		ProjectRoot:     project,
		HasProject:      true,
		ProjectScope:    state.Scope{Root: project, ManifestPath: filepath.Join(project, "repertoire.yaml"), LockPath: filepath.Join(project, "repertoire.lock.json")},
		GlobalScope:     state.Scope{Global: true, Root: global, ManifestPath: filepath.Join(global, "repertoire.yaml"), LockPath: filepath.Join(global, "repertoire.lock.json")},
		ProjectManifest: state.NewManifest(),
		GlobalManifest:  state.NewManifest(),
		ProjectLock:     state.NewLock(),
		GlobalLock:      state.NewLock(),
		ResolveSkill: func(state.Manifest, string, string) (installer.ResolvedSkill, error) {
			return resolved, nil
		},
	}
}

func markdownFixture(t *testing.T) installer.ResolvedSkill {
	t.Helper()
	source := filepath.Join(t.TempDir(), "agents.md")
	if err := os.WriteFile(source, []byte("Demo guidance v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return installer.ResolvedSkill{
		Name: "demo",
		Instructions: map[string][]installer.ResolvedArtifact{
			"agents": {{
				ArtifactEntry: state.ArtifactEntry{
					ID: "guidance", Source: "agents.md", Destination: "AGENTS.md", Mode: state.ArtifactModeMarkdownSection,
				},
				SourcePath: source,
			}},
		},
		Artifacts: map[string][]installer.ResolvedArtifact{},
	}
}

// installDemoArtifact runs the real installer for the fixture and locks the
// result in the project lock, giving every test a truthful starting state.
func installDemoArtifact(t *testing.T, env *Env, resolved installer.ResolvedSkill) {
	t.Helper()
	selected := installer.ProjectArtifacts(resolved, false)
	artifacts, err := installer.InstallArtifacts(selected, []installer.Target{{Name: "agents"}}, env.ProjectRoot, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	env.ProjectLock.Skills["demo"] = state.LockSkill{
		Catalog: "test", Targets: []string{"agents"}, Artifacts: artifacts,
	}
	if err := env.saveProjectLock(); err != nil {
		t.Fatal(err)
	}
}

func TestDoctorMissingDestination(t *testing.T) {
	resolved := markdownFixture(t)
	env := doctorEnvironment(t, resolved)
	installDemoArtifact(t, env, resolved)
	destination := filepath.Join(env.ProjectRoot, "AGENTS.md")
	if err := os.Remove(destination); err != nil {
		t.Fatal(err)
	}

	report, err := Run(env, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Issues) != 1 || report.Issues[0].Check != "missing-destination" {
		t.Fatalf("report = %+v", report.Issues)
	}

	fixed, err := Run(env, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixed.Issues) != 0 || len(fixed.Fixed) != 1 {
		t.Fatalf("fix = %+v / %+v", fixed.Fixed, fixed.Issues)
	}
	content, err := os.ReadFile(destination)
	if err != nil || !strings.Contains(string(content), "Demo guidance v1") {
		t.Fatalf("restored destination: %v\n%s", err, content)
	}
}

func TestDoctorModifiedSection(t *testing.T) {
	resolved := markdownFixture(t)
	env := doctorEnvironment(t, resolved)
	installDemoArtifact(t, env, resolved)
	destination := filepath.Join(env.ProjectRoot, "AGENTS.md")
	content, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte(strings.Replace(string(content), "Demo guidance v1", "local edit", 1)), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Run(env, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Issues) != 1 || report.Issues[0].Check != "modified-managed-content" {
		t.Fatalf("report = %+v", report.Issues)
	}

	fixed, err := Run(env, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixed.Issues) != 0 {
		t.Fatalf("residual = %+v", fixed.Issues)
	}
	restored, err := os.ReadFile(destination)
	if err != nil || !strings.Contains(string(restored), "Demo guidance v1") || strings.Contains(string(restored), "local edit") {
		t.Fatalf("restored destination: %v\n%s", err, restored)
	}
}

func TestDoctorOrphanedMarkers(t *testing.T) {
	resolved := markdownFixture(t)
	env := doctorEnvironment(t, resolved)
	installDemoArtifact(t, env, resolved)
	destination := filepath.Join(env.ProjectRoot, "AGENTS.md")
	content, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	orphan := "\n<!-- repertoire:ghost:codex:x:start -->\nBoo\n<!-- repertoire:ghost:codex:x:end -->\n"
	if err := os.WriteFile(destination, append(content, []byte(orphan)...), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Run(env, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Issues) != 1 || report.Issues[0].Check != "orphaned-markers" {
		t.Fatalf("report = %+v", report.Issues)
	}

	fixed, err := Run(env, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixed.Issues) != 0 {
		t.Fatalf("residual = %+v", fixed.Issues)
	}
	restored, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("fix must never delete the destination: %v", err)
	}
	if strings.Contains(string(restored), "ghost") || !strings.Contains(string(restored), "Demo guidance v1") {
		t.Fatalf("restored destination:\n%s", restored)
	}
}

func TestDoctorDuplicateSectionsCollapse(t *testing.T) {
	resolved := markdownFixture(t)
	env := doctorEnvironment(t, resolved)
	destination := filepath.Join(env.ProjectRoot, "AGENTS.md")
	var content strings.Builder
	content.WriteString("# User instructions\n")
	var artifacts []state.LockArtifact
	for _, target := range []string{"codex", "cursor", "opencode"} {
		marker := "repertoire:demo:" + target + ":guidance"
		content.WriteString("\n")
		content.Write(installer.RenderMarkedSection(marker, []byte("Demo guidance v1\n")))
	}
	if err := os.WriteFile(destination, []byte(content.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"codex", "cursor", "opencode"} {
		marker := "repertoire:demo:" + target + ":guidance"
		digest, found, err := installer.MarkdownSectionDigest(destination, marker)
		if err != nil || !found {
			t.Fatalf("digest %s: %v", marker, err)
		}
		artifacts = append(artifacts, state.LockArtifact{
			ID: "guidance", Target: target, Destination: destination,
			Mode: state.ArtifactModeMarkdownSection, Marker: marker, Digest: digest,
			MarkdownSeparator: "\n",
		})
	}
	env.ProjectLock.Skills["demo"] = state.LockSkill{
		Catalog: "test", Targets: []string{"codex", "cursor", "opencode"}, Artifacts: artifacts,
	}
	if err := env.saveProjectLock(); err != nil {
		t.Fatal(err)
	}

	report, err := Run(env, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Issues) != 1 || report.Issues[0].Check != "duplicate-sections" {
		t.Fatalf("report = %+v", report.Issues)
	}

	fixed, err := Run(env, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixed.Issues) != 0 || len(fixed.Fixed) != 1 {
		t.Fatalf("fix = %+v / %+v", fixed.Fixed, fixed.Issues)
	}
	collapsed, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(collapsed), ":start -->") != 1 ||
		!strings.Contains(string(collapsed), "repertoire:demo:all:guidance:start") {
		t.Fatalf("collapsed destination:\n%s", collapsed)
	}
	if !strings.HasPrefix(string(collapsed), "# User instructions\n") {
		t.Fatalf("user content missing:\n%s", collapsed)
	}
	lock, err := state.LoadLock(env.ProjectScope.LockPath)
	if err != nil {
		t.Fatal(err)
	}
	locked := lock.Skills["demo"].Artifacts
	if len(locked) != 1 || locked[0].Target != "all" || locked[0].Marker != "repertoire:demo:all:guidance" {
		t.Fatalf("locked artifacts = %+v", locked)
	}
}

func TestDoctorDuplicateWithLocallyModifiedMemberStays(t *testing.T) {
	resolved := markdownFixture(t)
	env := doctorEnvironment(t, resolved)
	destination := filepath.Join(env.ProjectRoot, "AGENTS.md")
	var content strings.Builder
	for _, target := range []string{"codex", "cursor"} {
		marker := "repertoire:demo:" + target + ":guidance"
		if content.Len() > 0 {
			content.WriteString("\n")
		}
		content.Write(installer.RenderMarkedSection(marker, []byte("Demo guidance v1\n")))
	}
	if err := os.WriteFile(destination, []byte(content.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	var artifacts []state.LockArtifact
	for _, target := range []string{"codex", "cursor"} {
		marker := "repertoire:demo:" + target + ":guidance"
		digest, found, err := installer.MarkdownSectionDigest(destination, marker)
		if err != nil || !found {
			t.Fatalf("digest %s: %v", marker, err)
		}
		artifacts = append(artifacts, state.LockArtifact{
			ID: "guidance", Target: target, Destination: destination,
			Mode: state.ArtifactModeMarkdownSection, Marker: marker, Digest: digest,
		})
	}
	env.ProjectLock.Skills["demo"] = state.LockSkill{Catalog: "test", Targets: []string{"codex", "cursor"}, Artifacts: artifacts}
	if err := env.saveProjectLock(); err != nil {
		t.Fatal(err)
	}
	// Locally modify the cursor section after locking.
	current, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(current), "Demo guidance v1", "local edit", 1)
	if err := os.WriteFile(destination, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Run(env, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, issue := range report.Issues {
		if issue.Check == "duplicate-sections" {
			t.Fatalf("group with a locally modified member is not a duplicate: %+v", report.Issues)
		}
	}
	if len(report.Issues) != 1 || report.Issues[0].Check != "modified-managed-content" {
		t.Fatalf("modified member not reported: %+v", report.Issues)
	}
}

func TestDoctorStaleProjectEntries(t *testing.T) {
	env := doctorEnvironment(t, installer.ResolvedSkill{})
	env.GlobalLock.Projects[t.TempDir()] = map[string]state.LockProjectArtifacts{
		"demo": {Catalog: "test"},
	}
	vanished := filepath.Join(env.ProjectRoot, "does-not-exist")
	env.GlobalLock.Projects[vanished] = map[string]state.LockProjectArtifacts{"demo": {Catalog: "test"}}
	if err := env.saveGlobalLock(); err != nil {
		t.Fatal(err)
	}

	report, err := Run(env, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Issues) != 1 || report.Issues[0].Check != "stale-project-entries" || report.Issues[0].Subject != vanished {
		t.Fatalf("report = %+v", report.Issues)
	}

	fixed, err := Run(env, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixed.Issues) != 0 {
		t.Fatalf("residual = %+v", fixed.Issues)
	}
	lock, err := state.LoadLock(env.GlobalScope.LockPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := lock.Projects[vanished]; exists {
		t.Fatalf("stale project entry remains: %+v", lock.Projects)
	}
}

func TestDoctorGlobalSkillHealth(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	skillRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(skillRoot, "SKILL.md"), []byte("---\nname: demo\ndescription: Test\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	digest, err := installer.Digest(skillRoot)
	if err != nil {
		t.Fatal(err)
	}
	resolved := installer.ResolvedSkill{Name: "demo", Root: skillRoot, Digest: digest}
	env := doctorEnvironment(t, resolved)
	targets := []installer.Target{{Name: "agents", Root: filepath.Join(home, ".agents", "skills")}}
	locations, digests, err := installer.SkillWithDigests(resolved, targets, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	env.GlobalLock.Skills["demo"] = state.LockSkill{
		Catalog: "test", Targets: []string{"agents"}, Locations: locations, TargetDigests: digests, Digest: digest,
	}
	if err := env.saveGlobalLock(); err != nil {
		t.Fatal(err)
	}

	healthy, err := Run(env, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(healthy.Issues) != 0 {
		t.Fatalf("healthy report = %+v", healthy.Issues)
	}

	if err := os.RemoveAll(locations[0]); err != nil {
		t.Fatal(err)
	}
	report, err := Run(env, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Issues) != 1 || report.Issues[0].Check != "global-skill-health" {
		t.Fatalf("report = %+v", report.Issues)
	}

	fixed, err := Run(env, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixed.Issues) != 0 {
		t.Fatalf("residual = %+v", fixed.Issues)
	}
	if _, err := os.Stat(filepath.Join(home, ".agents", "skills", "demo", "SKILL.md")); err != nil {
		t.Fatalf("reinstalled skill missing: %v", err)
	}
}

func TestDoctorManifestDrift(t *testing.T) {
	env := doctorEnvironment(t, installer.ResolvedSkill{})
	env.ProjectManifest.Skills["declared-demo"] = state.BootstrapSkill{Scope: state.BootstrapScopeProject}
	env.ProjectManifest.Requirements["missing-requirement"] = state.Requirement{Catalog: "test"}

	report, err := Run(env, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Issues) != 2 {
		t.Fatalf("report = %+v", report.Issues)
	}
	remedies := map[string]string{}
	for _, issue := range report.Issues {
		if issue.Check != "manifest-drift" {
			t.Fatalf("unexpected check: %+v", issue)
		}
		remedies[issue.Subject] = issue.Remedy
	}
	if remedies["declared-demo"] != "run repertoire sync" || remedies["missing-requirement"] != "run repertoire install" {
		t.Fatalf("remedies = %+v", remedies)
	}

	// Doctor itself never installs; drift stays residual after a fix run so
	// the CLI can reconcile it through the sync path.
	fixed, err := Run(env, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixed.Issues) != 2 {
		t.Fatalf("residual = %+v", fixed.Issues)
	}
}

func TestDoctorGlobalSkillWithoutPointersIsNotDrift(t *testing.T) {
	// A global-scope bootstrap skill that declares no project instructions or
	// hooks never gains per-project lock pointers; that state is healthy.
	env := doctorEnvironment(t, installer.ResolvedSkill{Name: "quiet-demo"})
	env.ProjectManifest.Skills["quiet-demo"] = state.BootstrapSkill{Scope: state.BootstrapScopeGlobal}
	env.GlobalLock.Skills["quiet-demo"] = state.LockSkill{Catalog: "test", Targets: []string{"agents"}}

	report, err := Run(env, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Issues) != 0 {
		t.Fatalf("report = %+v", report.Issues)
	}
}

func TestDoctorConflictingDestination(t *testing.T) {
	env := doctorEnvironment(t, installer.ResolvedSkill{})
	destination := filepath.Join(env.ProjectRoot, ".claude", "CLAUDE.md")
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	// The file currently holds skill-a's content; skill-b's lock entry can
	// never match while both copy-manage the same destination.
	if err := os.WriteFile(destination, []byte("skill-a pointer\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	env.ResolveSkill = func(_ state.Manifest, name, _ string) (installer.ResolvedSkill, error) {
		digest := "digest-a"
		if name == "skill-b" {
			digest = "digest-b"
		}
		return installer.ResolvedSkill{
			Name: name,
			Instructions: map[string][]installer.ResolvedArtifact{
				"claude": {{
					ArtifactEntry: state.ArtifactEntry{
						ID: "claude-registration", Source: "claude.md", Destination: ".claude/CLAUDE.md", Mode: state.ArtifactModeCopy,
					},
					Digest: digest,
				}},
			},
			Artifacts: map[string][]installer.ResolvedArtifact{},
		}, nil
	}
	entry := func(skill, digest string) {
		env.ProjectLock.Skills[skill] = state.LockSkill{
			Catalog: "test", Targets: []string{"claude"},
			Artifacts: []state.LockArtifact{{
				ID: "claude-registration", Target: "claude", Destination: destination,
				Mode: state.ArtifactModeCopy, Digest: digest,
			}},
		}
	}
	entry("skill-a", "digest-a")
	entry("skill-b", "digest-b")
	if err := env.saveProjectLock(); err != nil {
		t.Fatal(err)
	}

	report, err := Run(env, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Issues) != 1 || report.Issues[0].Check != "conflicting-destination" {
		t.Fatalf("report = %+v", report.Issues)
	}
	if !strings.Contains(report.Issues[0].Detail, "skill-a") || !strings.Contains(report.Issues[0].Detail, "skill-b") {
		t.Fatalf("conflict detail = %+v", report.Issues[0])
	}

	// The catalogs still conflict, so --fix must not reinstall either side;
	// the conflict stays reported and no modification flap appears.
	fixed, err := Run(env, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixed.Fixed) != 0 || len(fixed.Issues) != 1 || fixed.Issues[0].Check != "conflicting-destination" {
		t.Fatalf("fix = %+v / %+v", fixed.Fixed, fixed.Issues)
	}
}

func TestDoctorResolvableConflictMigratesLocks(t *testing.T) {
	sources := t.TempDir()
	writeSource := func(name, content string) string {
		path := filepath.Join(sources, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	sourcesBySkill := map[string]string{
		"skill-a": writeSource("a.md", "Skill A pointer\n"),
		"skill-b": writeSource("b.md", "Skill B pointer\n"),
	}
	env := doctorEnvironment(t, installer.ResolvedSkill{})
	env.ResolveSkill = func(_ state.Manifest, name, _ string) (installer.ResolvedSkill, error) {
		return installer.ResolvedSkill{
			Name: name,
			Instructions: map[string][]installer.ResolvedArtifact{
				"claude": {{
					ArtifactEntry: state.ArtifactEntry{
						ID: "claude-registration", Source: name + ".md", Destination: ".claude/CLAUDE.md", Mode: state.ArtifactModeMarkdownSection,
					},
					SourcePath: sourcesBySkill[name],
				}},
			},
			Artifacts: map[string][]installer.ResolvedArtifact{},
		}, nil
	}
	destination := filepath.Join(env.ProjectRoot, ".claude", "CLAUDE.md")
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("Skill A pointer\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The lock predates the catalog fix: both skills copy-manage the file.
	for i, skill := range []string{"skill-a", "skill-b"} {
		env.ProjectLock.Skills[skill] = state.LockSkill{
			Catalog: "test", Targets: []string{"claude"},
			Artifacts: []state.LockArtifact{{
				ID: "claude-registration", Target: "claude", Destination: destination,
				Mode: state.ArtifactModeCopy, Digest: fmt.Sprintf("old-digest-%d", i),
			}},
		}
	}
	if err := env.saveProjectLock(); err != nil {
		t.Fatal(err)
	}

	report, err := Run(env, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Issues) != 1 || report.Issues[0].Check != "conflicting-destination" {
		t.Fatalf("report = %+v", report.Issues)
	}

	fixed, err := Run(env, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixed.Issues) != 0 || len(fixed.Fixed) == 0 {
		t.Fatalf("fix = %+v / %+v", fixed.Fixed, fixed.Issues)
	}
	content, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Skill A pointer", "Skill B pointer",
		"repertoire:skill-a:claude:claude-registration:start",
		"repertoire:skill-b:claude:claude-registration:start",
	} {
		if !strings.Contains(string(content), want) {
			t.Fatalf("migrated destination missing %q:\n%s", want, content)
		}
	}
	lock, err := state.LoadLock(env.ProjectScope.LockPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, skill := range []string{"skill-a", "skill-b"} {
		artifacts := lock.Skills[skill].Artifacts
		if len(artifacts) != 1 || artifacts[0].Mode != state.ArtifactModeMarkdownSection {
			t.Fatalf("%s artifacts = %+v", skill, artifacts)
		}
	}
}

func TestDoctorSameContentCopiesAreNotConflicts(t *testing.T) {
	env := doctorEnvironment(t, installer.ResolvedSkill{})
	destination := filepath.Join(env.ProjectRoot, "shared.md")
	if err := os.WriteFile(destination, []byte("same\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	digest, err := installer.DigestFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	for _, skill := range []string{"skill-a", "skill-b"} {
		env.ProjectLock.Skills[skill] = state.LockSkill{
			Catalog: "test",
			Artifacts: []state.LockArtifact{{
				ID: "shared", Target: "claude", Destination: destination,
				Mode: state.ArtifactModeCopy, Digest: digest,
			}},
		}
	}
	report, err := Run(env, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Issues) != 0 {
		t.Fatalf("identical copies must not conflict: %+v", report.Issues)
	}
}

func TestDoctorSkipsProjectChecksOutsideWorktree(t *testing.T) {
	env := doctorEnvironment(t, installer.ResolvedSkill{})
	env.HasProject = false
	env.ProjectRoot = ""
	env.ProjectLock.Skills["demo"] = state.LockSkill{
		Catalog:   "test",
		Artifacts: []state.LockArtifact{{ID: "guidance", Destination: filepath.Join(t.TempDir(), "missing.md"), Mode: state.ArtifactModeMarkdownSection}},
	}
	report, err := Run(env, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Issues) != 0 {
		t.Fatalf("project checks must be skipped: %+v", report.Issues)
	}
}

var errFixture = errors.New("no catalog in tests")

func TestDoctorScanToleratesUnresolvableDeclarations(t *testing.T) {
	env := doctorEnvironment(t, installer.ResolvedSkill{})
	env.ResolveSkill = func(state.Manifest, string, string) (installer.ResolvedSkill, error) {
		return installer.ResolvedSkill{}, errFixture
	}
	env.ProjectManifest.Skills["ghost"] = state.BootstrapSkill{Scope: state.BootstrapScopeProject}
	// Drift is reported, but scanning for sections must not fail.
	report, err := Run(env, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Issues) != 1 || report.Issues[0].Check != "manifest-drift" {
		t.Fatalf("report = %+v", report.Issues)
	}
}
