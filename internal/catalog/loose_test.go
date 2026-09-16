package catalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phillarmonic/repertoire-ai/internal/state"
)

func TestSynthesizeDiscoveryRoots(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSkillMD(t, root, "root-skill", "Root skill")
	writeSkillMD(t, filepath.Join(root, "skills", "demo"), "demo", "Demo skill")
	writeSkillMD(t, filepath.Join(root, "skills", "tools", "linter"), "linter", "Linter skill")
	writeSkillMD(t, filepath.Join(root, "skills", "lang", "go", "gofmt"), "gofmt", "gofmt skill")
	writeSkillMD(t, filepath.Join(root, ".agents", "skills", "agents-demo"), "agents-demo", "Agents skill")
	writeSkillMD(t, filepath.Join(root, "plugins", "plugin-dev", "skills", "skill-development"), "skill-development", "Plugin skill")
	writeSkillMD(t, filepath.Join(root, "external_plugins", "discord", "skills", "channel-access"), "channel-access", "Channel skill")
	tooDeep := filepath.Join(root, "plugins", "a", "b", "c", "skills", "buried")
	writeSkillMD(t, tooDeep, "buried", "Too deep")

	manifest, err := Synthesize(root)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"root-skill":        ".",
		"demo":              "skills/demo",
		"linter":            "skills/tools/linter",
		"gofmt":             "skills/lang/go/gofmt",
		"agents-demo":       ".agents/skills/agents-demo",
		"skill-development": "plugins/plugin-dev/skills/skill-development",
		"channel-access":    "external_plugins/discord/skills/channel-access",
	}
	if len(manifest.Catalog.Skills) != len(want) {
		t.Fatalf("skills = %#v", manifest.Catalog.Skills)
	}
	for name, path := range want {
		entry, ok := manifest.Catalog.Skills[name]
		if !ok || entry.Path != path {
			t.Fatalf("skill %q = %+v, want path %q", name, entry, path)
		}
	}
	if _, ok := manifest.Catalog.Skills["buried"]; ok {
		t.Fatal("discovered a skill beyond max depth")
	}
}

func TestSynthesizeDuplicateNames(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSkillMD(t, filepath.Join(root, "skills", "access"), "access", "First")
	writeSkillMD(t, filepath.Join(root, "external_plugins", "telegram", "skills", "access"), "access", "Second")
	_, err := Synthesize(root)
	if err == nil || !strings.Contains(err.Error(), `duplicate skill name "access"`) {
		t.Fatalf("duplicate error = %v", err)
	}
	if !strings.Contains(err.Error(), "skills/access") ||
		!strings.Contains(err.Error(), "external_plugins/telegram/skills/access") {
		t.Fatalf("duplicate error did not name both paths: %v", err)
	}
}

func TestSynthesizeZeroSkills(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	_, err := Synthesize(root)
	if err == nil || !strings.Contains(err.Error(), "no repertoire.yaml and no SKILL.md directories found under") {
		t.Fatalf("zero-skill error = %v", err)
	}
}

func TestSynthesizeIgnoresEscapingSymlink(t *testing.T) {
	t.Parallel()
	outside := t.TempDir()
	writeSkillMD(t, filepath.Join(outside, "escaped"), "escaped", "Outside skill")
	root := t.TempDir()
	writeSkillMD(t, filepath.Join(root, "skills", "kept"), "kept", "Inside skill")
	if err := os.MkdirAll(filepath.Join(root, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "escaped"), filepath.Join(root, "skills", "escaped")); err != nil {
		t.Skip("symlinks are unavailable")
	}
	manifest, err := Synthesize(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := manifest.Catalog.Skills["escaped"]; ok {
		t.Fatal("followed a symlink out of the catalog root")
	}
	if _, ok := manifest.Catalog.Skills["kept"]; !ok {
		t.Fatal("missing in-tree skill")
	}
}

func TestLoadCatalogFallsBackWhenManifestHasNoCatalogSection(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSkillMD(t, filepath.Join(root, "skills", "demo"), "demo", "Demo skill")
	if err := os.WriteFile(filepath.Join(root, "repertoire.yaml"), []byte("schema: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest, loose, err := loadCatalog(root)
	if err != nil || !loose {
		t.Fatalf("loadCatalog = %+v loose=%v err=%v", manifest, loose, err)
	}
	if _, ok := manifest.Catalog.Skills["demo"]; !ok {
		t.Fatalf("skills = %#v", manifest.Catalog.Skills)
	}
}

func TestLoadCatalogMalformedManifestDoesNotSynthesize(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSkillMD(t, filepath.Join(root, "skills", "demo"), "demo", "Demo skill")
	if err := os.WriteFile(filepath.Join(root, "repertoire.yaml"), []byte("schema: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, loose, err := loadCatalog(root)
	if err == nil || loose {
		t.Fatalf("expected malformed catalog error, loose=%v err=%v", loose, err)
	}
	if !strings.Contains(err.Error(), "load catalog at") {
		t.Fatalf("error = %v", err)
	}
}

func TestMaterializeLooseSetsRegistrationNameAndCommit(t *testing.T) {
	t.Parallel()
	repository := t.TempDir()
	run(t, repository, "init", "-q", "-b", "main")
	run(t, repository, "config", "user.email", "test@example.test")
	run(t, repository, "config", "user.name", "Test")
	writeSkillMD(t, filepath.Join(repository, "skills", "demo"), "demo", "Demo skill")
	run(t, repository, "add", ".")
	run(t, repository, "commit", "-qm", "initial")

	manager, err := NewManager(filepath.Join(t.TempDir(), "cache"))
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := manager.Materialize(Source{
		Name: "official", Registration: state.CatalogRegistration{Source: repository},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !resolved.Loose || resolved.Manifest.Catalog.Name != "official" || resolved.Commit == "" {
		t.Fatalf("unexpected loose materialization: %+v", resolved)
	}
	cached, err := manager.InspectCached(Source{
		Name: "official", Registration: state.CatalogRegistration{Source: repository},
	})
	if err != nil || !cached.Loose {
		t.Fatalf("InspectCached = %+v, %v", cached, err)
	}
}

func writeSkillMD(t *testing.T, directory, name, description string) {
	t.Helper()
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: " + name + "\ndescription: " + description + "\n---\n"
	if err := os.WriteFile(filepath.Join(directory, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
