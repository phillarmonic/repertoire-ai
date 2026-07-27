package cli

import (
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
