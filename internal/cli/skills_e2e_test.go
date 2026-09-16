package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAddInstallAndListEndToEnd(t *testing.T) {
	project := t.TempDir()
	runCommand(t, project, "git", "init", "-q")
	catalogRoot := t.TempDir()
	for _, name := range []string{"demo", "loose"} {
		root := filepath.Join(catalogRoot, "skills", name)
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
		content := "---\nname: " + name + "\ndescription: Test skill\n---\n"
		if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	manifest := "schema: 1\ncatalog:\n  name: local\n  skills:\n    demo:\n      path: skills/demo\n    loose:\n      path: skills/loose\n"
	if err := os.WriteFile(filepath.Join(catalogRoot, "repertoire.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	binary := testBinaryPath(t)
	moduleRoot := filepath.Clean(filepath.Join("..", ".."))
	runCommand(t, moduleRoot, "go", "build", "-o", binary, "./cmd/repertoire")
	runCommand(t, project, binary, "--project", "catalog", "add", catalogRoot, "--name", "local")
	runCommand(t, project, binary, "--project", "add", "demo", "--catalog", "local", "--target", "agents")
	runCommand(t, project, binary, "--project", "install", "loose", "--catalog", "local", "--target", "agents")
	output := runCommand(t, project, binary, "--project", "list")
	if !strings.Contains(output, "demo\tlocal\tdeclared\tagents") ||
		!strings.Contains(output, "loose\tlocal\tad-hoc\tagents") {
		t.Fatalf("unexpected list output:\n%s", output)
	}
	if _, err := os.Stat(filepath.Join(project, ".agents", "skills", "demo", "SKILL.md")); err != nil {
		t.Fatalf("installed skill: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(project, ".agents", "skills", "demo")); err != nil {
		t.Fatal(err)
	}
	runCommand(t, project, binary, "--project", "install", "--target", "all")
	for _, path := range []string{
		filepath.Join(project, ".agents", "skills", "demo", "SKILL.md"),
		filepath.Join(project, ".codex", "skills", "demo", "SKILL.md"),
		filepath.Join(project, ".windsurf", "skills", "demo", "SKILL.md"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("installed declared skill on all targets at %s: %v", path, err)
		}
	}
	if err := os.RemoveAll(filepath.Join(project, ".codex", "skills", "demo")); err != nil {
		t.Fatal(err)
	}
	runCommand(t, project, binary, "--project", "update", "demo", "--target", "all")
	if _, err := os.Stat(filepath.Join(project, ".codex", "skills", "demo", "SKILL.md")); err != nil {
		t.Fatalf("updated skill on all targets: %v", err)
	}
	runCommand(t, project, binary, "--project", "remove", "demo")
	runCommand(t, project, binary, "--project", "remove", "loose")
	output = runCommand(t, project, binary, "--project", "list")
	if strings.TrimSpace(output) != "" {
		t.Fatalf("expected empty installed list, got %q", output)
	}
}

func TestQualifiedCatalogSkillEndToEnd(t *testing.T) {
	project := t.TempDir()
	runCommand(t, project, "git", "init", "-q")
	catalogRoot := t.TempDir()
	skillRoot := filepath.Join(catalogRoot, "skills", "code")
	if err := os.MkdirAll(skillRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: phillarmonkey/code\ndescription: Test skill\n---\n"
	if err := os.WriteFile(filepath.Join(skillRoot, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := "schema: 1\ncatalog:\n  name: local\n  skills:\n    phillarmonkey/code:\n      path: skills/code\n"
	if err := os.WriteFile(filepath.Join(catalogRoot, "repertoire.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	binary := testBinaryPath(t)
	moduleRoot := filepath.Clean(filepath.Join("..", ".."))
	runCommand(t, moduleRoot, "go", "build", "-o", binary, "./cmd/repertoire")
	runCommand(t, project, binary, "--project", "catalog", "add", catalogRoot, "--name", "local")
	runCommand(t, project, binary, "--project", "add", "phillarmonkey/code", "--target", "agents")
	output := runCommand(t, project, binary, "--project", "list")
	if !strings.Contains(output, "phillarmonkey/code\tlocal\tdeclared\tagents") {
		t.Fatalf("unexpected list output:\n%s", output)
	}
	installed := filepath.Join(project, ".agents", "skills", "phillarmonkey-code", "SKILL.md")
	if _, err := os.Stat(installed); err != nil {
		t.Fatalf("qualified skill was not installed to flat directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(project, ".agents", "skills", "phillarmonkey", "code")); !os.IsNotExist(err) {
		t.Fatalf("qualified skill should not install to nested directory: %v", err)
	}
	runCommand(t, project, binary, "--project", "remove", "phillarmonkey/code")
	if _, err := os.Stat(filepath.Dir(installed)); !os.IsNotExist(err) {
		t.Fatalf("qualified skill was not removed: %v", err)
	}
}

func TestLooseCatalogAddInstallUpdateRemoveEndToEnd(t *testing.T) {
	project := t.TempDir()
	runCommand(t, project, "git", "init", "-q")
	catalogRoot := t.TempDir()
	runCommand(t, catalogRoot, "git", "init", "-q")
	runCommand(t, catalogRoot, "git", "config", "user.email", "test@example.test")
	runCommand(t, catalogRoot, "git", "config", "user.name", "Test")
	for _, name := range []string{"alpha", "beta"} {
		root := filepath.Join(catalogRoot, "skills", name)
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
		content := "---\nname: " + name + "\ndescription: Loose skill\n---\nv1\n"
		if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(catalogRoot, "plugins", "example-plugin", "skills", "example-skill"), 0o755); err != nil {
		t.Fatal(err)
	}
	pluginSkill := "---\nname: example-skill\ndescription: Marketplace skill\n---\nplugin-v1\n"
	if err := os.WriteFile(filepath.Join(catalogRoot, "plugins", "example-plugin", "skills", "example-skill", "SKILL.md"), []byte(pluginSkill), 0o644); err != nil {
		t.Fatal(err)
	}
	runCommand(t, catalogRoot, "git", "add", ".")
	runCommand(t, catalogRoot, "git", "commit", "-qm", "initial")

	binary := testBinaryPath(t)
	moduleRoot := filepath.Clean(filepath.Join("..", ".."))
	runCommand(t, moduleRoot, "go", "build", "-o", binary, "./cmd/repertoire")
	runCommand(t, project, binary, "--project", "catalog", "add", catalogRoot, "--name", "official")
	listed := runCommand(t, project, binary, "--project", "catalog", "list")
	if !strings.Contains(listed, "official") || !strings.Contains(listed, "(loose)") {
		t.Fatalf("catalog list did not mark the loose catalog:\n%s", listed)
	}
	available := runCommand(t, project, binary, "--project", "list", "--available", "--catalog", "official")
	for _, name := range []string{"alpha", "beta", "example-skill"} {
		if !strings.Contains(available, name+"\tofficial (loose)\tavailable") {
			t.Fatalf("available list missing %s:\n%s", name, available)
		}
	}
	runCommand(t, project, binary, "--project", "add", "alpha", "--catalog", "official", "--target", "agents")
	runCommand(t, project, binary, "--project", "add", "example-skill", "--catalog", "official", "--target", "agents")
	lock := readFileForTest(t, filepath.Join(project, "repertoire.lock.json"))
	if !strings.Contains(lock, catalogRoot) || !strings.Contains(lock, `"commit"`) {
		t.Fatalf("lock did not record source and commit:\n%s", lock)
	}
	if err := os.WriteFile(filepath.Join(catalogRoot, "skills", "alpha", "SKILL.md"), []byte("---\nname: alpha\ndescription: Loose skill\n---\nv2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runCommand(t, project, binary, "--project", "update", "alpha")
	installed := readFileForTest(t, filepath.Join(project, ".agents", "skills", "alpha", "SKILL.md"))
	if !strings.Contains(installed, "v2") {
		t.Fatalf("update did not refresh loose skill:\n%s", installed)
	}
	runCommand(t, project, binary, "--project", "remove", "alpha")
	runCommand(t, project, binary, "--project", "remove", "example-skill")
}

func TestLooseCatalogMalformedManifestStillErrors(t *testing.T) {
	project := t.TempDir()
	runCommand(t, project, "git", "init", "-q")
	catalogRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(catalogRoot, "skills", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(catalogRoot, "skills", "demo", "SKILL.md"), []byte("---\nname: demo\ndescription: Test\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(catalogRoot, "repertoire.yaml"), []byte("schema: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	binary := testBinaryPath(t)
	moduleRoot := filepath.Clean(filepath.Join("..", ".."))
	runCommand(t, moduleRoot, "go", "build", "-o", binary, "./cmd/repertoire")
	command := exec.Command(binary, "--project", "catalog", "add", catalogRoot, "--name", "broken")
	command.Dir = project
	output, err := command.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "load catalog at") {
		t.Fatalf("expected malformed catalog error, got err=%v\n%s", err, output)
	}
}

func TestPlatformVariantAndManagedHooksEndToEnd(t *testing.T) {
	project := t.TempDir()
	runCommand(t, project, "git", "init", "-q")
	catalogRoot := t.TempDir()
	writeGraphifyCatalogFixture(t, catalogRoot, "v1")

	binary := testBinaryPath(t)
	moduleRoot := filepath.Clean(filepath.Join("..", ".."))
	runCommand(t, moduleRoot, "go", "build", "-o", binary, "./cmd/repertoire")
	runCommand(t, project, binary, "--project", "catalog", "add", catalogRoot, "--name", "graphify")

	runCommand(t, project, binary, "--project", "add", "graphify", "--catalog", "graphify", "--target", "codex")
	assertFileContent(t, filepath.Join(project, ".codex", "skills", "graphify", "variant.txt"), "v1")
	assertContainsFile(t, filepath.Join(project, "AGENTS.md"), "Graphify v1")
	if _, err := os.Stat(filepath.Join(project, ".codex", "hooks.json")); !os.IsNotExist(err) {
		t.Fatalf("noninteractive install unexpectedly created hooks: %v", err)
	}
	runCommand(t, project, binary, "--project", "remove", "graphify")

	if err := os.WriteFile(filepath.Join(project, "AGENTS.md"), []byte("# User\n\nKeep me.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(project, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	userHooks := `{"hooks":{"PreToolUse":[{"matcher":"User","hooks":[{"command":"user-hook"}]}]}}`
	if err := os.WriteFile(filepath.Join(project, ".codex", "hooks.json"), []byte(userHooks), 0o644); err != nil {
		t.Fatal(err)
	}
	runCommand(t, project, binary, "--project", "add", "graphify", "--catalog", "graphify", "--target", "codex", "--with-hooks")
	assertContainsFile(t, filepath.Join(project, "AGENTS.md"), "Graphify v1", "Keep me.")
	assertContainsFile(t, filepath.Join(project, ".codex", "hooks.json"), "graphify-v1", "user-hook")

	writeGraphifyCatalogFixture(t, catalogRoot, "v2")
	runCommand(t, project, binary, "--project", "update", "graphify")
	assertFileContent(t, filepath.Join(project, ".codex", "skills", "graphify", "variant.txt"), "v2")
	assertContainsFile(t, filepath.Join(project, "AGENTS.md"), "Graphify v2", "Keep me.")
	assertContainsFile(t, filepath.Join(project, ".codex", "hooks.json"), "graphify-v2", "user-hook")

	runCommand(t, project, binary, "--project", "update", "graphify", "--no-hooks")
	assertContainsFile(t, filepath.Join(project, "AGENTS.md"), "Graphify v2", "Keep me.")
	hooks := readFileForTest(t, filepath.Join(project, ".codex", "hooks.json"))
	if !strings.Contains(hooks, "user-hook") || strings.Contains(hooks, "graphify-v2") {
		t.Fatalf("hooks after --no-hooks:\n%s", hooks)
	}

	runCommand(t, project, binary, "--project", "update", "graphify", "--with-hooks")
	runCommand(t, project, binary, "--project", "remove", "graphify")
	assertContainsFile(t, filepath.Join(project, "AGENTS.md"), "Keep me.")
	assertContainsFile(t, filepath.Join(project, ".codex", "hooks.json"), "user-hook")
}

func writeGraphifyCatalogFixture(t *testing.T, root, version string) {
	t.Helper()
	for _, directory := range []string{
		filepath.Join(root, "skills", "graphify"),
		filepath.Join(root, "platforms", "codex"),
		filepath.Join(root, "project-files"),
	} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	skill := "---\nname: graphify\ndescription: Test Graphify skill\n---\n"
	for _, directory := range []string{
		filepath.Join(root, "skills", "graphify"),
		filepath.Join(root, "platforms", "codex"),
	} {
		if err := os.WriteFile(filepath.Join(directory, "SKILL.md"), []byte(skill), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "platforms", "codex", "variant.txt"), []byte(version), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "project-files", "agents.md"), []byte("## Graphify "+version+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	hooks := `{"hooks":{"PreToolUse":[{"matcher":"Graphify","hooks":[{"command":"graphify-` + version + `"}]}]}}`
	if err := os.WriteFile(filepath.Join(root, "project-files", "hooks.json"), []byte(hooks), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := `schema: 1
catalog:
  name: graphify
  skills:
    graphify:
      path: skills/graphify
      variants:
        codex: platforms/codex
      instructions:
        codex:
          - id: guidance
            source: project-files/agents.md
            destination: AGENTS.md
            mode: markdown-section
      artifacts:
        codex:
          - id: hooks
            source: project-files/hooks.json
            destination: .codex/hooks.json
            mode: json-merge
`
	if err := os.WriteFile(filepath.Join(root, "repertoire.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertContainsFile(t *testing.T, path string, values ...string) {
	t.Helper()
	content := readFileForTest(t, path)
	for _, value := range values {
		if !strings.Contains(content, value) {
			t.Fatalf("%s does not contain %q:\n%s", path, value, content)
		}
	}
}

func readFileForTest(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func TestUpdateRefreshesCatalogsAndAvailableDiscovery(t *testing.T) {
	binary := testBinaryPath(t)
	moduleRoot := filepath.Clean(filepath.Join("..", ".."))
	runCommand(t, moduleRoot, "go", "build", "-o", binary, "./cmd/repertoire")

	project, _, environment := bootstrapEnvironment(t)
	work, remote, _ := createTrackingCatalog(t)
	runCommandWithEnv(t, project, environment, binary, "--project", "catalog", "add", "file://"+remote, "--name", "tracking", "--ref", "main")
	runCommandWithEnv(t, project, environment, binary, "--project", "add", "tracked", "--catalog", "tracking", "--target", "agents")

	versionPath := filepath.Join(project, ".agents", "skills", "tracked", "version.txt")
	assertFileContent(t, versionPath, "v1")

	if err := os.WriteFile(filepath.Join(work, "skills", "tracked", "version.txt"), []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	addSkillToTrackingCatalog(t, work, "fresh", "fresh-v1")
	runGit(t, work, "add", ".")
	runGit(t, work, "commit", "-qm", "v2")
	runGit(t, work, "push", "-q", "origin", "main")

	output := runCommandWithEnv(t, project, environment, binary, "--project", "update")
	if !strings.Contains(output, "updated catalog tracking") || !strings.Contains(output, "updated tracked") {
		t.Fatalf("update output:\n%s", output)
	}
	assertFileContent(t, versionPath, "v2")

	output = runCommandWithEnv(t, project, environment, binary, "--project", "list", "--available", "--catalog", "tracking")
	if !strings.Contains(output, "fresh\ttracking\tavailable") {
		t.Fatalf("available list did not discover fresh skill:\n%s", output)
	}

	addSkillToTrackingCatalog(t, work, "newer", "newer-v1")
	runGit(t, work, "add", ".")
	runGit(t, work, "commit", "-qm", "newer")
	runGit(t, work, "push", "-q", "origin", "main")

	output = runCommandWithEnv(t, project, environment, binary, "--project", "update", "tracking")
	if !strings.Contains(output, "updated catalog tracking") {
		t.Fatalf("catalog update output:\n%s", output)
	}
	output = runCommandWithEnv(t, project, environment, binary, "--project", "list", "--available", "--catalog", "tracking")
	if !strings.Contains(output, "newer\ttracking\tavailable") {
		t.Fatalf("available list after catalog update:\n%s", output)
	}
}

func TestUpdateWithNothingInstalledIsANoOp(t *testing.T) {
	binary := testBinaryPath(t)
	moduleRoot := filepath.Clean(filepath.Join("..", ".."))
	runCommand(t, moduleRoot, "go", "build", "-o", binary, "./cmd/repertoire")

	project, _, environment := bootstrapEnvironment(t)
	output := runCommandWithEnv(t, project, environment, binary, "--project", "update")
	if !strings.Contains(output, "nothing to update") {
		t.Fatalf("greenfield update output:\n%s", output)
	}
	output = runCommandWithEnv(t, project, environment, binary, "--global", "update")
	if !strings.Contains(output, "nothing to update") {
		t.Fatalf("greenfield global update output:\n%s", output)
	}
}

func addSkillToTrackingCatalog(t *testing.T, root, name, marker string) {
	t.Helper()
	directory := filepath.Join(root, "skills", name)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: " + name + "\ndescription: Test\n---\n\n" + marker + "\n"
	if err := os.WriteFile(filepath.Join(directory, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, "repertoire.yaml")
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	manifest = append(manifest, []byte("    "+name+":\n      path: skills/"+name+"\n")...)
	if err := os.WriteFile(manifestPath, manifest, 0o644); err != nil {
		t.Fatal(err)
	}
}

func testBinaryPath(t *testing.T) string {
	t.Helper()
	name := "repertoire"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(t.TempDir(), name)
}

func TestAddCommaListAndGlobEndToEnd(t *testing.T) {
	project := t.TempDir()
	runCommand(t, project, "git", "init", "-q")
	catalogRoot := t.TempDir()
	for _, name := range []string{"product-ideation", "product-naming", "demo"} {
		root := filepath.Join(catalogRoot, "skills", name)
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
		content := "---\nname: " + name + "\ndescription: Test skill\n---\n"
		if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	manifest := "schema: 1\ncatalog:\n  name: local\n  skills:\n    demo:\n      path: skills/demo\n    product-ideation:\n      path: skills/product-ideation\n    product-naming:\n      path: skills/product-naming\n"
	if err := os.WriteFile(filepath.Join(catalogRoot, "repertoire.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	binary := testBinaryPath(t)
	moduleRoot := filepath.Clean(filepath.Join("..", ".."))
	runCommand(t, moduleRoot, "go", "build", "-o", binary, "./cmd/repertoire")
	runCommand(t, project, binary, "--project", "catalog", "add", catalogRoot, "--name", "local")

	output := runCommand(t, project, binary, "--project", "add", "demo,product-*", "--catalog", "local", "--target", "agents", "--no-hooks")
	for _, expected := range []string{
		"added demo from local",
		"added product-ideation from local",
		"added product-naming from local",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected %q in add output:\n%s", expected, output)
		}
	}

	command := exec.Command(binary, "--project", "add", "nope-*", "--catalog", "local")
	command.Dir = project
	if failure, err := command.CombinedOutput(); err == nil ||
		!strings.Contains(string(failure), `pattern "nope-*" matched no available skills`) {
		t.Fatalf("expected unmatched pattern error, got err=%v\n%s", err, failure)
	}
}

func TestOneShotAddFromSourceEndToEnd(t *testing.T) {
	binary := testBinaryPath(t)
	moduleRoot := filepath.Clean(filepath.Join("..", ".."))
	runCommand(t, moduleRoot, "go", "build", "-o", binary, "./cmd/repertoire")

	project := t.TempDir()
	runCommand(t, project, "git", "init", "-q")

	catalogRoot := filepath.Join(t.TempDir(), "acme-skills")
	writeSkillCatalog(t, catalogRoot, true, "alpha", "beta")

	output := runCommand(t, project, binary, "--project", "add", catalogRoot, "--target", "agents", "--no-hooks")
	if !strings.Contains(output, "registered acme-skills") {
		t.Fatalf("expected catalog registration:\n%s", output)
	}
	for _, name := range []string{"alpha", "beta"} {
		if !strings.Contains(output, "added "+name+" from acme-skills") {
			t.Fatalf("expected added %s:\n%s", name, output)
		}
		if _, err := os.Stat(filepath.Join(project, ".agents", "skills", name, "SKILL.md")); err != nil {
			t.Fatalf("installed %s: %v", name, err)
		}
	}

	named := filepath.Join(t.TempDir(), "other-skills")
	writeSkillCatalog(t, named, true, "code-reviewer")
	output = runCommand(t, project, binary, "--project", "add", named, "--name", "company", "--skill", "code-reviewer", "--target", "agents", "--no-hooks")
	if !strings.Contains(output, "registered company") || !strings.Contains(output, "added code-reviewer from company") {
		t.Fatalf("expected --name/--skill add:\n%s", output)
	}

	tailed := filepath.Join(t.TempDir(), "tailed-skills")
	writeSkillCatalog(t, tailed, true, "gamma", "delta")
	output = runCommand(t, project, binary, "--project", "add", filepath.Join(tailed, "gamma"), "--target", "agents", "--no-hooks")
	if !strings.Contains(output, "registered tailed-skills") || !strings.Contains(output, "added gamma from tailed-skills") {
		t.Fatalf("expected trailing skill:\n%s", output)
	}
	if _, err := os.Stat(filepath.Join(project, ".agents", "skills", "delta", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatal("trailing skill should not install every skill")
	}

	looseRoot := filepath.Join(t.TempDir(), "loose-skills")
	writeSkillCatalog(t, looseRoot, false, "omega")
	runCommand(t, looseRoot, "git", "init", "-q")
	output = runCommand(t, project, binary, "--project", "add", looseRoot, "--target", "agents", "--no-hooks")
	if !strings.Contains(output, "registered loose-skills") || !strings.Contains(output, "added omega from loose-skills") {
		t.Fatalf("expected loose catalog add:\n%s", output)
	}

	collision := filepath.Join(t.TempDir(), "acme-skills")
	writeSkillCatalog(t, collision, true, "zeta")
	failure := runCommandWithEnvError(t, project, os.Environ(), binary, "--project", "add", collision, "--target", "agents", "--no-hooks")
	if !strings.Contains(failure, `catalog "acme-skills" is already registered from`) || !strings.Contains(failure, "pass --name") {
		t.Fatalf("expected name collision:\n%s", failure)
	}

	qualifiedProject := t.TempDir()
	runCommand(t, qualifiedProject, "git", "init", "-q")
	qualifiedCatalog := t.TempDir()
	skillRoot := filepath.Join(qualifiedCatalog, "skills", "code")
	if err := os.MkdirAll(skillRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: phillarmonkey/code\ndescription: Test skill\n---\n"
	if err := os.WriteFile(filepath.Join(skillRoot, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := "schema: 1\ncatalog:\n  name: local\n  skills:\n    phillarmonkey/code:\n      path: skills/code\n"
	if err := os.WriteFile(filepath.Join(qualifiedCatalog, "repertoire.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	runCommand(t, qualifiedProject, binary, "--project", "catalog", "add", qualifiedCatalog, "--name", "local")
	output = runCommand(t, qualifiedProject, binary, "--project", "add", "phillarmonkey/code", "--target", "agents", "--no-hooks")
	if strings.Contains(output, "registered") {
		t.Fatalf("qualified skill id was treated as a source:\n%s", output)
	}
	if !strings.Contains(output, "added phillarmonkey/code from local") {
		t.Fatalf("expected qualified skill add:\n%s", output)
	}
}

func TestShowReportsModifiedTargetEndToEnd(t *testing.T) {
	project := t.TempDir()
	runCommand(t, project, "git", "init", "-q")
	catalogRoot := t.TempDir()
	root := filepath.Join(catalogRoot, "skills", "demo")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("---\nname: demo\ndescription: Test skill\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(catalogRoot, "repertoire.yaml"), []byte("schema: 1\ncatalog:\n  name: local\n  skills:\n    demo:\n      path: skills/demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	binary := testBinaryPath(t)
	moduleRoot := filepath.Clean(filepath.Join("..", ".."))
	runCommand(t, moduleRoot, "go", "build", "-o", binary, "./cmd/repertoire")
	runCommand(t, project, binary, "--project", "catalog", "add", catalogRoot, "--name", "local")
	runCommand(t, project, binary, "--project", "add", "demo", "--catalog", "local", "--target", "agents", "--target", "codex", "--no-hooks")
	codexSkill := filepath.Join(project, ".codex", "skills", "demo", "SKILL.md")
	if err := os.WriteFile(codexSkill, []byte("---\nname: demo\ndescription: Test skill\n---\nlocal edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	output := runCommand(t, project, binary, "--project", "show", "demo", "--format", "table")
	agentsPath := filepath.Join(project, ".agents", "skills", "demo")
	codexPath := filepath.Join(project, ".codex", "skills", "demo")
	if !strings.Contains(output, "local") || !strings.Contains(output, catalogRoot) {
		t.Fatalf("show table missing catalog provenance:\n%s", output)
	}
	if !strings.Contains(output, agentsPath) || !strings.Contains(output, "intact") {
		t.Fatalf("show table missing intact agents copy:\n%s", output)
	}
	if !strings.Contains(output, codexPath) || !strings.Contains(output, "modified") {
		t.Fatalf("show table missing modified codex copy:\n%s", output)
	}

	jsonOutput := runCommand(t, project, binary, "--project", "show", "demo", "--format", "json")
	var view skillShowView
	if err := json.Unmarshal([]byte(jsonOutput), &view); err != nil {
		t.Fatalf("show JSON: %v\n%s", err, jsonOutput)
	}
	if view.Name != "demo" || view.Catalog != "local" || view.CatalogCache != showCachePresent {
		t.Fatalf("show JSON object = %+v", view)
	}
	if len(view.Targets) != 2 {
		t.Fatalf("show JSON targets = %+v", view.Targets)
	}
	byName := map[string]skillShowTarget{}
	for _, target := range view.Targets {
		byName[target.Name] = target
	}
	if byName["agents"].Status != showCopyIntact || byName["codex"].Status != showCopyModified {
		t.Fatalf("show JSON integrity = %+v", view.Targets)
	}

	missing := runCommandWithEnvError(t, project, os.Environ(), binary, "--project", "show", "absent")
	if !strings.Contains(missing, `skill "absent" is not managed in this scope`) {
		t.Fatalf("expected unmanaged skill error:\n%s", missing)
	}
}

func writeSkillCatalog(t *testing.T, root string, withManifest bool, skills ...string) {
	t.Helper()
	var manifest strings.Builder
	if withManifest {
		manifest.WriteString("schema: 1\ncatalog:\n  name: fixture\n  skills:\n")
	}
	for _, name := range skills {
		skillDir := filepath.Join(root, "skills", name)
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			t.Fatal(err)
		}
		content := "---\nname: " + name + "\ndescription: Test skill\n---\n"
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if withManifest {
			manifest.WriteString("    " + name + ":\n      path: skills/" + name + "\n")
		}
	}
	if withManifest {
		if err := os.WriteFile(filepath.Join(root, "repertoire.yaml"), []byte(manifest.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func runCommand(t *testing.T, directory, name string, arguments ...string) string {
	t.Helper()
	return runCommandWithEnv(t, directory, os.Environ(), name, arguments...)
}

func runCommandWithEnv(t *testing.T, directory string, environment []string, name string, arguments ...string) string {
	t.Helper()
	command := exec.Command(name, arguments...)
	command.Dir = directory
	command.Env = environment
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, arguments, err, output)
	}
	return string(output)
}
