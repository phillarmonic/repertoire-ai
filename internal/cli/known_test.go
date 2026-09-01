package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phillarmonic/repertoire-ai/internal/catalog"
	"github.com/phillarmonic/repertoire-ai/internal/state"
)

func TestKnownCatalogsIncludeAlternateScopeBootstrapAndCache(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	t.Setenv("APPDATA", filepath.Join(home, "config"))
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "cache"))

	globalConfig := globalConfigRoot(home)
	if err := os.MkdirAll(globalConfig, 0o755); err != nil {
		t.Fatal(err)
	}
	globalManifest := state.NewManifest()
	globalManifest.Catalogs["company"] = state.CatalogRegistration{Source: "https://github.com/example/company-skills.git"}
	if err := state.SaveManifest(filepath.Join(globalConfig, "repertoire.yaml"), globalManifest); err != nil {
		t.Fatal(err)
	}

	project := t.TempDir()
	if err := exec.Command("git", "-C", project, "init", "-q").Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	writeBootstrapFile(t, project, `schema: 1
catalogs:
  team:
    source: https://github.com/example/team-skills.git
skills:
  demo:
    catalog: team
    scope: global
    targets: [agents]
`)

	legacyProject := t.TempDir()
	if err := exec.Command("git", "-C", legacyProject, "init", "-q").Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	writeLegacyBootstrapFile(t, legacyProject, `schema: 1
catalogs:
  legacy-team:
    source: https://github.com/example/legacy-team-skills.git
skills:
  demo:
    catalog: legacy-team
    scope: global
    targets: [agents]
`)

	cacheParent := filepath.Join(userCacheRoot(home), "repertoire", "catalogs")
	if err := os.MkdirAll(cacheParent, 0o755); err != nil {
		t.Fatal(err)
	}
	cached := createBootstrapCatalog(t, "cached", map[string]string{"demo": "v1"})
	runGit(t, cached, "init", "-q", "-b", "main")
	runGit(t, cached, "config", "user.email", "test@example.test")
	runGit(t, cached, "config", "user.name", "Test")
	runGit(t, cached, "add", ".")
	runGit(t, cached, "commit", "-qm", "seed")
	runCommand(t, cacheParent, "git", "clone", "-q", cached, "cached")

	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	known := knownCatalogs(false, false, "")
	byName := map[string]knownCatalog{}
	for _, item := range known {
		byName[item.Name] = item
	}
	if byName["phillarmonic"].Kind != "built-in" {
		t.Fatalf("builtin = %+v", byName["phillarmonic"])
	}
	if byName["company"].Kind != "registered" || !strings.Contains(byName["company"].Source, "company-skills") {
		t.Fatalf("company = %+v", byName["company"])
	}
	if byName["team"].Kind != "project" {
		t.Fatalf("team = %+v", byName["team"])
	}
	if byName["cached"].Kind != "cached" {
		t.Fatalf("cached = %+v", byName["cached"])
	}

	if err := os.Chdir(legacyProject); err != nil {
		t.Fatal(err)
	}
	legacyKnown := knownCatalogs(false, false, "")
	legacyKind := ""
	for _, item := range legacyKnown {
		if item.Name == "legacy-team" {
			legacyKind = item.Kind
		}
	}
	if legacyKind != "bootstrap" {
		t.Fatalf("legacy-team kind = %q, want bootstrap fallback", legacyKind)
	}
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}

	global, projectFlag := false, false
	sources, _ := completeCatalogSources(&global, &projectFlag)(nil, nil, "github.com/example")
	joined := strings.Join(sources, "\n")
	if !strings.Contains(joined, "github.com/example/company-skills") ||
		!strings.Contains(joined, "github.com/example/team-skills") {
		t.Fatalf("source completions:\n%s", joined)
	}
	builtin, _ := completeCatalogSources(&global, &projectFlag)(nil, nil, "github.com/phillarmonic")
	if len(builtin) == 0 {
		t.Fatal("expected builtin source completion")
	}
}

func TestAvailableSkillCompletionsPreferNamespacedWhenTyped(t *testing.T) {
	local := writeCompletionCatalog(t, t.TempDir(), "local", map[string]string{
		"zensical": "skills/zensical",
	})
	manifest := state.NewManifest()
	manifest.Catalogs["phillarmonic"] = state.CatalogRegistration{Source: local}

	short := availableSkillCompletions(manifest, "", "zen", "", nil)
	if len(short) != 1 || !strings.HasPrefix(short[0], "zensical\t") {
		t.Fatalf("short completions = %#v", short)
	}

	idPrefix := catalog.SkillID(local, "zensical")
	namespaced := availableSkillCompletions(manifest, "", idPrefix[:len(idPrefix)/2], "", nil)
	if len(namespaced) != 1 || !strings.HasPrefix(namespaced[0], idPrefix) {
		// local abs path namespace — ensure prefix match works with dotted/slash input
		namespaced = availableSkillCompletions(manifest, "", strings.Split(idPrefix, "/")[0], "", nil)
		if len(namespaced) == 0 {
			t.Fatalf("namespaced completions = %#v (prefix %q)", namespaced, idPrefix)
		}
	}
}

func TestListCachedCatalogs(t *testing.T) {
	home := t.TempDir()
	cacheRoot := filepath.Join(home, "catalogs")
	if err := os.MkdirAll(cacheRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	remote := createBootstrapCatalog(t, "demo", map[string]string{"skill": "v1"})
	runGit(t, remote, "init", "-q", "-b", "main")
	runGit(t, remote, "config", "user.email", "test@example.test")
	runGit(t, remote, "config", "user.name", "Test")
	runGit(t, remote, "add", ".")
	runGit(t, remote, "commit", "-qm", "seed")
	runCommand(t, cacheRoot, "git", "clone", "-q", remote, "demo")

	manager, err := catalog.NewManager(cacheRoot)
	if err != nil {
		t.Fatal(err)
	}
	cached, err := manager.ListCached()
	if err != nil || len(cached) != 1 || cached[0].Name != "demo" {
		t.Fatalf("ListCached = %+v, %v", cached, err)
	}
	if cached[0].Registration.Source == "" {
		t.Fatal("expected cached source URL")
	}
}
