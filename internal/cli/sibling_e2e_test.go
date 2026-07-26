package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const siblingCatalog = "/Users/andy/repos/phillarmonic/ai-skills"

func TestSiblingCatalogProjectAndGlobalWorkflows(t *testing.T) {
	if _, err := os.Stat(filepath.Join(siblingCatalog, "repertoire.yaml")); err != nil {
		t.Skipf("sibling catalog unavailable: %v", err)
	}
	binary := filepath.Join(t.TempDir(), "repertoire")
	runCommand(t, filepath.Clean(filepath.Join("..", "..")), "go", "build", "-o", binary, "./cmd/repertoire")

	project := t.TempDir()
	runCommand(t, project, "git", "init", "-q")
	runCommand(t, project, binary, "catalog", "add", siblingCatalog, "--name", "phillarmonic", "--force")
	runCommand(t, project, binary, "add", "zensical", "--catalog", "phillarmonic", "--target", "agents")
	runCommand(t, project, binary, "update", "zensical")
	runCommand(t, project, binary, "remove", "zensical")

	home := t.TempDir()
	environment := append(os.Environ(),
		"HOME="+home,
		"XDG_CONFIG_HOME="+filepath.Join(home, ".config"),
		"XDG_CACHE_HOME="+filepath.Join(home, ".cache"),
		"CODEX_HOME="+filepath.Join(home, ".codex"),
	)
	runCommandWithEnv(t, t.TempDir(), environment, binary, "--global", "catalog", "add", siblingCatalog, "--name", "phillarmonic", "--force")
	runCommandWithEnv(t, t.TempDir(), environment, binary, "--global", "install", "zensical", "--catalog", "phillarmonic", "--target", "agents")
	output := runCommandWithEnv(t, t.TempDir(), environment, binary, "--global", "list")
	if !strings.Contains(output, "zensical\tphillarmonic\tad-hoc\tagents") {
		t.Fatalf("unexpected global list: %s", output)
	}
	runCommandWithEnv(t, t.TempDir(), environment, binary, "--global", "remove", "zensical")
	if _, err := os.Stat(filepath.Join(home, ".agents", "skills", "zensical")); !os.IsNotExist(err) {
		t.Fatalf("global target was not removed: %v", err)
	}
}
