package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phillarmonic/repertoire-ai/internal/catalog"
	installer "github.com/phillarmonic/repertoire-ai/internal/install"
	"github.com/phillarmonic/repertoire-ai/internal/state"
)

func TestCatalogInitScaffoldsLoadableCatalog(t *testing.T) {
	root := t.TempDir()
	output := mustRunCatalogInit(t, root, "company", []string{"code-reviewer", "shared-helpers"}, false, false)
	for _, want := range []string{
		"created repertoire.yaml",
		"created skills/code-reviewer/SKILL.md",
		"created skills/shared-helpers/SKILL.md",
		"edit skills/*/SKILL.md",
		"--override company=.",
		"catalog add",
		"git init",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}

	manifest, err := state.LoadManifest(filepath.Join(root, "repertoire.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Schema != 1 || manifest.Tool != state.ManifestTool {
		t.Fatalf("manifest schema/tool: %+v", manifest)
	}
	if manifest.Catalog == nil || manifest.Catalog.Name != "company" || manifest.Catalog.Description == "" {
		t.Fatalf("catalog section: %+v", manifest.Catalog)
	}
	if _, ok := manifest.Catalog.Skills["code-reviewer"]; !ok {
		t.Fatalf("skills: %+v", manifest.Catalog.Skills)
	}

	manager, err := catalog.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	materialized, err := manager.Materialize(catalog.Source{
		Name:         "company",
		Registration: state.CatalogRegistration{Source: root},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if materialized.Loose {
		t.Fatal("scaffolded catalog loaded as loose")
	}
	skillRoot := filepath.Join(root, "skills", "code-reviewer")
	if err := installer.ValidateSkill(skillRoot, "code-reviewer"); err != nil {
		t.Fatal(err)
	}
	content := readFileForTest(t, filepath.Join(skillRoot, "SKILL.md"))
	if !strings.Contains(content, "name: code-reviewer") || !strings.Contains(content, "description:") {
		t.Fatalf("SKILL.md:\n%s", content)
	}
}

func TestCatalogInitDefaultNameAndExampleSkill(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "Team Skills")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	output := mustRunCatalogInit(t, root, "", nil, false, false)
	if !strings.Contains(output, "created skills/team-skills-example/SKILL.md") {
		t.Fatalf("example skill output:\n%s", output)
	}
	manifest, err := state.LoadManifest(filepath.Join(root, "repertoire.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Catalog == nil || manifest.Catalog.Name != "team-skills" {
		t.Fatalf("default catalog: %+v", manifest.Catalog)
	}
}

func TestCatalogInitRejectsInvalidNames(t *testing.T) {
	root := t.TempDir()
	_, err := runCatalogInitCapture(t, root, "Company_Skills", nil, false, false)
	if err == nil || !strings.Contains(err.Error(), "must contain 1-64 lowercase letters, digits, or single hyphens") {
		t.Fatalf("invalid catalog name: %v", err)
	}
	_, err = runCatalogInitCapture(t, root, "company", []string{"Not_Valid"}, false, false)
	if err == nil || !strings.Contains(err.Error(), "must contain 1-64 lowercase letters, digits, or single hyphens") {
		t.Fatalf("invalid skill name: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "repertoire.yaml")); !os.IsNotExist(statErr) {
		t.Fatalf("invalid names wrote repertoire.yaml: %v", statErr)
	}
}

func TestCatalogInitRefusesExistingManifestWithoutForce(t *testing.T) {
	root := t.TempDir()
	mustRunCatalogInit(t, root, "company", []string{"demo"}, false, false)
	before := readFileForTest(t, filepath.Join(root, "repertoire.yaml"))
	_, err := runCatalogInitCapture(t, root, "other", []string{"other-skill"}, false, false)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("refuse existing: %v", err)
	}
	after := readFileForTest(t, filepath.Join(root, "repertoire.yaml"))
	if before != after {
		t.Fatalf("refused init changed repertoire.yaml")
	}
	output := mustRunCatalogInit(t, root, "other", []string{"other-skill"}, true, false)
	if !strings.Contains(output, "created repertoire.yaml") {
		t.Fatalf("force output:\n%s", output)
	}
	manifest, err := state.LoadManifest(filepath.Join(root, "repertoire.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Catalog.Name != "other" {
		t.Fatalf("forced catalog name %q", manifest.Catalog.Name)
	}
}

func TestCatalogInitDryRunDoesNotWrite(t *testing.T) {
	root := t.TempDir()
	output := mustRunCatalogInit(t, root, "company", []string{"demo"}, false, true)
	if !strings.Contains(output, "would create repertoire.yaml") || !strings.Contains(output, "would create skills/demo/SKILL.md") {
		t.Fatalf("dry-run output:\n%s", output)
	}
	if _, err := os.Stat(filepath.Join(root, "repertoire.yaml")); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote repertoire.yaml: %v", err)
	}
}

func TestCatalogInitCommandWiresFlags(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	var stdout bytes.Buffer
	command := NewRootCommand("test", &stdout, &stdout)
	command.SetArgs([]string{"catalog", "init", "wired", "--skill", "alpha"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "skills", "alpha", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
}

func TestCatalogInitProducesInstallableCatalogEndToEnd(t *testing.T) {
	binary := testBinaryPath(t)
	moduleRoot := filepath.Clean(filepath.Join("..", ".."))
	runCommand(t, moduleRoot, "go", "build", "-o", binary, "./cmd/repertoire")

	catalogRoot := t.TempDir()
	initOutput := runCommand(t, catalogRoot, binary, "catalog", "init", "local", "--skill", "demo")
	if !strings.Contains(initOutput, "created repertoire.yaml") {
		t.Fatalf("catalog init:\n%s", initOutput)
	}

	project, _, environment := bootstrapEnvironment(t)
	runCommandWithEnv(t, project, environment, binary, "--project", "catalog", "add", catalogRoot, "--name", "local")
	addOutput := runCommandWithEnv(t, project, environment, binary, "--project", "add", "demo", "--catalog", "local", "--target", "agents", "--no-hooks")
	if !strings.Contains(addOutput, "added demo") {
		t.Fatalf("install from scaffolded catalog:\n%s", addOutput)
	}
	if _, err := os.Stat(filepath.Join(project, ".agents", "skills", "demo", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
}

func mustRunCatalogInit(t *testing.T, root, name string, skills []string, force, dryRun bool) string {
	t.Helper()
	output, err := runCatalogInitCapture(t, root, name, skills, force, dryRun)
	if err != nil {
		t.Fatal(err)
	}
	return output
}

func runCatalogInitCapture(t *testing.T, root, name string, skills []string, force, dryRun bool) (string, error) {
	t.Helper()
	var stdout bytes.Buffer
	command := NewRootCommand("test", &stdout, &stdout)
	err := runCatalogInit(command, root, name, skills, force, dryRun)
	return stdout.String(), err
}
