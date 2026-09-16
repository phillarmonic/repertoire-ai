package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitWritesStarterManifestEndToEnd(t *testing.T) {
	binary := testBinaryPath(t)
	moduleRoot := filepath.Clean(filepath.Join("..", ".."))
	runCommand(t, moduleRoot, "go", "build", "-o", binary, "./cmd/repertoire")

	t.Run("fresh repo writes starter without installing", func(t *testing.T) {
		project, home, environment := bootstrapEnvironment(t)
		seedBuiltinCatalogCache(t, home, map[string]string{
			"demo": "demo-v1",
		})
		output := runCommandWithEnv(t, project, environment, binary, "init")
		if !strings.Contains(output, "created repertoire.yaml") ||
			!strings.Contains(output, "repertoire bootstrap") ||
			!strings.Contains(output, initDocsURL) {
			t.Fatalf("init output:\n%s", output)
		}
		content := readFileForTest(t, filepath.Join(project, "repertoire.yaml"))
		if !strings.Contains(content, "schema: 1") ||
			!strings.Contains(content, "tool: https://github.com/phillarmonic/repertoire-ai") ||
			!strings.Contains(content, "github.com/phillarmonic/ai-skills/demo:") ||
			!strings.Contains(content, "scope: global") {
			t.Fatalf("created manifest:\n%s", content)
		}
		if _, err := os.Stat(filepath.Join(home, ".codex", "skills", "demo")); !os.IsNotExist(err) {
			t.Fatalf("init installed a skill: %v", err)
		}
		if _, err := os.Stat(filepath.Join(project, ".agents", "skills", "demo")); !os.IsNotExist(err) {
			t.Fatalf("init installed a project skill: %v", err)
		}
	})

	t.Run("existing skills section is refused and --force rewrites", func(t *testing.T) {
		project, home, environment := bootstrapEnvironment(t)
		seedBuiltinCatalogCache(t, home, map[string]string{
			"demo": "demo-v1",
		})
		if err := os.WriteFile(filepath.Join(project, "repertoire.yaml"), []byte(`schema: 1
catalogs:
  company:
    source: /catalogs/company
requirements:
  leftover:
    catalog: company
    targets: [codex]
skills:
  leftover:
    catalog: company
    scope: project
    targets: [agents]
`), 0o644); err != nil {
			t.Fatal(err)
		}
		output := runCommandWithEnvError(t, project, environment, binary, "init")
		if !strings.Contains(output, "already declares skills") {
			t.Fatalf("refuse output:\n%s", output)
		}
		before := readFileForTest(t, filepath.Join(project, "repertoire.yaml"))
		if !strings.Contains(before, "leftover:") {
			t.Fatalf("refused init changed the manifest:\n%s", before)
		}
		output = runCommandWithEnv(t, project, environment, binary, "init", "--force")
		if !strings.Contains(output, "updated repertoire.yaml") {
			t.Fatalf("force output:\n%s", output)
		}
		after := readFileForTest(t, filepath.Join(project, "repertoire.yaml"))
		if !strings.Contains(after, "github.com/phillarmonic/ai-skills/demo:") ||
			!strings.Contains(after, "source: /catalogs/company") ||
			!strings.Contains(after, "leftover:") ||
			strings.Contains(after, "scope: project") {
			t.Fatalf("forced manifest:\n%s", after)
		}
	})

	t.Run("existing catalogs section survives when skills are missing", func(t *testing.T) {
		project, home, environment := bootstrapEnvironment(t)
		seedBuiltinCatalogCache(t, home, map[string]string{
			"demo": "demo-v1",
		})
		if err := os.WriteFile(filepath.Join(project, "repertoire.yaml"), []byte(`schema: 1
catalogs:
  company:
    source: /catalogs/company
requirements:
  leftover:
    catalog: company
    targets: [codex]
`), 0o644); err != nil {
			t.Fatal(err)
		}
		output := runCommandWithEnv(t, project, environment, binary, "init")
		if !strings.Contains(output, "updated repertoire.yaml") {
			t.Fatalf("init with catalogs:\n%s", output)
		}
		content := readFileForTest(t, filepath.Join(project, "repertoire.yaml"))
		if !strings.Contains(content, "source: /catalogs/company") ||
			!strings.Contains(content, "leftover:") ||
			!strings.Contains(content, "github.com/phillarmonic/ai-skills/demo:") {
			t.Fatalf("preserved manifest:\n%s", content)
		}
	})

	t.Run("global flag is rejected", func(t *testing.T) {
		project, _, environment := bootstrapEnvironment(t)
		output := runCommandWithEnvError(t, project, environment, binary, "init", "--global")
		if !strings.Contains(output, "--global is not supported") {
			t.Fatalf("global flag output:\n%s", output)
		}
	})
}
