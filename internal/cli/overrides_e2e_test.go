package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestLocalCatalogOverrideEndToEnd verifies that a remote catalog source can be
// redirected to a local checkout via REPERTOIRE_OVERRIDES or --override, so a
// skill repository can be tested without pushing first.
func TestLocalCatalogOverrideEndToEnd(t *testing.T) {
	binary := testBinaryPath(t)
	moduleRoot := filepath.Clean(filepath.Join("..", ".."))
	runCommand(t, moduleRoot, "go", "build", "-o", binary, "./cmd/repertoire")

	t.Run("env var overrides a remote catalog", func(t *testing.T) {
		project, home, environment := bootstrapEnvironment(t)
		catalogRoot := createBootstrapCatalog(t, "local", map[string]string{
			"demo": "from-local-override",
		})
		writeBootstrapFile(t, project, `schema: 1
catalogs:
  local:
    source: https://example.invalid/skills.git
skills:
  demo:
    catalog: local
    targets: [agents]
`)
		environment = append(environment, "REPERTOIRE_OVERRIDES=local="+catalogRoot)
		output := runCommandWithEnv(t, project, environment, binary, "bootstrap")
		if !strings.Contains(output, "bootstrapped demo (global)") {
			t.Fatalf("bootstrap output:\n%s", output)
		}
		skill := filepath.Join(home, ".agents", "skills", "demo", "SKILL.md")
		content := readFileForTest(t, skill)
		if !strings.Contains(content, "from-local-override") {
			t.Fatalf("installed skill does not carry override content:\n%s", content)
		}
	})

	t.Run("--override flag beats env and resolves local", func(t *testing.T) {
		project, home, environment := bootstrapEnvironment(t)
		catalogRoot := createBootstrapCatalog(t, "local", map[string]string{
			"demo": "from-flag-override",
		})
		writeBootstrapFile(t, project, `schema: 1
catalogs:
  local:
    source: https://example.invalid/skills.git
skills:
  demo:
    catalog: local
    targets: [agents]
`)
		// A conflicting env override must lose to the flag.
		environment = append(environment, "REPERTOIRE_OVERRIDES=local="+filepath.Join(t.TempDir(), "elsewhere"))
		output := runCommandWithEnv(t, project, environment, binary, "--override", "local="+catalogRoot, "bootstrap")
		if !strings.Contains(output, "bootstrapped demo (global)") {
			t.Fatalf("bootstrap output:\n%s", output)
		}
		skill := filepath.Join(home, ".agents", "skills", "demo", "SKILL.md")
		content := readFileForTest(t, skill)
		if !strings.Contains(content, "from-flag-override") {
			t.Fatalf("installed skill does not carry flag override content:\n%s", content)
		}
	})

	t.Run("list --available resolves skills from the override", func(t *testing.T) {
		project, _, environment := bootstrapEnvironment(t)
		catalogRoot := createBootstrapCatalog(t, "local", map[string]string{
			"local-only": "visible via override",
		})
		writeBootstrapFile(t, project, `schema: 1
catalogs:
  local:
    source: https://example.invalid/skills.git
`)
		environment = append(environment, "REPERTOIRE_OVERRIDES=local="+catalogRoot)
		output := runCommandWithEnv(t, project, environment, binary, "--project", "list", "--available")
		if !strings.Contains(output, "local-only\tlocal") {
			t.Fatalf("list --available output:\n%s", output)
		}
	})

	t.Run("catalog list marks overridden sources", func(t *testing.T) {
		project, _, environment := bootstrapEnvironment(t)
		writeBootstrapFile(t, project, "schema: 1\n")
		environment = append(environment, "REPERTOIRE_OVERRIDES=phillarmonic="+t.TempDir())
		output := runCommandWithEnv(t, project, environment, binary, "catalog", "list")
		if !strings.Contains(output, "overridden ->") {
			t.Fatalf("catalog list does not mark the override:\n%s", output)
		}
	})

	t.Run("invalid override flags are rejected", func(t *testing.T) {
		project, _, environment := bootstrapEnvironment(t)
		writeBootstrapFile(t, project, "schema: 1\n")
		output := runCommandWithEnvError(t, project, environment, binary, "--override", "noequals", "list")
		if !strings.Contains(output, "invalid --override") {
			t.Fatalf("expected invalid --override error, got:\n%s", output)
		}
	})

	t.Run("missing override path fails with a clear error", func(t *testing.T) {
		project, _, environment := bootstrapEnvironment(t)
		catalogRoot := createBootstrapCatalog(t, "local", map[string]string{"demo": "x"})
		writeBootstrapFile(t, project, `schema: 1
catalogs:
  local:
    source: https://example.invalid/skills.git
skills:
  demo:
    catalog: local
    targets: [agents]
`)
		_ = catalogRoot // the override points at a path that does not exist
		environment = append(environment, "REPERTOIRE_OVERRIDES=local="+filepath.Join(project, "missing-checkout"))
		output := runCommandWithEnvError(t, project, environment, binary, "bootstrap")
		if !strings.Contains(output, "override") {
			t.Fatalf("expected override error, got:\n%s", output)
		}
	})
}
