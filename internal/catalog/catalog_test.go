package catalog

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phillarmonic/repertoire-ai/internal/state"
)

func TestSourceUtilitiesAndBuiltinOverride(t *testing.T) {
	t.Parallel()
	if got := NormalizeSource("github.com/example/skills"); got != "https://github.com/example/skills.git" {
		t.Fatalf("unexpected normalized source %q", got)
	}
	if got := RedactSource("https://token@example.test/org/repo.git"); strings.Contains(got, "token") {
		t.Fatalf("credential was not redacted: %q", got)
	}
	manifest := state.NewManifest()
	if sources := Sources(manifest); len(sources) != 1 || !sources[0].Builtin {
		t.Fatalf("expected builtin source: %+v", sources)
	}
	manifest.Catalogs[BuiltinName] = state.CatalogRegistration{Source: "/local"}
	if sources := Sources(manifest); len(sources) != 1 || sources[0].Builtin {
		t.Fatalf("expected explicit override: %+v", sources)
	}
}

func TestMaterializeLocalAndTrackingGitCatalog(t *testing.T) {
	t.Parallel()
	repository := t.TempDir()
	run(t, repository, "init", "-q", "-b", "main")
	run(t, repository, "config", "user.email", "test@example.test")
	run(t, repository, "config", "user.name", "Test")
	writeCatalog(t, repository)
	run(t, repository, "add", ".")
	run(t, repository, "commit", "-qm", "initial")
	run(t, repository, "tag", "v1")

	manager, err := NewManager(filepath.Join(t.TempDir(), "cache"))
	if err != nil {
		t.Fatal(err)
	}
	local, err := manager.Materialize(Source{
		Name: "local", Registration: state.CatalogRegistration{Source: repository},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if local.Manifest.Catalog.Name != "example" || local.Commit == "" {
		t.Fatalf("unexpected local materialization: %+v", local)
	}

	remote, err := manager.Materialize(Source{
		Name: "remote", Registration: state.CatalogRegistration{Source: "file://" + repository, Ref: "v1"},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if remote.Tracking {
		t.Fatal("tag should be immutable")
	}

	tracking, err := manager.Materialize(Source{
		Name: "tracking", Registration: state.CatalogRegistration{Source: "file://" + repository, Ref: "main"},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	firstCommit := tracking.Commit
	if writeErr := os.WriteFile(filepath.Join(repository, "change.txt"), []byte("next"), 0o644); writeErr != nil {
		t.Fatal(writeErr)
	}
	run(t, repository, "add", "change.txt")
	run(t, repository, "commit", "-qm", "next")
	tracking, err = manager.Materialize(Source{
		Name: "tracking", Registration: state.CatalogRegistration{Source: "file://" + repository, Ref: "main"},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !tracking.Tracking || tracking.Commit == firstCommit {
		t.Fatalf("tracking branch did not advance: %+v", tracking)
	}
}

func writeCatalog(t *testing.T, root string) {
	t.Helper()
	content := "schema: 1\ncatalog:\n  name: example\n  skills:\n    demo:\n      path: skills/demo\n"
	if err := os.WriteFile(filepath.Join(root, "repertoire.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func run(t *testing.T, root string, arguments ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, arguments...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", arguments, err, output)
	}
}
